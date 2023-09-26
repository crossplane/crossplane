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

package transport

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// validatingRoundTripper is a round tripper that validates attributes of the
// request.
type validatingRoundTripper struct {
	validations []requestValidationFn
}

// RoundTrip validates a request and returns an error if a validation fails.
func (v *validatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for _, v := range v.validations {
		if err := v(req); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// requestValidationFn validates some aspect of a request in a round tripper.
type requestValidationFn func(*http.Request) error

// userAgentValidator validates the user agent header on a request.
func userAgentValidator(userAgent string) requestValidationFn {
	return func(r *http.Request) error {
		if diff := cmp.Diff(r.Header.Get("User-Agent"), userAgent); diff != "" {
			return errors.Errorf("expected User-Agent %s but got %s", userAgent, r.Header.Get("User-Agent"))
		}
		return nil
	}
}

var (
	_ http.RoundTripper = &UserAgent{}
	_ http.RoundTripper = &validatingRoundTripper{}
)

func TestUserAgent(t *testing.T) {
	cases := map[string]struct {
		reason      string
		req         *http.Request
		validations []requestValidationFn
		userAgent   string
		err         error
	}{
		"NoExistingNoProvided": {
			reason: "If no user agent is provided and none exists, there should be no user agent present.",
			req: &http.Request{
				Header: http.Header{},
			},
			validations: []requestValidationFn{
				userAgentValidator(""),
			},
		},
		"OneExistingNoProvided": {
			reason: "If no user agent is provided and one exists, it should be cleared.",
			req: &http.Request{
				Header: http.Header{
					"User-Agent": []string{"something"},
				},
			},
			validations: []requestValidationFn{
				userAgentValidator(""),
			},
		},
		"NoExistingProvided": {
			reason: "If a user agent is provided and none exists, it should be set to the provided.",
			req: &http.Request{
				Header: http.Header{},
			},
			validations: []requestValidationFn{
				userAgentValidator("test"),
			},
			userAgent: "test",
		},
		"OneExistingProvided": {
			reason: "If user agent is provided and one exists, it should be replaced.",
			req: &http.Request{
				Header: http.Header{
					"User-Agent": []string{"something"},
				},
			},
			validations: []requestValidationFn{
				userAgentValidator("test"),
			},
			userAgent: "test",
		},
		"MultipleExistingProvided": {
			reason: "If user agent is provided and multiple exist, they should be replaced.",
			req: &http.Request{
				Header: http.Header{
					"User-Agent": []string{"something", "else"},
				},
			},
			validations: []requestValidationFn{
				userAgentValidator("test"),
			},
			userAgent: "test",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			u := NewUserAgent(&validatingRoundTripper{
				validations: tc.validations,
			}, tc.userAgent)
			_, err := u.RoundTrip(tc.req) //nolint:bodyclose // No body to close.
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRoundTrip(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
