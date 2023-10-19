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
	"net/http"

	"github.com/alecthomas/kong"
	"github.com/upbound/up-sdk-go"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/internal/xpkg/upbound"
)

const (
	logoutPath = "/v1/logout"

	errLogoutFailed      = "unable to logout"
	errRemoveTokenFailed = "failed to remove token"
)

// AfterApply sets default values in login after assignment and validation.
func (c *logoutCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	c.client = cfg.Client
	return nil
}

// logoutCmd invalidates a stored session token for a given profile.
type logoutCmd struct {
	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`

	// Internal state. These aren't part of the user-exposed CLI structure.
	client up.Client
}

// Help prints out the help for the logout command.
func (c *logoutCmd) Help() string {
	return `
Crossplane can be extended using packages. A Crossplane package is sometimes
called an xpkg. Crossplane supports configuration, provider and function
packages. 

A package is an opinionated OCI image that contains everything needed to extend
Crossplane with new functionality. For example installing a provider package
extends Crossplane with support for new kinds of managed resource (MR).

This command logs out of the xpkg.upbound.io package registry. The Crossplane
CLI pushes packages to xpkg.upbound.io by default.

See https://docs.crossplane.io/latest/concepts/packages for more information.
`
}

// Run executes the logout command.
func (c *logoutCmd) Run(k *kong.Context, upCtx *upbound.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := c.client.NewRequest(ctx, http.MethodPost, logoutPath, "", nil)
	if err != nil {
		return errors.Wrap(err, errLogoutFailed)
	}
	if err := c.client.Do(req, nil); err != nil {
		return errors.Wrap(err, errLogoutFailed)
	}
	// Logout is successful, remove token from config and update.
	upCtx.Profile.Session = ""
	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, upCtx.Profile); err != nil {
		return errors.Wrap(err, errRemoveTokenFailed)
	}
	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return errors.Wrap(err, "failed to update config file")
	}

	_, _ = fmt.Fprintf(k.Stdout, "%s logged out.\n", upCtx.Profile.ID)
	return nil
}
