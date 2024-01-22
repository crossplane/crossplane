// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package transport contains HTTP transport utilities for Crossplane.
package transport

import (
	"fmt"
	"net/http"

	"github.com/crossplane/crossplane/internal/version"
)

// DefaultUserAgent is the default User-Agent header that is set when making
// HTTP requests for packages.
var DefaultUserAgent = fmt.Sprintf("%s/%s", "crossplane", version.New().GetVersionString())

// UserAgent wraps a RoundTripper and injects a user agent header.
type UserAgent struct {
	rt        http.RoundTripper
	userAgent string
}

// NewUserAgent constructs a new UserAgent transport.
func NewUserAgent(rt http.RoundTripper, userAgent string) *UserAgent {
	return &UserAgent{
		rt:        rt,
		userAgent: userAgent,
	}
}

// RoundTrip injects a User-Agent header into every request.
func (u *UserAgent) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", u.userAgent)
	return u.rt.RoundTrip(req)
}
