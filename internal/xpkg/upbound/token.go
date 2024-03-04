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

package upbound

import (
	"encoding/json"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const errInvalidTokenFile = "token file is invalid"

// TokenFile is the format in which Upbound tokens are stored on disk.
type TokenFile struct {
	AccessID string `json:"accessId"` //nolint:tagliatelle // Should be accessID, but keeping accessId for backward compatibility.
	Token    string `json:"token"`
}

// tokenConf is the configuration for obtaining a token.
type tokenConf struct {
	fs afero.Fs
}

// TokenOption modifies how a token is obtained.
type TokenOption func(conf *tokenConf)

// TokenFromPath extracts a token from the provided path.
func TokenFromPath(path string, opts ...TokenOption) (TokenFile, error) {
	conf := &tokenConf{
		fs: afero.NewOsFs(),
	}
	for _, o := range opts {
		o(conf)
	}
	tf := TokenFile{}
	f, err := conf.fs.Open(filepath.Clean(path))
	if err != nil {
		return tf, err
	}
	defer f.Close() //nolint:errcheck // we don't care about the error
	if err := json.NewDecoder(f).Decode(&tf); err != nil {
		return tf, err
	}
	if tf.AccessID == "" || tf.Token == "" {
		return tf, errors.New(errInvalidTokenFile)
	}
	return tf, nil
}
