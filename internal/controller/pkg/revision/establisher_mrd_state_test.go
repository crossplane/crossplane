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

package revision

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

type ssaClient struct {
	client.Client

	applied atomic.Int64
}

func newSSAClient(t *testing.T, objs ...client.Object) *ssaClient {
	t.Helper()

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	c := &ssaClient{}
	c.Client = fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		WithReturnManagedFields().
		WithInterceptorFuncs(interceptor.Funcs{Get: c.get, Patch: c.patch}).
		Build()

	return c
}

func (c *ssaClient) get(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := cl.Get(ctx, key, obj, opts...); err != nil {
		return err
	}

	if mrd, ok := obj.(*v1alpha1.ManagedResourceDefinition); ok && mrd.Spec.State == "" {
		mrd.Spec.State = v1alpha1.ManagedResourceDefinitionInactive
	}

	return nil
}

func (c *ssaClient) patch(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if po := (&client.PatchOptions{}).ApplyOptions(opts); slices.Contains(po.DryRun, metav1.DryRunAll) {
		return cl.Patch(ctx, obj, patch, opts...)
	}

	c.applied.Add(1)

	if obj.GetObjectKind().GroupVersionKind().Kind != v1alpha1.ManagedResourceDefinitionKind {
		return cl.Patch(ctx, obj, patch, opts...)
	}

	before := &v1alpha1.ManagedResourceDefinition{}
	_ = cl.Get(ctx, client.ObjectKeyFromObject(obj), before)

	if err := cl.Patch(ctx, obj, patch, opts...); err != nil {
		return err
	}

	got, _, _ := unstructured.NestedString(obj.(*unstructured.Unstructured).Object, "spec", "state")
	if got == "" {
		got = string(v1alpha1.ManagedResourceDefinitionInactive)
	}

	if before.Spec.State.IsActive() && got != string(v1alpha1.ManagedResourceDefinitionActive) {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: v1alpha1.Group, Kind: v1alpha1.ManagedResourceDefinitionKind},
			obj.GetName(),
			field.ErrorList{field.Invalid(field.NewPath("spec", "state"), got, "state cannot be changed once it becomes Active")},
		)
	}

	return nil
}

func (c *ssaClient) applies() int {
	return int(c.applied.Load())
}

func (c *ssaClient) state(t *testing.T, name string) v1alpha1.ManagedResourceDefinitionState {
	t.Helper()

	return c.mrd(t, name).Spec.State
}

func (c *ssaClient) mrd(t *testing.T, name string) *v1alpha1.ManagedResourceDefinition {
	t.Helper()

	m := &v1alpha1.ManagedResourceDefinition{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: name}, m); err != nil {
		t.Fatalf("cannot get MRD %q: %v", name, err)
	}

	return m
}

func (c *ssaClient) activate(t *testing.T, name string) {
	t.Helper()

	m := c.mrd(t, name)
	m.Spec.State = v1alpha1.ManagedResourceDefinitionActive

	if err := c.Update(context.Background(), m); err != nil {
		t.Fatalf("cannot activate MRD %q: %v", name, err)
	}
}

func testMRD(name string, state v1alpha1.ManagedResourceDefinitionState, shortNames ...string) *v1alpha1.ManagedResourceDefinition {
	return &v1alpha1.ManagedResourceDefinition{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.ManagedResourceDefinitionKind},
		ObjectMeta: metav1.ObjectMeta{Name: name + ".example.org"},
		Spec: v1alpha1.ManagedResourceDefinitionSpec{
			CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
				Group: "example.org",
				Names: extv1.CustomResourceDefinitionNames{Plural: name, Kind: name, ShortNames: shortNames},
				Scope: extv1.NamespaceScoped,
			},
			State: state,
		},
	}
}

func testProviderRevision(name string, uid types.UID) *v1.ProviderRevision {
	return &v1.ProviderRevision{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.SchemeGroupVersion.String(), Kind: v1.ProviderRevisionKind},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			UID:    uid,
			Labels: map[string]string{v1.LabelParentPackage: "provider-name"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.ProviderKind,
				Name:       "provider-name",
				UID:        "provider-uid",
			}},
		},
		Status: v1.ProviderRevisionStatus{
			PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
				TLSServerSecretName: new("tls-server-secret"),
			},
		},
	}
}

func establish(t *testing.T, e *APIEstablisher, rev *v1.ProviderRevision, control bool, mrds ...*v1alpha1.ManagedResourceDefinition) {
	t.Helper()

	objs := make([]runtime.Object, len(mrds))
	for i := range mrds {
		objs[i] = mrds[i]
	}

	refs, err := e.Establish(context.Background(), objs, rev, control)
	if err != nil {
		t.Fatalf("e.Establish(...): unexpected error: %v", err)
	}

	if control {
		rev.SetObjects(refs)
	}
}

func TestAPIEstablisherMRDState(t *testing.T) {
	active := v1alpha1.ManagedResourceDefinitionActive
	inactive := v1alpha1.ManagedResourceDefinitionInactive

	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "tls-server-secret"},
		Data:       map[string][]byte{"tls.crt": []byte("CABUNDLE")},
	}

	t.Run("CRDOnlyPackageStaysActive", func(t *testing.T) {
		c := newSSAClient(t, tlsSecret)
		e := newAPIEstablisher(c)
		rev := testProviderRevision("rev-1", "rev-1-uid")

		establish(t, e, rev, true, testMRD("widgets", active), testMRD("gadgets", active))

		if diff := cmp.Diff(2, c.applies()); diff != "" {
			t.Errorf("creating the package: -want applies, +got applies:\n%s", diff)
		}

		created := c.applies()

		for range 3 {
			establish(t, e, rev, true, testMRD("widgets", active), testMRD("gadgets", active))
		}

		if diff := cmp.Diff(created, c.applies()); diff != "" {
			t.Errorf("re-establishing an unchanged package: -want applies, +got applies:\n%s", diff)
		}

		for _, name := range []string{"widgets", "gadgets"} {
			if got := c.state(t, name+".example.org"); got != active {
				t.Errorf("MRD %q: want state %q, got %q", name, active, got)
			}
		}
	})

	t.Run("SafeStartPackageKeepsActivation", func(t *testing.T) {
		c := newSSAClient(t, tlsSecret)
		e := newAPIEstablisher(c)
		rev := testProviderRevision("rev-1", "rev-1-uid")

		establish(t, e, rev, true, testMRD("widgets", ""), testMRD("gadgets", ""))

		for _, name := range []string{"widgets", "gadgets"} {
			if got := c.state(t, name+".example.org"); got != inactive {
				t.Fatalf("MRD %q: want it created Inactive, got %q", name, got)
			}
		}

		c.activate(t, "widgets.example.org")

		created := c.applies()

		for range 3 {
			establish(t, e, rev, true, testMRD("widgets", ""), testMRD("gadgets", ""))
		}

		if diff := cmp.Diff(created, c.applies()); diff != "" {
			t.Errorf("re-establishing an unchanged package: -want applies, +got applies:\n%s", diff)
		}

		establish(t, e, rev, true, testMRD("widgets", "", "w"), testMRD("gadgets", "", "g"))

		if diff := cmp.Diff(created+2, c.applies()); diff != "" {
			t.Errorf("establishing a changed package: -want applies, +got applies:\n%s", diff)
		}

		if got := c.state(t, "widgets.example.org"); got != active {
			t.Errorf("activated MRD: want state %q, got %q", active, got)
		}
		if got := c.state(t, "gadgets.example.org"); got != inactive {
			t.Errorf("MRD nothing activated: want it Inactive, got %q", got)
		}
		if diff := cmp.Diff([]string{"w"}, c.mrd(t, "widgets.example.org").Spec.Names.ShortNames); diff != "" {
			t.Errorf("activated MRD: -want short names, +got short names:\n%s", diff)
		}
	})

	t.Run("StaleInactiveReadDoesNotDeactivate", func(t *testing.T) {
		c := newSSAClient(t, tlsSecret)
		e := newAPIEstablisher(c)
		rev := testProviderRevision("rev-1", "rev-1-uid")

		establish(t, e, rev, true, testMRD("widgets", ""))

		stale := c.mrd(t, "widgets.example.org")
		c.activate(t, "widgets.example.org")

		apply, needed, err := e.prepareApply(context.Background(), stale, testMRD("widgets", "", "w"), rev, true)
		if err != nil {
			t.Fatalf("e.prepareApply(...): unexpected error: %v", err)
		}
		if !needed {
			t.Fatal("e.prepareApply(...): want the changed package to need an apply")
		}

		if err := e.patch(context.Background(), apply, FieldOwnerAPIEstablisher); err != nil {
			t.Errorf("e.patch(...): applying what an Inactive read of a since-activated MRD produced: %v", err)
		}
		if got := c.state(t, "widgets.example.org"); got != active {
			t.Errorf("want state %q, got %q", active, got)
		}
	})

	cases := map[string]struct {
		reason   string
		v1, v2   v1alpha1.ManagedResourceDefinitionState
		activate bool
		want     v1alpha1.ManagedResourceDefinitionState
	}{
		"SafeStartExplicitInactive": {
			reason:   "An MRD that a policy activated stays Active when the package states Inactive.",
			v1:       inactive,
			v2:       inactive,
			activate: true,
			want:     active,
		},
		"CRDOnlyToCRDOnly": {
			reason: "An Active MRD of a provider without safe-start stays Active when the provider is upgraded.",
			v1:     active,
			v2:     active,
			want:   active,
		},
		"CRDOnlyToSafeStart": {
			reason: "An Active MRD stays Active when an upgrade adds the safe-start capability, which leaves the package without a state.",
			v1:     active,
			v2:     "",
			want:   active,
		},
		"SafeStartToSafeStart": {
			reason:   "An MRD that a policy activated stays Active when the provider is upgraded.",
			v1:       "",
			v2:       "",
			activate: true,
			want:     active,
		},
		"SafeStartInactive": {
			reason: "An MRD that nothing activated stays Inactive when the provider is upgraded.",
			v1:     "",
			v2:     "",
			want:   inactive,
		},
		"SafeStartToCRDOnly": {
			reason: "An Inactive MRD becomes Active when an upgrade drops the safe-start capability, which makes the package ask for Active.",
			v1:     "",
			v2:     active,
			want:   active,
		},
	}

	for name, tc := range cases {
		t.Run("Upgrade/"+name, func(t *testing.T) {
			c := newSSAClient(t, tlsSecret)
			e := newAPIEstablisher(c)
			rev1 := testProviderRevision("rev-1", "rev-1-uid")
			rev2 := testProviderRevision("rev-2", "rev-2-uid")

			establish(t, e, rev1, true, testMRD("widgets", tc.v1))

			if tc.activate {
				c.activate(t, "widgets.example.org")
			}

			if err := e.ReleaseObjects(context.Background(), rev1); err != nil {
				t.Fatalf("e.ReleaseObjects(...): unexpected error: %v", err)
			}

			establish(t, e, rev2, true, testMRD("widgets", tc.v2, "w"))

			got := c.mrd(t, "widgets.example.org")
			if state := c.state(t, "widgets.example.org"); state != tc.want {
				t.Errorf("\n%s\nwant state %q, got %q", tc.reason, tc.want, state)
			}
			if diff := cmp.Diff([]string{"w"}, got.Spec.Names.ShortNames); diff != "" {
				t.Errorf("\n%s\n-want short names of the new revision, +got short names:\n%s", tc.reason, diff)
			}

			ctrl := metav1.GetControllerOf(got)
			if ctrl == nil || ctrl.UID != rev2.GetUID() {
				t.Errorf("\n%s\nwant the new revision to control the MRD, got %v", tc.reason, ctrl)
			}
			if !slices.ContainsFunc(got.GetOwnerReferences(), func(r metav1.OwnerReference) bool { return r.UID == rev1.GetUID() }) {
				t.Errorf("\n%s\nwant the old revision to own the MRD", tc.reason)
			}

			settled := c.applies()
			establish(t, e, rev2, true, testMRD("widgets", tc.v2, "w"))

			if diff := cmp.Diff(settled, c.applies()); diff != "" {
				t.Errorf("\n%s\nre-establishing the upgraded package: -want applies, +got applies:\n%s", tc.reason, diff)
			}

			establish(t, e, rev1, false, testMRD("widgets", tc.v1))

			if state := c.state(t, "widgets.example.org"); state != tc.want {
				t.Errorf("\n%s\nafter the old revision established without control: want state %q, got %q", tc.reason, tc.want, state)
			}
			if ctrl := metav1.GetControllerOf(c.mrd(t, "widgets.example.org")); ctrl == nil || ctrl.UID != rev2.GetUID() {
				t.Errorf("\n%s\nafter the old revision established without control: want the new revision to control the MRD, got %v", tc.reason, ctrl)
			}
		})
	}
}
