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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFindImageTagForVersionConstraint(t *testing.T) {
	repoName := "ubuntu"
	responseTags := []byte(`{"tags":["1.2.3","4.5.6"]}`)
	cases := map[string]struct {
		responseBody  []byte
		host          string
		constraint    string
		expectedImage string
		expectError   bool
	}{
		"NoConstraint": {
			responseBody:  responseTags,
			constraint:    "1.2.3",
			expectedImage: "ubuntu:1.2.3",
		},
		"Constraint": {
			responseBody:  responseTags,
			constraint:    ">=1.2.3",
			expectedImage: "ubuntu:4.5.6",
		},
		"ConstraintV": {
			responseBody:  responseTags,
			constraint:    ">=v1.2.3",
			expectedImage: "ubuntu:4.5.6",
		},
		"ConstraintPreRelease": {
			responseBody:  responseTags,
			constraint:    ">v4.5.6-rc.0.100.g658deda0.dirty",
			expectedImage: "ubuntu:4.5.6",
		},
		"CannotFetchTags": {
			responseBody: responseTags,
			host:         "wrong.host",
			constraint:   ">=4.5.6",
			expectError:  true,
		},
		"NoMatchingTag": {
			responseBody: responseTags,
			constraint:   ">4.5.6",
			expectError:  true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tagsPath := fmt.Sprintf("/v2/%s/tags/list", repoName)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)
				case tagsPath:
					if r.Method != http.MethodGet {
						t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
					}

					w.Write(tc.responseBody)
				default:
					t.Fatalf("Unexpected path: %v", r.URL.Path)
				}
			}))
			defer server.Close()

			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("url.Parse(%v) = %v", server.URL, err)
			}

			host := u.Host
			if tc.host != "" {
				host = tc.host
			}

			image, err := findImageTagForVersionConstraint(fmt.Sprintf("%s/%s:%s", host, repoName, tc.constraint))

			expectedImage := ""
			if !tc.expectError {
				expectedImage = fmt.Sprintf("%s/%s", host, tc.expectedImage)
			}

			if tc.expectError && err == nil {
				t.Errorf("[%s] expected: error\n", name)
			} else if expectedImage != image {
				t.Errorf("[%s] expected: %s, got: %s\n", name, expectedImage, image)
			}
		})
	}
}
