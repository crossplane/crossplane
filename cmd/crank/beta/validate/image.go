/*
Copyright 2024 The Crossplane Authors.

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

package validate

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// ImageFetcher defines an interface for fetching images
type ImageFetcher interface {
	FetchBaseLayer(image string) (*conregv1.Layer, error)
}

// Fetcher implements the ImageFetcher interface
type Fetcher struct{}

// FetchBaseLayer fetches the base layer of the image which contains the 'package.yaml' file
func (f *Fetcher) FetchBaseLayer(image string) (*conregv1.Layer, error) {
	cBytes, err := crane.Config(image)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get config")
	}

	cfg := &conregv1.ConfigFile{}
	if err := yaml.Unmarshal(cBytes, cfg); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal image config")
	}

	if cfg.Config.Labels == nil {
		return nil, errors.New("cannot get image labels")
	}

	var label string
	ls := cfg.Config.Labels
	for v, k := range ls {
		if k == baseLayerLabel {
			label = v // e.g.: io.crossplane.xpkg:sha256:0158764f65dc2a68728fdffa6ee6f2c9ef158f2dfed35abbd4f5bef8973e4b59
		}
	}

	lDigest := strings.SplitN(label, ":", 2)[1] // e.g.: sha256:0158764f65dc2a68728fdffa6ee6f2c9ef158f2dfed35abbd4f5bef8973e4b59

	ll, err := crane.PullLayer(fmt.Sprintf(refFmt, image, lDigest))
	if err != nil {
		return nil, errors.Wrapf(err, "cannot pull base layer %s", lDigest)
	}

	return &ll, nil
}

func extractPackageContent(layer conregv1.Layer) ([][]byte, []byte, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot get uncompressed layer")
	}

	objs, err := load(rc)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot read from layer")
	}

	// we need the meta object for identifying the dependencies
	var metaObj []byte
	if len(objs) > 0 {
		metaObj = objs[0]
	}

	// the first line of the layer is not part of the meta object, so we need to remove it
	metaStr := string(metaObj)
	metaLines := strings.Split(metaStr, "\n")
	metaStr = strings.Join(metaLines[1:], "\n")

	// the last obj is not yaml, so we need to remove it
	return objs[1 : len(objs)-1], []byte(metaStr), nil
}
