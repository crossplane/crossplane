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
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

var (
	randImage           v1.Image
	noPlatform          *v1.Platform
	expectedAnnotations map[string]string
	indexWithExtensions v1.ImageIndex
)

func init() {
	layerSize := int64(1024)
	expectedAnnotations = map[string]string{
		AnnotationKey: ManifestAnnotation,
	}
	randImage, _ = random.Image(layerSize, 1)
	noPlatform = nil
	indexWithExtensions = mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: empty.Image,
		Descriptor: v1.Descriptor{
			MediaType:   types.OCIManifestSchema1,
			Annotations: expectedAnnotations,
		},
	})
}

func TestAppend(t *testing.T) {
	type args struct {
		keychain      remote.Option
		remoteRef     name.Reference
		image         v1.Image
		index         v1.ImageIndex
		manifestCount int
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"SuccessWithCorrectManifestAnnotation": {
			reason: "Extensions manifest is correctly annotated",
			args: args{
				image:         randImage,
				index:         empty.Index,
				manifestCount: 1,
			},
			want: nil,
		},
		"OnlyOneAnnotatedManifest": {
			reason: "Only one annotated extensions manifest exists in the index",
			args: args{
				image:         randImage,
				index:         indexWithExtensions,
				manifestCount: 1,
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			appender := NewAppender(tc.args.keychain, tc.args.remoteRef)

			index, err := appender.Append(tc.args.index, tc.args.image)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAppend(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			manifestList, _ := index.IndexManifest()

			if len(manifestList.Manifests) != tc.args.manifestCount {
				t.Errorf("Unexpected number of manifests in the index. Expected %d, found %d",
					tc.args.manifestCount,
					len(manifestList.Manifests))
			}

			extManifest := manifestList.Manifests[0]
			if diff := cmp.Diff(extManifest.Annotations, expectedAnnotations); diff != "" {
				t.Errorf("\n%s\nUnexpected or missing manifest annotations: -want annotations, +got annotations:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(extManifest.Platform, noPlatform); diff != "" {
				t.Errorf("\n%s\nUnexpected platform information on manifest: -want platform, +got platform:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(extManifest.MediaType, types.DockerManifestSchema2); diff != "" {
				t.Errorf("\n%s\nUnexpected manifest media type for index: -want mediatype, +got mediatype:\n%s", tc.reason, diff)
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
			extManifest, err := ImageFromFiles(afero.NewBasePathFs(afero.NewOsFs(), tc.args.root), "/")

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAppend(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			manifest, _ := extManifest.Manifest()
			layer := manifest.Layers[0]
			if diff := cmp.Diff(layer.Annotations, tc.want.annotations); diff != "" {
				t.Errorf("\n%s\nUnexpected or missing manifest annotations: -want annotations, +got annotations\n%s", tc.reason, diff)
			}
		})
	}
}
