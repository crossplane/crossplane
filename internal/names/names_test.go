/*
Copyright 2025 The Crossplane Authors.

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

// Forked from https://github.com/knative/pkg/blob/ee1db869c7ef25eb4ac5c9ba0ab73fdc3f1b9dfa/kmeta/names_test.go

/*
copyright 2019 the knative authors

licensed under the apache license, version 2.0 (the "license");
you may not use this file except in compliance with the license.
you may obtain a copy of the license at

    http://www.apache.org/licenses/license-2.0

unless required by applicable law or agreed to in writing, software
distributed under the license is distributed on an "as is" basis,
without warranties or conditions of any kind, either express or implied.
see the license for the specific language governing permissions and
limitations under the license.
*/

package names

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestChildName(t *testing.T) {
	tests := []struct {
		parent string
		suffix string
		want   string
	}{
		{
			parent: "asdf",
			suffix: "-deployment",
			want:   "asdf-deployment",
		}, {
			parent: strings.Repeat("f", 63),
			suffix: "-deployment",
			want:   "ffffffffffffffffffff105d7597f637e83cc711605ac3ea4957-deployment",
		}, {
			parent: strings.Repeat("f", 63),
			suffix: "-deploy",
			want:   "ffffffffffffffffffffffff105d7597f637e83cc711605ac3ea4957-deploy",
		}, {
			parent: strings.Repeat("f", 63),
			suffix: strings.Repeat("f", 63),
			want:   "fffffffffffffffffffffffffffffff0502661254f13c89973cb3a83e0cbec0",
		}, {
			parent: "a",
			suffix: strings.Repeat("f", 63),
			want:   "ab5cfd486935decbc0d305799f4ce4414ffffffffffffffffffffffffffffff",
		}, {
			parent: strings.Repeat("b", 32),
			suffix: strings.Repeat("f", 32),
			want:   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb329c7c81b9ab3ba71aa139066aa5625d",
		}, {
			parent: "aaaa",
			suffix: strings.Repeat("b---a", 20),
			want:   "aaaa7a3f7966594e3f0849720eced8212c18b---ab---ab---ab---ab---ab",
		}, {
			parent: strings.Repeat("a", 17),
			suffix: "a.-.-.-.-.-.-.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			want:   "aaaaaaaaaaaaaaaaaa1eb10dc911444f8434af83b7225442da",
		},
	}

	for _, test := range tests {
		t.Run(test.parent+"-"+test.suffix, func(t *testing.T) {
			got, want := ChildName(test.parent, test.suffix), test.want
			if errs := validation.IsDNS1123Subdomain(got); len(errs) != 0 {
				t.Errorf("Invalid DNS1123 Subdomain %s\n\n Errors: %v", got, errs)
			}
			if got != want {
				t.Errorf("%s-%s: got: %63s want: %63s\ndiff:%s", test.parent, test.suffix, got, want, cmp.Diff(want, got))
			}
		})
	}
}
