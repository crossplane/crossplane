// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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
	AccessID string `json:"accessId"`
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
