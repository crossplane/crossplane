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

package beta

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestGenerateManifestsScoped tests that scoped resources produce manifests with
// their scope fields set and distinct names when the external name repeats.
func TestGenerateManifestsScoped(t *testing.T) {
	mrd := testMRD("dns.example.org", "Record", "records")
	c := &discoverCmd{Namespace: "default", ProviderConfig: "default"}

	got, err := c.generateManifests(mrd, []externalResource{
		{Name: "record-1", Scope: map[string]string{"zone": "b.example"}},
		{Name: "record-1", Scope: map[string]string{"zone": "a.example"}},
	})
	if err != nil {
		t.Fatalf("generateManifests(...): unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("generateManifests(...): want 2 manifests, got %d", len(got))
	}

	if got[0].GetName() == got[1].GetName() {
		t.Errorf("generateManifests(...): same name %q for two scopes", got[0].GetName())
	}

	zones := make([]string, 0, len(got))
	for _, obj := range got {
		zone, found, err := unstructured.NestedString(obj.Object, "spec", "forProvider", "zone")
		if err != nil || !found {
			t.Fatalf("generateManifests(...): spec.forProvider.zone missing: found=%v err=%v", found, err)
		}
		zones = append(zones, zone)
	}

	if diff := cmp.Diff([]string{"a.example", "b.example"}, zones); diff != "" {
		t.Errorf("generateManifests(...): zones -want, +got:\n%s", diff)
	}
}

// TestGenerateManifestsUnscoped tests that unscoped resources get no forProvider
// fields, so their manifests are unchanged by scope support.
func TestGenerateManifestsUnscoped(t *testing.T) {
	mrd := testMRD("s3.aws.crossplane.io", "Bucket", "buckets")
	c := &discoverCmd{Namespace: "default", ProviderConfig: "default"}

	got, err := c.generateManifests(mrd, resources("my-bucket"))
	if err != nil {
		t.Fatalf("generateManifests(...): unexpected error: %v", err)
	}

	if _, found, _ := unstructured.NestedMap(got[0].Object, "spec", "forProvider"); found {
		t.Errorf("generateManifests(...): unscoped resource has spec.forProvider")
	}
}

// TestParseScopeFilter tests the --scope key=value parser.
func TestParseScopeFilter(t *testing.T) {
	tests := map[string]struct {
		args    []string
		want    map[string]string
		wantErr bool
	}{
		"Empty":      {args: nil, want: map[string]string{}},
		"One":        {args: []string{"zone=a.example"}, want: map[string]string{"zone": "a.example"}},
		"Two":        {args: []string{"zone=a.example", "project=p"}, want: map[string]string{"zone": "a.example", "project": "p"}},
		"NoEquals":   {args: []string{"zone"}, wantErr: true},
		"EmptyKey":   {args: []string{"=a"}, wantErr: true},
		"EmptyValue": {args: []string{"zone="}, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseScopeFilter(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseScopeFilter(%v): want error, got %v", tc.args, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseScopeFilter(%v): unexpected error: %v", tc.args, err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("parseScopeFilter(%v): -want, +got:\n%s", tc.args, diff)
			}
		})
	}
}

// TestInScope tests that the filter keeps only resources in the requested scope.
func TestInScope(t *testing.T) {
	all := []externalResource{
		{Name: "record-1", Scope: map[string]string{"zone": "a.example"}},
		{Name: "record-2", Scope: map[string]string{"zone": "b.example"}},
		{Name: "record-3"},
	}

	got := inScope(all, map[string]string{"zone": "a.example"})
	if diff := cmp.Diff([]externalResource{all[0]}, got); diff != "" {
		t.Errorf("inScope(...): -want, +got:\n%s", diff)
	}
}
