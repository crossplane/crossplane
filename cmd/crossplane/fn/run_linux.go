//go:build linux

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

package fn

import (
	"context"
	"io"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	fnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/v1alpha1"
	"github.com/crossplane/crossplane/internal/xfn"
)

// Error strings
const (
	errReadRL      = "cannot read ResourceList"
	errUnmarshalRL = "cannot unmarshal ResourceList YAML"
	errMarshalRL   = "cannot marshal ResourceList YAML"
	errFnFailed    = "function failed"
	errWriteRL     = "cannot write ResourceList YAML to stdout"
)

// Run a Composition container function.
func (c *containerCommand) Run() error {
	defer c.ResourceList.Close() //nolint:errcheck // This file is only open for reading.

	s := xfn.NewFilesystemStore(c.CacheDir)
	f := xfn.NewContainerRunner(c.Image, xfn.WithOCIFetcher(&BasicFetcher{}), xfn.WithOCIStore(s))

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	yb, err := io.ReadAll(c.ResourceList)
	if err != nil {
		return errors.Wrap(err, errReadRL)
	}

	in := &fnv1alpha1.ResourceList{}
	if err := yaml.Unmarshal(yb, in); err != nil {
		return errors.Wrap(err, errUnmarshalRL)
	}

	out, err := f.Run(ctx, in)
	if err != nil {
		return errors.Wrap(err, errFnFailed)
	}

	// TODO(negz): Write YAML to stdout.
	yb, err = yaml.Marshal(out)
	if err != nil {
		return errors.Wrap(err, errMarshalRL)
	}

	_, err = os.Stdout.Write(yb)
	return errors.Wrap(err, errWriteRL)
}
