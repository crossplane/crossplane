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
package claim

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestTemplateNameGenerator(t *testing.T) {
	makeClaim := func(name, namespace string, gvk schema.GroupVersionKind) resource.CompositeClaim {
		cm := claim.New(claim.WithGroupVersionKind(gvk))
		cm.SetName(name)
		cm.SetNamespace(namespace)
		return cm
	}

	type args struct {
		template  string
		composite resource.Composite
		claim     resource.CompositeClaim
	}
	type want struct {
		name        string
		templateErr error
		err         error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"success": {
			reason: "Should generate a name from the given template",
			args: args{
				template:  `{{ printf "%s.%s-%s.%s" .Claim.Namespace .Claim.Kind (sha1sum .Claim.GVK | substr 0 5) .Claim.Name  }}`,
				composite: &fake.Composite{},
				claim:     makeClaim("test", "test-ns", fake.GVK(&claim.Unstructured{})),
			},
			want: want{
				name: "test-ns.Unstructured-1f9a9.test",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g, err := NewTemplateNameGenerator(tc.args.template)
			if diff := cmp.Diff(tc.want.templateErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewTemplateNameGenerator(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			err = g.Generate(context.Background(), tc.args.claim, tc.args.composite)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.name, tc.args.composite.GetName(), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
