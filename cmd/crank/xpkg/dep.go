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

package xpkg

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/cache"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/manager"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/resolver/image"
	"github.com/crossplane/crossplane/internal/xpkg/v2/workspace"
)

const (
	errMetaFileNotFound = "crossplane.yaml file not found in current directory"
)

// AfterApply constructs and binds context to any subcommands
// that have Run() methods that receive it.
func (c *depCmd) AfterApply(kongCtx *kong.Context, p pterm.TextPrinter) error {
	kongCtx.Bind(pterm.DefaultBulletList.WithWriter(kongCtx.Stdout))
	ctx := context.Background()
	fs := afero.NewOsFs()

	cache, err := cache.NewLocal(c.CacheDir)
	if err != nil {
		return err
	}

	c.c = cache

	// only parse the workspace if we aren't attempting to clean the cache
	if !c.CleanCache {

		r := image.NewResolver()

		m, err := manager.New(
			manager.WithCache(cache),
			manager.WithResolver(r),
		)

		if err != nil {
			return err
		}

		c.m = m

		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		ws, err := workspace.New(wd, workspace.WithFS(fs), workspace.WithPrinter(p))
		if err != nil {
			return err
		}
		c.ws = ws

		if err := ws.Parse(ctx); err != nil {
			return err
		}
	}

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

// depCmd manages crossplane dependencies.
type depCmd struct {
	c  *cache.Local
	m  *manager.Manager
	ws *workspace.Workspace

	// TODO(@tnthornton) remove cacheDir flag. Having a user supplied flag
	// can result in broken behavior between xpls and dep. CacheDir should
	// only be supplied by the Config.
	CacheDir   string `short:"d" help:"Directory used for caching package images." default:"~/.crossplane/cache/" env:"CACHE_DIR" type:"path"`
	CleanCache bool   `short:"c" help:"Clean dep cache."`

	Package string `arg:"" optional:"" help:"Package to be added."`
}

func (c *depCmd) Help() string {
	return `
The dep command manages crossplane package dependencies of the package 
in the current directory. It caches package information in a local file system
cache (by default in ~/.crossplane/cache), to be used e.g. for the Crossplane language
server.

If a package (e.g. provider-foo@v0.42.0 or provider-foo for latest) is specified,
it will be added to the crossplane.yaml file in the current directory as dependency. 
`
}

// Run executes the dep command.
func (c *depCmd) Run(ctx context.Context, p pterm.TextPrinter, pb *pterm.BulletListPrinter) error {
	// no need to do anything else if clean cache was called.

	// TODO (@tnthornton) this feels a little out of place here. We should
	// consider adding a separate command for doing this.
	if c.CleanCache {
		if err := c.c.Clean(); err != nil {
			return err
		}
		p.Printfln("xpkg cache cleaned")
		return nil
	}

	if c.Package != "" {
		if err := c.userSuppliedDep(ctx); err != nil {
			return err
		}
		p.Printfln("%s added to xpkg cache", c.Package)
		return nil
	}

	deps, err := c.metaSuppliedDeps(ctx)
	if err != nil {
		return err
	}
	if len(deps) == 0 {
		p.Printfln("No dependencies specified")
		return nil
	}
	p.Printfln("Dependencies added to xpkg cache:")
	li := make([]pterm.BulletListItem, len(deps))
	for i, d := range deps {
		li[i] = pterm.BulletListItem{
			Level:  0,
			Text:   fmt.Sprintf("%s (%s)", d.Package, d.Constraints),
			Bullet: "-",
		}
	}
	// TODO(hasheddan): bullet list printer incorrectly appends an extra
	// trailing newline. Update when fixed upstream.
	return pb.WithItems(li).Render()
}

func (c *depCmd) userSuppliedDep(ctx context.Context) error {
	// exit early check if we were supplied an invalid package string
	_, err := xpkg.ValidDep(c.Package)
	if err != nil {
		return err
	}

	d := dep.New(c.Package)

	ud, _, err := c.m.AddAll(ctx, d)
	if err != nil {
		return errors.Wrapf(err, "in %s", c.Package)
	}

	meta := c.ws.View().Meta()

	if meta != nil {
		// crossplane.yaml file exists in the workspace, upsert the new dependency
		if err := meta.Upsert(ud); err != nil {
			return err
		}

		if err := c.ws.Write(meta); err != nil {
			return err
		}
	}

	return nil
}

func (c *depCmd) metaSuppliedDeps(ctx context.Context) ([]v1beta1.Dependency, error) {
	meta := c.ws.View().Meta()

	if meta == nil {
		return nil, errors.New(errMetaFileNotFound)
	}

	deps, err := meta.DependsOn()
	if err != nil {
		return nil, err
	}

	resolvedDeps := make([]v1beta1.Dependency, len(deps))
	for i, d := range deps {
		ud, _, err := c.m.AddAll(ctx, d)
		if err != nil {
			return nil, err
		}
		resolvedDeps[i] = ud
	}

	return resolvedDeps, nil
}
