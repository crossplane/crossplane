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

package revision

import (
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Tests mutate.Extract() - an API used by Crossplane.
func FuzzGCRExtract(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		f, err := os.Create("tarfile")
		if err != nil {
			t.Skip()
		}
		defer func() {
			f.Close()
			os.Remove("tarfile")
		}()
		_, err = f.Write(data)
		if err != nil {
			t.Skip()
		}
		img, err := tarball.ImageFromPath("tarfile", nil)
		if err == nil {
			_, _ = img.Manifest()
			_ = mutate.Extract(img)
		}
	})
}

// Tests name.ParseReference - an API used by Crossplane
func FuzzParseReference(f *testing.F) {
	f.Fuzz(func(t *testing.T, data string) {
		_, _ = name.ParseReference(data)
	})
}
