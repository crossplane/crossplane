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

// Package transport contains HTTP transport utilities for Crossplane.
package transport

import (
	"fmt"
	"net/http"

	"github.com/crossplane/crossplane/internal/version"
)

// DefaultUserAgent is the default User-Agent header that is set when making
// HTTP requests for packages.
func DefaultUserAgent() string {
	return fmt.Sprintf("%s/%s", "crossplane", version.New().GetVersionString())
}

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
