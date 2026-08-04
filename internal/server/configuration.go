package server

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/serving"
)

const maximumConfiguration = 1 << 20

type configurationFile struct {
	Repositories []repositoryConfiguration `yaml:"repositories"`
}

type repositoryConfiguration struct {
	ID            string `yaml:"id"`
	Name          string `yaml:"name,omitempty"`
	Provider      string `yaml:"provider"`
	Remote        string `yaml:"remote"`
	DefaultBranch string `yaml:"default_branch"`
	Default       bool   `yaml:"default,omitempty"`
}

func loadRepositories(path string) ([]serving.Repository, error) {
	content, err := readBounded(path, maximumConfiguration)
	if err != nil {
		return nil, fmt.Errorf("reading server configuration %s: %w", path, err)
	}
	var parsed configurationFile
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parsing server configuration %s: %w", path, err)
	}
	repositories := make([]serving.Repository, len(parsed.Repositories))
	for index, configured := range parsed.Repositories {
		repositories[index] = serving.Repository{
			Identity: application.RepositoryIdentity{ID: configured.ID, Name: configured.Name},
			Provider: configured.Provider, Remote: configured.Remote,
			Branch: configured.DefaultBranch, Default: configured.Default,
		}
	}
	return repositories, nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return content, nil
}
