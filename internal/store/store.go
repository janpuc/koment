package store

import (
	"fmt"
	"io/fs"
	"os"
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

type Store struct{ root string }

func Open(root string) *Store { return &Store{root: root} }

func (s *Store) Root() string { return s.root }

// FindRoot walks up from start for the directory that owns the annotations,
// preferring an existing .koment over the enclosing git work tree.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}

	gitRoot := ""
	for {
		if isDir(filepath.Join(dir, DirName)) {
			return dir, nil
		}
		if gitRoot == "" && exists(filepath.Join(dir, ".git")) {
			gitRoot = dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
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
	return validSourcePath(filepath.ToSlash(path))
}

func (s *Store) fromAbsolute(absolute, original string) (string, error) {
	relative, err := filepath.Rel(s.root, absolute)
	if err != nil {
		return "", fmt.Errorf("%s is not inside %s: %w", original, s.root, err)
	}
	return validSourcePath(filepath.ToSlash(relative))
}

func validSourcePath(path string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(path))
	switch {
	case clean == "" || clean == ".":
		return "", fmt.Errorf("empty source path")
	case filepath.IsAbs(clean) || strings.HasPrefix(clean, "/"):
		return "", fmt.Errorf("source path %s must be relative to the repository root", path)
	case clean == ".." || strings.HasPrefix(clean, "../"):
		return "", fmt.Errorf("source path %s escapes the repository root", path)
	}
	return clean, nil
}

func (s *Store) SourcePath(file string) (string, error) {
	clean, err := validSourcePath(file)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, filepath.FromSlash(clean)), nil
}

func (s *Store) RecordPath(file string) (string, error) {
	clean, err := validSourcePath(file)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.annotationsRoot(), filepath.FromSlash(clean)+recordSuffix), nil
}

func (s *Store) annotationsRoot() string {
	return filepath.Join(s.root, DirName, annotationsDir)
}

// Load reads the record for one source file. A file with no annotations
// reports fs.ErrNotExist rather than an empty record.
func (s *Store) Load(file string) (*Record, error) {
	path, err := s.RecordPath(file)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var record Record
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("in %s: %w", path, err)
	}

	clean, err := validSourcePath(file)
	if err != nil {
		return nil, err
	}
	if record.File != clean {
		return nil, fmt.Errorf("in %s: record claims file %s but is stored for %s", path, record.File, clean)
	}
	return &record, nil
}

func (s *Store) Save(record *Record) error {
	if err := record.Validate(); err != nil {
		return err
	}
	path, err := s.RecordPath(record.File)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}

	var encoded strings.Builder
	encoder := yaml.NewEncoder(&encoded)
	encoder.SetIndent(yamlIndent)
	if err := encoder.Encode(record); err != nil {
		return fmt.Errorf("encoding record for %s: %w", record.File, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("encoding record for %s: %w", record.File, err)
	}
	return writeAtomically(path, []byte(encoded.String()))
}

func writeAtomically(path string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("creating temporary file beside %s: %w", path, err)
	}
	defer func() { _ = os.Remove(temporary.Name()) }()

	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing %s: %w", temporary.Name(), err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", temporary.Name(), err)
	}
	if err := os.Chmod(temporary.Name(), 0o644); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", temporary.Name(), err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}

// FindByID locates one annotation across every record, by its stable id.
func (s *Store) FindByID(id string) (*Record, int, error) {
	files, err := s.AnnotatedFiles()
	if err != nil {
		return nil, 0, err
	}

	for _, file := range files {
		record, err := s.Load(file)
		if err != nil {
			return nil, 0, err
		}
		for i, annotation := range record.Annotations {
			if annotation.ID == id {
				return record, i, nil
			}
		}
	}
	return nil, 0, fmt.Errorf("no annotation with id %s", id)
}

// Remove deletes a file's record entirely, for when its last annotation moved
// elsewhere. An empty record file would fail validation on the next load.
func (s *Store) Remove(file string) error {
	path, err := s.RecordPath(file)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

// AnnotatedFiles lists every source path that has a record, sorted.
func (s *Store) AnnotatedFiles() ([]string, error) {
	root := s.annotationsRoot()
	var files []string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, recordSuffix) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(strings.TrimSuffix(relative, recordSuffix)))
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
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
