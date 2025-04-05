package lint

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Loader interface defines the contract for different input sources.
type Loader interface {
	Load() ([]*yaml.Node, error)
}

// NewLoader returns a Loader based on the input source.
func NewLoader(input string) (Loader, error) {
	if input == "-" {
		return &StdinLoader{}, nil
	}

	fi, err := os.Stat(input)
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat input source")
	}

	if fi.IsDir() {
		return &FolderLoader{path: input}, nil
	}

	return &FileLoader{path: input}, nil
}

// StdinLoader implements the Loader interface for reading from stdin.
type StdinLoader struct{}

// Load reads the contents from stdin.
func (s *StdinLoader) Load() ([]*yaml.Node, error) {
	stream, err := loadYAMLDocuments(os.Stdin)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load YAML from stdin")
	}

	return streamToNodes(stream)
}

// FileLoader implements the Loader interface for reading from a file.
type FileLoader struct {
	path string
}

// Load reads the contents from a file.
func (f *FileLoader) Load() ([]*yaml.Node, error) {
	stream, err := readFile(f.path)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read file")
	}

	return streamToNodes(stream)
}

// FolderLoader implements the Loader interface for reading from a folder.
type FolderLoader struct {
	path string
}

// Load reads the contents from all files in a folder.
func (f *FolderLoader) Load() ([]*yaml.Node, error) {
	var stream [][]byte
	err := filepath.Walk(f.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if isYAMLFile(info) {
			s, err := readFile(path)
			if err != nil {
				return err
			}
			stream = append(stream, s...)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot read folder")
	}

	return streamToNodes(stream)
}

func isYAMLFile(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	ext := filepath.Ext(info.Name())
	return ext == ".yaml" || ext == ".yml"
}

func readFile(path string) ([][]byte, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "cannot open file")
	}
	defer f.Close()

	return loadYAMLDocuments(f)
}

// loadYAMLDocuments splits multi-doc YAML input into individual byte slices.
func loadYAMLDocuments(r io.Reader) ([][]byte, error) {
	var stream [][]byte
	reader := bufio.NewReader(r)
	decoder := yaml.NewDecoder(reader)

	for {
		var rawNode yaml.Node
		err := decoder.Decode(&rawNode)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "cannot decode YAML document")
		}
		// Re-encode each individual document back to []byte for consistent processing
		docBytes, err := marshalNode(&rawNode)
		if err != nil {
			return nil, errors.Wrap(err, "cannot re-marshal YAML document")
		}
		stream = append(stream, docBytes)
	}

	return stream, nil
}

// streamToNodes parses []byte YAML docs into *yaml.Node trees.
func streamToNodes(stream [][]byte) ([]*yaml.Node, error) {
	var nodes []*yaml.Node

	for _, doc := range stream {
		var root yaml.Node
		if err := yaml.Unmarshal(doc, &root); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal YAML document")
		}
		if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
			continue
		}
		nodes = append(nodes, root.Content[0])
	}

	return nodes, nil
}

func marshalNode(node *yaml.Node) ([]byte, error) {
	return yaml.Marshal(node)
}
