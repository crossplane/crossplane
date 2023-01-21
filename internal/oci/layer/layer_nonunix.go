//go:build !unix

/*
Copyright 2022 The Crossplane Authors.

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

package layer

import (
	"archive/tar"
	"io"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// ExtractFIFO returns an error on non-Unix systems
func ExtractFIFO(_ *tar.Header, _ io.Reader, _ string) error {
	return errors.New("FIFOs are only supported on Unix")
}
