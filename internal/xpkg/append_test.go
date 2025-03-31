/*
Copyright 2025 The Crossplane Authors.

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

package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	randImage           v1.Image
	noPlatform          *v1.Platform
	expectedAnnotations map[string]string
)

func init() {
	layerSize := int64(1024)
	expectedAnnotations = map[string]string{
		AnnotationKey: ManifestAnnotation,
	}
	randImage, _ = random.Image(layerSize, 1)
	noPlatform = nil
}

func TestAppend(t *testing.T) {
	type args struct {
		keychain  remote.Option
		remoteRef name.Reference
		image     v1.Image
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"SuccessWithCorrectManifestAnnotation": {
			reason: "Extensions manifest is correctly annotated",
			args: args{
				image: randImage,
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			appender := NewAppender(tc.args.keychain, tc.args.remoteRef)

			index, err := appender.Append(empty.Index, tc.args.image)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAppend(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			manifestList, _ := index.IndexManifest()
			extManifest := manifestList.Manifests[0]
			if !cmp.Equal(extManifest.Annotations, expectedAnnotations) {
				t.Errorf("Unexpected or missing manifest annotations: %s", cmp.Diff(extManifest.Annotations, expectedAnnotations))
			}
			if !cmp.Equal(extManifest.Platform, noPlatform) {
				t.Errorf("Unexpected platform information on manifest: %s/%s", extManifest.Platform.OS, extManifest.Platform.Architecture)
			}
		})
	}
}

func TestExtensionsImage(t *testing.T) {
	type args struct {
		root string
	}
	type want struct {
		err         error
		annotations map[string]string
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessWithCorrectLayerAnnotations": {
			reason: "Extensions image has correct layering",
			args:   args{root: "./testdata"},
			want: want{
				err:         nil,
				annotations: map[string]string{AnnotationKey: "examples"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			extManifest, err := ImageFromFiles(tc.args.root)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAppend(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			manifest, _ := extManifest.Manifest()
			layer := manifest.Layers[0]
			if !cmp.Equal(layer.Annotations, tc.want.annotations) {
				t.Errorf("Unexpected or missing manifest annotations: %s", cmp.Diff(layer.Annotations, expectedAnnotations))
			}
		})
	}
}
