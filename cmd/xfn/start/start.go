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

// Package start implements the reference Composition Function runner.
// It exposes a gRPC API that may be used to run Composition Functions.
package start

import (
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/xfn"
)

// Error strings
const (
	errListenAndServe = "cannot listen for and serve gRPC API"
)

// Args contains the default registry used to pull XFN containers.
type Args struct {
	Registry string
}

// Command starts a gRPC API to run Composition Functions.
type Command struct {
	CacheDir   string `short:"c" help:"Directory used for caching function images and containers." default:"/xfn"`
	MapRootUID int    `help:"UID that will map to 0 in the function's user namespace. The following 65336 UIDs must be available. Ignored if xfn does not have CAP_SETUID and CAP_SETGID." default:"100000"`
	MapRootGID int    `help:"GID that will map to 0 in the function's user namespace. The following 65336 GIDs must be available. Ignored if xfn does not have CAP_SETUID and CAP_SETGID." default:"100000"`
	Network    string `help:"Network on which to listen for gRPC connections." default:"unix"`
	Address    string `help:"Address at which to listen for gRPC connections." default:"@crossplane/fn/default.sock"`
}

// Run a Composition Function gRPC API.
func (c *Command) Run(args *Args, log logging.Logger) error {
	// If we don't have CAP_SETUID or CAP_SETGID, we'll only be able to map our
	// own UID and GID to root inside the user namespace.
	rootUID := os.Getuid()
	rootGID := os.Getgid()
	setuid := xfn.HasCapSetUID() && xfn.HasCapSetGID() // We're using 'setuid' as shorthand for both here.
	if setuid {
		rootUID = c.MapRootUID
		rootGID = c.MapRootGID
	}

	// TODO(negz): Expose a healthz endpoint and otel metrics.
	f := xfn.NewContainerRunner(
		xfn.SetUID(setuid),
		xfn.MapToRoot(rootUID, rootGID),
		xfn.WithCacheDir(filepath.Clean(c.CacheDir)),
		xfn.WithLogger(log),
		xfn.WithRegistry(args.Registry))
	return errors.Wrap(f.ListenAndServe(c.Network, c.Address), errListenAndServe)
}
