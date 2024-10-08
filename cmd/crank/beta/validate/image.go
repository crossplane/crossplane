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
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/crane"
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// ImageFetcher defines an interface for fetching images.
type ImageFetcher interface {
	FetchBaseLayer(image string) (*conregv1.Layer, error)
}

// Fetcher implements the ImageFetcher interface.
type Fetcher struct{}

// FetchBaseLayer fetches the base layer of the image which contains the 'package.yaml' file.
func (f *Fetcher) FetchBaseLayer(image string) (*conregv1.Layer, error) {
	if strings.Contains(image, "@") {
		// Strip the digest before fetching the image
		image = strings.SplitN(image, "@", 2)[0]
	} else if strings.Contains(image, ":") {
		var err error
		image, err = findImageTagForVersionConstraint(image)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot find image tag for version constraint")
		}
	}

	cBytes, err := crane.Config(image)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get config")
	}

	cfg := &conregv1.ConfigFile{}
	if err := yaml.Unmarshal(cBytes, cfg); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal image config")
	}

	// TODO(ezgidemirel): consider using the annotations instead of labels to find out the base layer like package managed
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

func findImageTagForVersionConstraint(image string) (string, error) {
	// Separate the image base and the image tag
	parts := strings.Split(image, ":")
	lastPart := len(parts) - 1
	imageBase := strings.Join(parts[0:lastPart], ":")
	imageTag := parts[lastPart]

	// Check if the tag is a constraint
	isConstraint := true
	c, err := semver.NewConstraint(imageTag)
	if err != nil {
		isConstraint = false
	}

	// Return original image if no constraint was detected
	if !isConstraint {
		return image, nil
	}

	// Fetch all image tags
	tags, err := crane.ListTags(imageBase)
	if err != nil {
		return "", errors.Wrapf(err, "cannot fetch tags for the image %s", imageBase)
	}

	// Convert tags to semver versions
	vs := []*semver.Version{}
	for _, r := range tags {
		v, err := semver.NewVersion(r)
		if err != nil {
			// We skip any tags that are not valid semantic versions
			continue
		}
		vs = append(vs, v)
	}

	// Sort all versions and find the last version complient with the constraint
	sort.Sort(sort.Reverse(semver.Collection(vs)))
	var addVer string
	for _, v := range vs {
		if c.Check(v) {
			addVer = v.Original()

			break
		}
	}

	if addVer == "" {
		return "", errors.Errorf("cannot find any tag complient with the constraint %s", imageTag)
	}

	// Compose new complete image string if any complient version was found
	image = fmt.Sprintf("%s:%s", imageBase, addVer)

	return image, nil
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

	// the first line of the layer is not part of the object, so we need to remove it
	o := string(objs[0])
	ol := strings.Split(o, "\n")
	o = strings.Join(ol[1:], "\n")

	objs[0] = []byte(o)

	// extract meta and schema objects
	var metaObj []byte
	var schemaObjs [][]byte
	if len(objs) > 0 {
		for _, obj := range objs {
			if strings.Contains(string(obj), "meta.pkg.crossplane.io") {
				metaObj = obj
				break
			}
			schemaObjs = append(schemaObjs, obj)
		}
	}

	// the last obj is not yaml, so we need to remove it
	return schemaObjs, metaObj, nil
}
