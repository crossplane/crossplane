// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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
