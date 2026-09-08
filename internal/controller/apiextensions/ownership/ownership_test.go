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

package ownership

import (
	"context"
	"testing"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
)

func TestApplyRequiresExistingControl(t *testing.T) {
	parent := &v1.CompositeResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.org", UID: "xrd-uid"}}
	owner := meta.AsController(meta.TypedReferenceTo(parent, v1.CompositeResourceDefinitionGroupVersionKind))
	foreign := owner
	foreign.UID = "another-xrd-uid"
	for name, tc := range map[string]struct {
		exists  bool
		owners  []metav1.OwnerReference
		allowed bool
	}{
		"Create":  {allowed: true},
		"Owned":   {exists: true, owners: []metav1.OwnerReference{owner}, allowed: true},
		"Unowned": {exists: true},
		"Foreign": {exists: true, owners: []metav1.OwnerReference{foreign}},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			if err := extv1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			c := fake.NewClientBuilder().WithScheme(scheme).Build()
			if tc.exists {
				if err := c.Create(ctx, &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: parent.Name, OwnerReferences: tc.owners}}); err != nil {
					t.Fatal(err)
				}
			}
			desired := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: parent.Name, OwnerReferences: []metav1.OwnerReference{owner}}}
			err := resource.NewAPIUpdatingApplicator(c).Apply(ctx, desired, MustBeControlledBy(parent))
			if (err == nil) != tc.allowed {
				t.Fatalf("Apply error = %v, want allowed %v", err, tc.allowed)
			}
			if !tc.allowed {
				got := &extv1.CustomResourceDefinition{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(desired), got); err != nil {
					t.Fatal(err)
				}
				if metav1.IsControlledBy(got, parent) {
					t.Fatal("adopted unrelated CRD")
				}
			}
		})
	}
}
