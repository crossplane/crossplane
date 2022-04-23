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

// Package run implements a convenience CLI to run and test Composition Functions.
package run

import (
	"context"
	"io"
	"os"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	fnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/v1alpha1"
	"github.com/crossplane/crossplane/internal/xfn"
)

// Error strings
const (
	errCreateRunner = "cannot create container runner"
	errReadFIO      = "cannot read FunctionIO"
	errUnmarshalFIO = "cannot unmarshal FunctionIO YAML"
	errMarshalFIO   = "cannot marshal FunctionIO YAML"
	errWriteFIO     = "cannot write FunctionIO YAML to stdout"

	errFnFailed = "function failed"
)

// Command runs a Composition function.
type Command struct {
	CacheDir   string        `short:"c" help:"Directory used for caching function images and containers." default:"/xfn"`
	Timeout    time.Duration `help:"Maximum time for which the function may run before being killed." default:"30s"`
	MapRootUID int           `help:"UID that will map to 0 in the function's user namespace. The following 65336 UIDs must be available. Ignored if xfn does not have CAP_SETUID and CAP_SETGID." default:"100000"`
	MapRootGID int           `help:"GID that will map to 0 in the function's user namespace. The following 65336 GIDs must be available. Ignored if xfn does not have CAP_SETUID and CAP_SETGID." default:"100000"`

	Image      string   `arg:"" help:"OCI image to run."`
	FunctionIO *os.File `arg:"" help:"YAML encoded FunctionIO to pass to the function."`
}

// Run a Composition container function.
func (c *Command) Run() error {
	defer c.FunctionIO.Close() //nolint:errcheck,gosec // This file is only open for reading.

	// If we don't have CAP_SETUID or CAP_SETGID, we'll only be able to map our
	// own UID and GID to root inside the user namespace.
	rootUID := os.Getuid()
	rootGID := os.Getgid()
	setuid := xfn.HasCapSetUID() && xfn.HasCapSetGID() // We're using 'setuid' as shorthand for both here.
	if setuid {
		rootUID = c.MapRootUID
		rootGID = c.MapRootGID
	}

	f := xfn.NewContainerRunner(c.Image,
		xfn.SetUID(setuid),
		xfn.MapToRoot(rootUID, rootGID),
		xfn.WithCacheDir(c.CacheDir))

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	yb, err := io.ReadAll(c.FunctionIO)
	if err != nil {
		return errors.Wrap(err, errReadFIO)
	}

	in := &fnv1alpha1.FunctionIO{}
	if err := yaml.Unmarshal(yb, in); err != nil {
		return errors.Wrap(err, errUnmarshalFIO)
	}

	out, err := f.Run(ctx, in)
	if err != nil {
		return errors.Wrap(err, errFnFailed)
	}

	yb, err = yaml.Marshal(out)
	if err != nil {
		return errors.Wrap(err, errMarshalFIO)
	}

	_, err = os.Stdout.Write(yb)
	return errors.Wrap(err, errWriteFIO)
}
