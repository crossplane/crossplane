/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// Source is a source for interacting with a Config.
type Source interface {
	Initialize() error
	GetConfig() (*Config, error)
	UpdateConfig(cfg *Config) error
}

// NewFSSource constructs a new FSSource. Path must be supplied via modifier or
// Initialize must be called to use default.
// NOTE(hasheddan): using empty path by default is a bit of a footgun, so we
// should consider refactoring here. The motivation for the current design is to
// allow for flexibility in cases where the consumer does not want to create if
// the path does not exist, or they want to provide the FSSource as the default
// without handling an error in construction (see Docker credential helper for
// example).
func NewFSSource(modifiers ...FSSourceModifier) *FSSource {
	src := &FSSource{
		fs: afero.NewOsFs(),
	}
	for _, m := range modifiers {
		m(src)
	}

	return src
}

// FSSourceModifier modifies an FSSource.
type FSSourceModifier func(*FSSource)

// WithPath sets the config path for the filesystem source.
func WithPath(p string) FSSourceModifier {
	return func(f *FSSource) {
		f.path = filepath.Clean(p)
	}
}

// WithFS overrides the FSSource filesystem with the given filesystem.
func WithFS(fs afero.Fs) FSSourceModifier {
	return func(f *FSSource) {
		f.fs = fs
	}
}

// FSSource provides a filesystem source for interacting with a Config.
type FSSource struct {
	fs   afero.Fs
	path string
}

// Initialize creates a config in the filesystem if one does not exist. If path
// is not defined the default path is constructed.
func (src *FSSource) Initialize() error {
	if src.path == "" {
		p, err := GetDefaultPath()
		if err != nil {
			return err
		}
		src.path = p
	}
	if _, err := src.fs.Stat(src.path); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := src.fs.MkdirAll(filepath.Dir(src.path), 0o755); err != nil {
			return err
		}
		f, err := src.fs.OpenFile(src.path, os.O_CREATE, 0o600)
		if err != nil {
			return err
		}
		defer f.Close() //nolint:errcheck // we don't care about the error
	}
	return nil
}

// GetConfig fetches the config from a filesystem.
func (src *FSSource) GetConfig() (*Config, error) {
	f, err := src.fs.Open(src.path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // we don't care about the error
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	conf := &Config{}
	if len(b) == 0 {
		return conf, nil
	}
	if err := json.Unmarshal(b, conf); err != nil {
		return nil, err
	}
	return conf, nil
}

// UpdateConfig updates the Config in the filesystem.
func (src *FSSource) UpdateConfig(c *Config) error {
	f, err := src.fs.OpenFile(src.path, os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	// NOTE(hasheddan): We both defer and explicitly call Close() to ensure that
	// we close the file in the case that we encounter an error before write,
	// and that we return an error in the case that we write and then fail to
	// close the file (i.e. write buffer is not flushed). In the latter case the
	// deferred Close() will error (see https://golang.org/pkg/os/#File.Close),
	// but we do not check it.
	defer f.Close() //nolint:errcheck // we don't care about the error
	b, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	return f.Close()
}
