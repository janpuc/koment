package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

const (
	DirName = ".koment"

	annotationsDir = "annotations"
	recordSuffix   = ".yaml"
	yamlIndent     = 2
)

const schemaDirective = "# yaml-language-server: $schema=" + SchemaURL + "\n"

type Store struct{ root string }

func Open(root string) *Store { return &Store{root: root} }

func (s *Store) Root() string { return s.root }

func closeRepositoryRoot(root *os.Root, returnedError *error) {
	if err := root.Close(); err != nil {
		*returnedError = errors.Join(*returnedError, fmt.Errorf("closing repository root: %w", err))
	}
}

// FindRoot walks up from start for the directory that owns the annotations,
// preferring an existing .koment over the enclosing git work tree.
func FindRoot(start string) (string, error) {
	directory, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}

	gitRoot := ""
	for {
		if isDir(filepath.Join(directory, DirName)) {
			return directory, nil
		}
		if gitRoot == "" && exists(filepath.Join(directory, ".git")) {
			gitRoot = directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}

	if gitRoot != "" {
		return gitRoot, nil
	}
	return "", fmt.Errorf("no %s or .git directory at or above %s", DirName, start)
}

// FromWorkingDirectory reads a path the way a person typing it at a shell
// prompt means it: relative to where they are standing.
func (s *Store) FromWorkingDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	return s.fromAbsolute(absolute, path)
}

// FromRoot reads a path the way an API caller means it: already relative to the
// repository root, wherever the koment process happens to be running.
func (s *Store) FromRoot(path string) (string, error) {
	if filepath.IsAbs(path) {
		return s.fromAbsolute(path, path)
	}
	return validSourcePath(filepath.ToSlash(filepath.Clean(path)))
}

func (s *Store) fromAbsolute(absolute, original string) (string, error) {
	relative, err := filepath.Rel(s.root, absolute)
	if err != nil {
		return "", fmt.Errorf("%s is not inside %s: %w", original, s.root, err)
	}
	return validSourcePath(filepath.ToSlash(relative))
}

func validSourcePath(value string) (string, error) {
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("source path %s must use forward slashes", value)
	}
	clean := path.Clean(value)
	switch {
	case clean == "" || clean == ".":
		return "", fmt.Errorf("empty source path")
	case clean != value:
		return "", fmt.Errorf("source path %s is not canonical; use %s", value, clean)
	case path.IsAbs(clean) || hasDrivePrefix(clean):
		return "", fmt.Errorf("source path %s must be relative to the repository root", value)
	case clean == ".." || strings.HasPrefix(clean, "../"):
		return "", fmt.Errorf("source path %s escapes the repository root", value)
	}
	return clean, nil
}

func hasDrivePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	letter := value[0]
	return letter >= 'A' && letter <= 'Z' || letter >= 'a' && letter <= 'z'
}

func (s *Store) ReadSource(file string) (_ []byte, returnedError error) {
	clean, err := validSourcePath(file)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, fmt.Errorf("opening repository root %s: %w", s.root, err)
	}
	defer closeRepositoryRoot(root, &returnedError)
	content, err := root.ReadFile(filepath.FromSlash(clean))
	if err != nil {
		return nil, fmt.Errorf("reading source %s: %w", clean, err)
	}
	return content, nil
}

// WriteSource atomically replaces a repository file without crossing its root.
func (s *Store) WriteSource(file string, content []byte) (returnedError error) {
	clean, err := validSourcePath(file)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return fmt.Errorf("opening repository root %s: %w", s.root, err)
	}
	defer closeRepositoryRoot(root, &returnedError)
	name := filepath.FromSlash(clean)
	information, err := root.Stat(name)
	if err != nil {
		return fmt.Errorf("reading permissions for %s: %w", clean, err)
	}
	if err := writeAtomicallyWithMode(root, name, content, information.Mode().Perm()); err != nil {
		return fmt.Errorf("writing source %s: %w", clean, err)
	}
	return nil
}

func (s *Store) RecordPath(id string) (string, error) {
	name, err := recordName(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, name), nil
}

func recordName(id string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("annotation id %q is not a canonical ULID", id)
	}
	return filepath.Join(DirName, annotationsDir, id+recordSuffix), nil
}

func (s *Store) Load(id string) (_ *Annotation, returnedError error) {
	name, err := recordName(id)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, fmt.Errorf("opening repository root %s: %w", s.root, err)
	}
	defer closeRepositoryRoot(root, &returnedError)
	content, err := root.ReadFile(name)
	if err != nil {
		return nil, err
	}

	var annotation Annotation
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&annotation); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("parsing %s: multiple YAML documents are not allowed", name)
		}
		return nil, fmt.Errorf("parsing %s after the annotation: %w", name, err)
	}
	if err := annotation.Validate(); err != nil {
		return nil, fmt.Errorf("in %s: %w", name, err)
	}
	if annotation.ID != id {
		return nil, fmt.Errorf("in %s: record claims id %s but filename claims %s", name, annotation.ID, id)
	}
	return &annotation, nil
}

func (s *Store) Save(annotation *Annotation) (returnedError error) {
	if err := annotation.Validate(); err != nil {
		return err
	}
	name, err := recordName(annotation.ID)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return fmt.Errorf("opening repository root %s: %w", s.root, err)
	}
	defer closeRepositoryRoot(root, &returnedError)
	if err := root.MkdirAll(filepath.Join(DirName, annotationsDir), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Join(DirName, annotationsDir), err)
	}

	var encoded strings.Builder
	encoded.WriteString(schemaDirective)
	encoder := yaml.NewEncoder(&encoded)
	encoder.SetIndent(yamlIndent)
	if err := encoder.Encode(annotation); err != nil {
		return fmt.Errorf("encoding annotation %s: %w", annotation.ID, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("encoding annotation %s: %w", annotation.ID, err)
	}
	return writeAtomically(root, name, []byte(encoded.String()))
}

func writeAtomically(root *os.Root, name string, content []byte) error {
	return writeAtomicallyWithMode(root, name, content, 0o644)
}

func writeAtomicallyWithMode(root *os.Root, name string, content []byte, mode fs.FileMode) error {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return fmt.Errorf("creating temporary name for %s: %w", name, err)
	}
	temporaryName := name + "." + hex.EncodeToString(entropy[:])
	temporary, err := root.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("creating temporary file beside %s: %w", name, err)
	}
	defer func() { _ = root.Remove(temporaryName) }()

	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing %s: %w", temporaryName, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", temporaryName, err)
	}
	if err := root.Rename(temporaryName, name); err != nil {
		return fmt.Errorf("replacing %s: %w", name, err)
	}
	return nil
}

func (s *Store) FindByID(id string) (*Annotation, error) {
	annotation, err := s.Load(id)
	if errorsIsNotExist(err) {
		return nil, fmt.Errorf("no annotation with id %s", id)
	}
	return annotation, err
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func (s *Store) Remove(id string) (returnedError error) {
	name, err := recordName(id)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return fmt.Errorf("opening repository root %s: %w", s.root, err)
	}
	defer closeRepositoryRoot(root, &returnedError)
	if err := root.Remove(name); err != nil {
		return fmt.Errorf("removing %s: %w", name, err)
	}
	return nil
}

func (s *Store) All() (_ []Annotation, returnedError error) {
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, fmt.Errorf("opening repository root %s: %w", s.root, err)
	}
	defer closeRepositoryRoot(root, &returnedError)
	directory := path.Join(DirName, annotationsDir)
	entries, err := fs.ReadDir(root.FS(), directory)
	if errorsIsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", directory, err)
	}

	annotations := make([]Annotation, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("unexpected directory %s in flat annotation store", path.Join(directory, entry.Name()))
		}
		if !strings.HasSuffix(entry.Name(), recordSuffix) {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), recordSuffix)
		annotation, err := s.Load(id)
		if err != nil {
			return nil, err
		}
		annotations = append(annotations, *annotation)
	}
	return annotations, nil
}

func (s *Store) ForFile(file string) ([]Annotation, error) {
	clean, err := validSourcePath(file)
	if err != nil {
		return nil, err
	}
	all, err := s.All()
	if err != nil {
		return nil, err
	}
	annotations := make([]Annotation, 0)
	for _, annotation := range all {
		if annotation.File == clean {
			annotations = append(annotations, annotation)
		}
	}
	return annotations, nil
}

func (s *Store) AnnotatedFiles() ([]string, error) {
	annotations, err := s.All()
	if err != nil {
		return nil, err
	}
	unique := make(map[string]struct{}, len(annotations))
	for _, annotation := range annotations {
		unique[annotation.File] = struct{}{}
	}
	files := make([]string, 0, len(unique))
	for file := range unique {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
