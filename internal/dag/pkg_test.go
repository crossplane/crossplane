/*
Copyright 2026 The Crossplane Authors.

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

package dag

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
)

func TestPackagesToNodes(t *testing.T) {
	const (
		src = "example.com/fn"
		v1  = "v1.0.0"
		v2  = "v2.0.0"
	)

	type want struct {
		// identifier -> versions
		versions map[string][]string
		// identifier -> dependency identifiers (in order)
		deps map[string][]string
		// number of nodes
		count int
	}

	cases := map[string]struct {
		reason string
		pkgs   []v1beta1.LockPackage
		want   want
	}{
		"DistinctSources": {
			reason: "Packages with distinct sources should each become their own node.",
			pkgs: []v1beta1.LockPackage{
				{Name: "a", Source: "example.com/a", Version: v1},
				{Name: "b", Source: "example.com/b", Version: v2},
			},
			want: want{
				count:    2,
				versions: map[string][]string{"example.com/a": {v1}, "example.com/b": {v2}},
				deps:     map[string][]string{"example.com/a": {}, "example.com/b": {}},
			},
		},
		"MergesSameSource": {
			reason: "Multiple versions of the same source should merge into one node tracking all versions with a deduplicated union of dependencies.",
			pkgs: []v1beta1.LockPackage{
				{
					Name:    "fn-v1",
					Source:  src,
					Version: v1,
					Dependencies: []v1beta1.Dependency{
						{Package: "example.com/dep-a", Constraints: ">=1.0.0"},
					},
				},
				{
					Name:    "fn-v2",
					Source:  src,
					Version: v2,
					Dependencies: []v1beta1.Dependency{
						{Package: "example.com/dep-a", Constraints: ">=1.0.0"}, // duplicate
						{Package: "example.com/dep-b", Constraints: ">=2.0.0"}, // unique
					},
				},
			},
			want: want{
				count:    1,
				versions: map[string][]string{src: {v1, v2}},
				deps:     map[string][]string{src: {"example.com/dep-a", "example.com/dep-b"}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			nodes := PackagesToNodes(tc.pkgs...)

			if len(nodes) != tc.want.count {
				t.Fatalf("\n%s\nPackagesToNodes(...): want %d nodes, got %d", tc.reason, tc.want.count, len(nodes))
			}

			for _, n := range nodes {
				pn, ok := n.(*PackageNode)
				if !ok {
					t.Fatalf("node %q is not a *PackageNode", n.Identifier())
				}

				if diff := cmp.Diff(tc.want.versions[n.Identifier()], pn.GetVersions()); diff != "" {
					t.Errorf("\n%s\nversions for %q: -want, +got:\n%s", tc.reason, n.Identifier(), diff)
				}

				depIDs := make([]string, len(n.Neighbors()))
				for i, ne := range n.Neighbors() {
					depIDs[i] = ne.Identifier()
				}

				if diff := cmp.Diff(tc.want.deps[n.Identifier()], depIDs); diff != "" {
					t.Errorf("\n%s\ndependencies for %q: -want, +got:\n%s", tc.reason, n.Identifier(), diff)
				}
			}
		})
	}
}

func TestIsValidConstraints(t *testing.T) {
	const (
		src = "example.com/fn"
		v1  = "v1.0.0"
		v2  = "v2.0.0"
	)

	cases := map[string]struct {
		reason    string
		installed Node
		wanted    Node
		want      bool
	}{
		"SingleVersionSatisfies": {
			reason:    "A single installed version that satisfies the constraint is valid.",
			installed: &PackageNode{LockPackage: v1beta1.LockPackage{Source: src, Version: "v1.5.0"}},
			wanted:    &DependencyNode{Dependency: v1beta1.Dependency{Package: src, Constraints: ">=1.0.0"}},
			want:      true,
		},
		"SingleVersionDoesNotSatisfy": {
			reason:    "A single installed version that doesn't satisfy the constraint is invalid.",
			installed: &PackageNode{LockPackage: v1beta1.LockPackage{Source: src, Version: "v1.5.0"}},
			wanted:    &DependencyNode{Dependency: v1beta1.Dependency{Package: src, Constraints: ">=2.0.0"}},
			want:      false,
		},
		"AnyVersionSatisfies": {
			reason:    "When multiple versions are installed, the constraint is satisfied if any version satisfies it.",
			installed: &PackageNode{LockPackage: v1beta1.LockPackage{Source: src, Version: v1}, versions: []string{v1, v2}},
			wanted:    &DependencyNode{Dependency: v1beta1.Dependency{Package: src, Constraints: ">=2.0.0"}},
			want:      true,
		},
		"NoVersionSatisfies": {
			reason:    "When no installed version satisfies the constraint it is invalid.",
			installed: &PackageNode{LockPackage: v1beta1.LockPackage{Source: src, Version: v1}, versions: []string{v1, v2}},
			wanted:    &DependencyNode{Dependency: v1beta1.Dependency{Package: src, Constraints: ">=3.0.0"}},
			want:      false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isValidConstraints(tc.installed, tc.wanted); got != tc.want {
				t.Errorf("\n%s\nisValidConstraints(...): want %v, got %v", tc.reason, tc.want, got)
			}
		})
	}
}
