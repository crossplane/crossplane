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

package composite

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xcrd"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/features"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

func TestWriteCompositionStatus(t *testing.T) {
	old := metav1.NewTime(time.Unix(100, 0).UTC())
	responsive := v1.WatchCircuitClosed()
	responsive.LastTransitionTime = old
	responsive.ObservedGeneration = 2
	custom := xpv2.Condition{Type: "Pipeline", Status: corev1.ConditionTrue, Reason: "Passed", LastTransitionTime: old}
	foreign := xpv2.Condition{Type: "External", Status: corev1.ConditionTrue, Reason: "Verified", LastTransitionTime: old}

	xr := composite.New(composite.WithSchema(composite.SchemaModern),
		composite.WithGroupVersionKind(schema.GroupVersionKind{Group: "test.crossplane.io", Version: "v1", Kind: "XWidget"}))
	xr.SetName("widget")
	xr.SetNamespace("test")
	xr.SetResourceVersion("12")
	xr.SetDeletionPolicy(v1.CompositionDeletionPolicyForeground)
	xr.SetConditions(responsive, custom, foreign)
	xr.Object["status"].(map[string]any)["externalField"] = "keep"

	calls := 0
	c := &test.MockClient{
		MockStatusPatch: test.NewMockSubResourcePatchFn(nil, func(o client.Object) error {
			calls++
			intent := o.(*composite.Unstructured)
			if got := intent.GetResourceVersion(); got != "12" {
				t.Errorf("status precondition = %q, want 12", got)
			}
			if key := client.ObjectKeyFromObject(intent); key != client.ObjectKeyFromObject(xr) || intent.GroupVersionKind() != xr.GroupVersionKind() {
				t.Errorf("status apply targets %v %v, want %v %v", key, intent.GroupVersionKind(), client.ObjectKeyFromObject(xr), xr.GroupVersionKind())
			}
			if got := intent.GetDeletionPolicy(); got != v1.CompositionDeletionPolicyForeground {
				t.Errorf("status apply deletion policy = %q, want Foreground", got)
			}
			if diff := cmp.Diff([]xpv2.Condition{responsive, custom}, intent.GetConditions()); diff != "" {
				t.Errorf("status conditions (-want +got):\n%s", diff)
			}
			if widgets, _, _ := kunstructured.NestedInt64(intent.Object, "status", "widgets"); widgets != 42 {
				t.Errorf("function status widgets = %d, want 42", widgets)
			}
			if _, found, _ := kunstructured.NestedFieldNoCopy(intent.Object, "status", "externalField"); found {
				t.Error("status apply claimed a field from another controller")
			}
			return nil
		}),
	}
	res := CompositionResult{
		DesiredStatus: map[string]any{
			"widgets":    int64(42),
			"crossplane": map[string]any{"deletionPolicy": "Background"},
		},
		Conditions: []TargetedCondition{{Condition: custom}},
	}
	r := NewReconciler(c, xr.GroupVersionKind())
	if err := r.writeCompositionStatus(t.Context(), xr, res, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("got %d status applies, want 1", calls)
	}
}

func TestWriteCompositionStatusTransientErrors(t *testing.T) {
	gr := schema.GroupResource{Group: "test.crossplane.io", Resource: "xwidgets"}
	for name, patchErr := range map[string]error{
		"Conflict":           kerrors.NewConflict(gr, "widget", errors.New("conflict")),
		"ServiceUnavailable": kerrors.NewServiceUnavailable("apiserver unavailable"),
		"Timeout":            kerrors.NewTimeoutError("request timed out", 1),
		"ServerTimeout":      kerrors.NewServerTimeout(gr, "patch", 1),
		"TooManyRequests":    kerrors.NewTooManyRequests("rate limited", 1),
		"DeadlineExceeded":   context.DeadlineExceeded,
		"Canceled":           context.Canceled,
	} {
		t.Run(name, func(t *testing.T) {
			xr := composite.New(composite.WithSchema(composite.SchemaModern),
				composite.WithGroupVersionKind(schema.GroupVersionKind{Group: "test.crossplane.io", Version: "v1", Kind: "XWidget"}))
			xr.SetName("widget")
			xr.SetConditions(xpv2.ReconcileSuccess(), xpv2.Available())
			statusBefore, _, _ := kunstructured.NestedFieldCopy(xr.Object, statusField)
			updates := 0
			c := &test.MockClient{
				MockStatusPatch: test.NewMockSubResourcePatchFn(patchErr),
				MockStatusUpdate: func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
					updates++
					return nil
				},
			}
			recorder := record.NewFakeRecorder(10)
			r := NewReconciler(c, xr.GroupVersionKind(), WithRecorder(event.NewAPIRecorder(recorder)))
			err := r.writeCompositionStatus(t.Context(), xr, CompositionResult{DesiredStatus: map[string]any{"widgets": int64(1)}}, statusBefore)
			if !errors.Is(err, patchErr) || updates != 0 || len(recorder.Events) != 0 {
				t.Errorf("writeCompositionStatus() = %v (updates=%d, events=%d), want %v with no fallback or events", err, updates, len(recorder.Events), patchErr)
			}
		})
	}
}

func TestCompositionConditionsRawObservedGeneration(t *testing.T) {
	oldTime := metav1.NewTime(time.Unix(100, 0).UTC())
	newerTime := metav1.NewTime(time.Unix(200, 0).UTC())
	for name, tc := range map[string]struct {
		existingGen int64
		rawGen      int64
	}{
		"ClearsWhenOmitted": {existingGen: 3, rawGen: 0},
		"SetsWhenAdded":     {existingGen: 0, rawGen: 5},
	} {
		t.Run(name, func(t *testing.T) {
			xr := composite.New()
			existing := xpv2.Condition{
				Type: xpv2.TypeHealthy, Status: corev1.ConditionTrue, Reason: "Healthy",
				Message: "ok", LastTransitionTime: oldTime, ObservedGeneration: tc.existingGen,
			}
			xr.SetConditions(existing)
			xr.SetManagedFields([]metav1.ManagedFieldsEntry{{
				Manager: FieldOwnerXR, Operation: metav1.ManagedFieldsOperationApply, Subresource: statusField,
				FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:status":{"f:conditions":{"k:{\"type\":\"Healthy\"}":{}}}}`)},
			}})
			statusBefore, _, _ := kunstructured.NestedFieldCopy(xr.Object, statusField)
			raw := map[string]any{
				"type": string(xpv2.TypeHealthy), "status": "True", "reason": "Healthy",
				"message": "ok", "lastTransitionTime": newerTime.Format(time.RFC3339),
			}
			if tc.rawGen != 0 {
				raw["observedGeneration"] = tc.rawGen
			}
			conditions, err := compositionConditions(xr, CompositionResult{
				DesiredStatus: map[string]any{"conditions": []any{raw}},
			}, statusBefore)
			if err != nil {
				t.Fatal(err)
			}
			xr.Object[statusField] = map[string]any{"conditions": conditions}
			want := existing
			want.ObservedGeneration = tc.rawGen
			if diff := cmp.Diff(want, xr.GetCondition(xpv2.TypeHealthy)); diff != "" {
				t.Errorf("raw condition (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompositionConditionsRawSystemTypes(t *testing.T) {
	for _, typ := range []xpv2.ConditionType{xpv2.TypeHealthy, xpv2.TypeUpToDate} {
		t.Run(string(typ), func(t *testing.T) {
			xr := composite.New()
			ready := xpv2.Available()
			xr.SetConditions(ready)
			res := CompositionResult{DesiredStatus: map[string]any{"conditions": []any{
				map[string]any{"type": string(typ), "status": "True", "reason": "FunctionReported", "lastTransitionTime": "2026-01-01T00:00:00Z"},
				map[string]any{"type": "Ready", "status": "False", "reason": "FunctionReported", "lastTransitionTime": "2026-01-01T00:00:00Z"},
			}}}
			conditions, err := compositionConditions(xr, res, nil)
			if err != nil {
				t.Fatal(err)
			}
			xr.Object["status"] = map[string]any{"conditions": conditions}
			if got := xr.GetCondition(typ); got.Status != corev1.ConditionTrue || got.Reason != "FunctionReported" {
				t.Errorf("raw condition %s = %+v, want True/FunctionReported", typ, got)
			}
			if got := xr.GetCondition(xpv2.TypeReady); !got.Equal(ready) {
				t.Errorf("controller condition changed: %+v", got)
			}
		})
	}
}

func newCompositionStatusAPI(t *testing.T, scope v1.CompositeResourceScope) (client.WithWatch, schema.GroupVersionKind) {
	t.Helper()
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		t.Skip("requires Kubernetes API server and etcd via KUBEBUILDER_ASSETS")
	}

	gvk := schema.GroupVersionKind{Group: "test.crossplane.io", Version: "v1", Kind: "XWidget"}
	statusProps := xcrd.CompositeResourceStatusProps(scope)
	statusProps["widgets"] = extv1.JSONSchemaProps{Type: "integer", Minimum: new(float64(0))}
	statusProps["externalField"] = extv1.JSONSchemaProps{Type: "string"}
	statusProps["lastHandledReconcileAt"] = extv1.JSONSchemaProps{Type: "string"}
	e := &envtest.Environment{
		BinaryAssetsDirectory: assets,
		CRDs: []*extv1.CustomResourceDefinition{{
			ObjectMeta: metav1.ObjectMeta{Name: "xwidgets." + gvk.Group},
			Spec: extv1.CustomResourceDefinitionSpec{
				Group: gvk.Group,
				Scope: extv1.ClusterScoped,
				Names: extv1.CustomResourceDefinitionNames{Kind: gvk.Kind, Plural: "xwidgets", Singular: "xwidget"},
				Versions: []extv1.CustomResourceDefinitionVersion{{
					Name: gvk.Version, Served: true, Storage: true,
					Subresources: &extv1.CustomResourceSubresources{Status: &extv1.CustomResourceSubresourceStatus{}},
					Schema: &extv1.CustomResourceValidation{OpenAPIV3Schema: &extv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]extv1.JSONSchemaProps{
							"spec":   {Type: "object", XPreserveUnknownFields: new(true)},
							"status": {Type: "object", Properties: statusProps},
						},
					}},
				}},
			},
		}},
	}
	cfg, err := e.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := e.Stop(); err != nil {
			t.Error(err)
		}
	})

	api, err := client.NewWithWatch(cfg, client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return api, gvk
}

func newCompositionStatusReconciler(api client.WithWatch, xr *composite.Unstructured, policy v1.CompositionDeletionPolicy, runner FunctionRunner, funcs interceptor.Funcs, opts ...ReconcilerOption) *Reconciler {
	get := funcs.Get
	funcs.Get = func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*pkgv1.Function); ok {
			return mockGetFunction(nil)(ctx, key, obj)
		}
		if get != nil {
			return get(ctx, c, key, obj, opts...)
		}
		return c.Get(ctx, key, obj, opts...)
	}
	funcs.List = func(ctx context.Context, _ client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		return mockListFunctionRevisions()(ctx, list, opts...)
	}
	c := interceptor.NewClient(api, funcs)
	rev := NewCompositionRevision()
	rev.Spec.Pipeline = []v1.PipelineStep{{Step: "run", FunctionRef: &v1.FunctionReference{Name: "cool-function"}}}
	rev.Spec.DeletionPolicy = policy
	return NewReconciler(c, xr.GroupVersionKind(), append([]ReconcilerOption{
		WithCompositeSchema(xr.Schema),
		WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, _ resource.Composite) error { return nil })),
		WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
			return rev, nil
		})),
		WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error { return nil })),
		WithComposer(NewFunctionComposer(c, api, runner)),
	}, opts...)...)
}

func assertUnchangedComposite(t *testing.T, before, after *composite.Unstructured) {
	t.Helper()
	if diff := cmp.Diff(before.Object, after.Object); diff != "" {
		t.Errorf("reconcile changed the XR (-before +after):\n%s", diff)
	}
}

// TestCompositionStatusApply exercises SSA against a real API server.
func TestCompositionStatusApply(t *testing.T) {
	api, gvk := newCompositionStatusAPI(t, v1.CompositeResourceScopeLegacyCluster)
	ctx := t.Context()
	xr := composite.New(composite.WithGroupVersionKind(gvk), composite.WithSchema(composite.SchemaLegacy))
	xr.SetName("widget")
	xr.SetAnnotations(map[string]string{meta.AnnotationKeyReconcileRequestedAt: "token"})
	if err := api.Create(ctx, xr); err != nil {
		t.Fatal(err)
	}
	key := client.ObjectKeyFromObject(xr)
	read := func() *composite.Unstructured {
		t.Helper()
		x := composite.New(composite.WithGroupVersionKind(gvk), composite.WithSchema(composite.SchemaLegacy))
		if err := api.Get(ctx, key, x); err != nil {
			t.Fatal(err)
		}
		return x
	}

	oldTime := metav1.NewTime(time.Unix(100, 0).UTC())
	foreign := xpv2.Condition{Type: "External", Status: corev1.ConditionTrue, Reason: "Verified", LastTransitionTime: oldTime}
	external := composite.New(composite.WithGroupVersionKind(gvk))
	external.SetName(key.Name)
	external.SetConditions(foreign)
	external.Object["status"].(map[string]any)["externalField"] = "keep"
	//nolint:staticcheck // client.Apply is also how the existing XR status is written.
	if err := api.Status().Patch(ctx, external, client.Apply, client.FieldOwner("external")); err != nil {
		t.Fatal(err)
	}

	// Seed an object with split status ownership, as written by v2.4.2.
	seed := composite.New(composite.WithGroupVersionKind(gvk))
	seed.SetName(key.Name)
	seed.Object["status"] = map[string]any{"widgets": int64(1)}
	//nolint:staticcheck // Exercise the existing apply field owner.
	if err := api.Status().Patch(ctx, seed, client.Apply, client.FieldOwner(FieldOwnerXR)); err != nil {
		t.Fatal(err)
	}
	seed = read()
	ready := xpv2.Creating().WithMessage("Waiting for the widget")
	ready.ObservedGeneration = seed.GetGeneration()
	responsive := v1.WatchCircuitClosed()
	responsive.ObservedGeneration, responsive.LastTransitionTime = seed.GetGeneration(), oldTime
	pipelineCondition := xpv2.Condition{Type: "Pipeline", Status: corev1.ConditionFalse, Reason: "Waiting", Message: "Old failure", LastTransitionTime: oldTime, ObservedGeneration: seed.GetGeneration()}
	seed.SetConditions(ready, responsive, pipelineCondition)
	if err := api.Status().Update(ctx, seed, client.FieldOwner("crossplane")); err != nil {
		t.Fatal(err)
	}

	var cached *composite.Unstructured
	var beforeApply func()
	patches := 0
	funcs := interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if cached != nil {
				*obj.(*composite.Unstructured) = *cached.DeepCopy()
				return nil
			}
			return c.Get(ctx, key, obj, opts...)
		},
		SubResourcePatch: func(ctx context.Context, c client.Client, sub string, obj client.Object, p client.Patch, opts ...client.SubResourcePatchOption) error {
			patches++
			if beforeApply != nil {
				beforeApply()
			}
			return c.SubResource(sub).Patch(ctx, obj, p, opts...)
		},
		SubResourceUpdate: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ ...client.SubResourceUpdateOption) error {
			t.Fatal("successful composition must only use one status apply")
			return nil
		},
	}
	setStatusField := true
	runner := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
		status := map[string]any{}
		if setStatusField {
			status["widgets"] = 42
		}
		return &fnv1.RunFunctionResponse{
			Desired: &fnv1.State{Composite: &fnv1.Resource{
				Resource: MustStruct(map[string]any{"status": status}),
			}},
			Conditions: []*fnv1.Condition{{
				Type: "Pipeline", Status: fnv1.Status_STATUS_CONDITION_TRUE, Reason: "Passed",
				Target: new(fnv1.Target_TARGET_COMPOSITE_AND_CLAIM),
			}},
		}, nil
	})
	r := newCompositionStatusReconciler(api, xr, "", runner, funcs,
		WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, o ConnectionSecretOwner, _ managed.ConnectionDetails) (bool, error) {
			return o.(*composite.Unstructured).GetConnectionDetailsLastPublishedTime() == nil, nil
		})),
	)
	req := reconcile.Request{NamespacedName: key}

	// Raise the spec generation while leaving old conditions in status.
	initial := read()
	initial.Object["spec"] = map[string]any{"replicas": int64(2)}
	if err := api.Update(ctx, initial); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if patches != 1 {
		t.Fatalf("got %d status applies, want 1", patches)
	}
	current := read()
	for _, typ := range []xpv2.ConditionType{xpv2.TypeReady, xpv2.TypeSynced, v1.TypeResponsive} {
		got := current.GetCondition(typ)
		if got.Status != corev1.ConditionTrue || got.ObservedGeneration != current.GetGeneration() {
			t.Errorf("condition %s = %+v, want True at generation %d", typ, got, current.GetGeneration())
		}
	}
	if got := current.GetCondition("Pipeline"); got.Status != corev1.ConditionTrue || got.Message != "" || got.ObservedGeneration != 0 {
		t.Errorf("function condition = %+v, want True without the previous failure message or observed generation", got)
	}
	if got := current.GetCondition("External"); !got.Equal(foreign) || !got.LastTransitionTime.Equal(&oldTime) {
		t.Errorf("external condition changed: %+v", got)
	}
	if got, _, _ := kunstructured.NestedString(current.Object, "status", "externalField"); got != "keep" {
		t.Errorf("external status = %q, want keep", got)
	}
	if got, _, _ := kunstructured.NestedInt64(current.Object, "status", "widgets"); got != 42 {
		t.Errorf("function status widgets = %d, want 42", got)
	}
	if got := current.GetLastHandledReconcileAt(); got != "token" {
		t.Errorf("handled reconcile token = %q, want token", got)
	}
	if current.GetConnectionDetailsLastPublishedTime() == nil {
		t.Error("connection details publication time is missing")
	}
	if diff := cmp.Diff([]xpv2.ConditionType{"Pipeline"}, current.GetClaimConditionTypes()); diff != "" {
		t.Errorf("claim condition types (-want +got):\n%s", diff)
	}

	// Replay a stale cache snapshot.
	cached = initial
	got, err := r.Reconcile(ctx, req)
	if err != nil || got != (reconcile.Result{Requeue: true}) {
		t.Fatalf("stale reconcile = (%+v, %v), want requeue", got, err)
	}
	cached = nil
	if patches != 1 || read().GetResourceVersion() != current.GetResourceVersion() {
		t.Fatal("stale reconcile changed status")
	}

	// Write a newer status immediately before the final apply.
	var concurrent *composite.Unstructured
	beforeApply = func() {
		concurrent = composite.New(composite.WithGroupVersionKind(gvk))
		concurrent.SetName(key.Name)
		concurrent.SetConditions(xpv2.Condition{Type: "External", Status: corev1.ConditionFalse, Reason: "Changed", LastTransitionTime: metav1.Now()})
		concurrent.Object["status"].(map[string]any)["externalField"] = "keep"
		//nolint:staticcheck // Write only the condition owned by this controller.
		if err := api.Status().Patch(ctx, concurrent, client.Apply, client.FieldOwner("external")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Reconcile(ctx, req); !kerrors.IsConflict(err) {
		t.Fatalf("concurrent reconcile error = %v, want conflict", err)
	}
	beforeApply = nil
	if got := read(); got.GetResourceVersion() != concurrent.GetResourceVersion() || got.GetCondition("External").Status != corev1.ConditionFalse {
		t.Fatal("final apply overwrote a concurrent status change")
	}

	// Cross the timestamp's one-second precision.
	time.Sleep(time.Second)
	for range 3 {
		if _, err := r.Reconcile(ctx, req); err != nil {
			t.Fatal(err)
		}
		assertUnchangedComposite(t, concurrent, read())
	}

	setStatusField = false
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := kunstructured.NestedFieldNoCopy(read().Object, "status", "widgets"); found {
		t.Fatal("omitting a previously applied function status field should remove it")
	}
}

func TestInvalidCompositionStatus(t *testing.T) {
	external := map[string]any{
		"type": "External", "status": "True", "reason": "Verified",
		"message": "", "lastTransitionTime": "2026-01-01T00:00:00Z",
	}
	for name, tc := range map[string]struct {
		status     map[string]any
		conditions []*fnv1.Condition
		policy     v1.CompositionDeletionPolicy
		invalid    bool
		published  bool
		fresh      bool
	}{
		"SchemaInvalid":             {status: map[string]any{"widgets": -1}, invalid: true},
		"MalformedConditions":       {status: map[string]any{"conditions": "not a condition list"}, invalid: true},
		"WrongFieldType":            {status: map[string]any{"widgets": "not an integer"}},
		"WrongObjectType":           {status: map[string]any{"connectionDetails": "not an object"}},
		"WrongPublishedObjectType":  {status: map[string]any{"connectionDetails": "not an object"}, invalid: true, published: true},
		"FirstMalformedConditions":  {status: map[string]any{"conditions": "not a condition list"}, invalid: true, fresh: true},
		"FreshModernDeletionPolicy": {status: map[string]any{"widgets": -1}, policy: v1.CompositionDeletionPolicyPipeline, invalid: true, fresh: true},
		"InvalidFunctionCondition": {
			status: map[string]any{"widgets": 2},
			conditions: []*fnv1.Condition{{
				Type: "BadCondition", Status: fnv1.Status_STATUS_CONDITION_TRUE, Reason: "Invalid",
				Target: new(fnv1.Target_TARGET_COMPOSITE_AND_CLAIM),
			}},
			invalid: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			scope := v1.CompositeResourceScopeLegacyCluster
			schema := composite.SchemaLegacy
			if tc.policy != "" {
				scope = v1.CompositeResourceScopeCluster
				schema = composite.SchemaModern
			}
			api, gvk := newCompositionStatusAPI(t, scope)
			ctx := t.Context()
			xr := composite.New(composite.WithGroupVersionKind(gvk), composite.WithSchema(schema))
			xr.SetName("invalid-widget")
			if err := api.Create(ctx, xr); err != nil {
				t.Fatal(err)
			}
			desiredStatus := map[string]any{"widgets": 1}
			updates := 0
			rejectBadCondition := func(obj client.Object) error {
				if u, ok := obj.(*composite.Unstructured); ok && len(tc.conditions) > 0 && u.GetCondition("BadCondition").Reason != "" {
					return kerrors.NewInvalid(gvk.GroupKind(), u.GetName(), nil)
				}
				return nil
			}
			funcs := interceptor.Funcs{
				SubResourcePatch: func(ctx context.Context, c client.Client, sub string, obj client.Object, p client.Patch, opts ...client.SubResourcePatchOption) error {
					if err := rejectBadCondition(obj); err != nil {
						return err
					}
					return c.SubResource(sub).Patch(ctx, obj, p, opts...)
				},
				SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					updates++
					if err := rejectBadCondition(obj); err != nil {
						return err
					}
					return c.SubResource(sub).Update(ctx, obj, opts...)
				},
			}
			var desiredConditions []*fnv1.Condition
			var desiredReady fnv1.Ready
			runner := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
				return &fnv1.RunFunctionResponse{
					Desired:    &fnv1.State{Composite: &fnv1.Resource{Resource: MustStruct(map[string]any{"status": desiredStatus}), Ready: desiredReady}},
					Conditions: desiredConditions,
				}, nil
			})
			recorder := record.NewFakeRecorder(10)
			opts := []ReconcilerOption{WithRecorder(event.NewAPIRecorder(recorder))}
			if tc.policy != "" {
				ff := &feature.Flags{}
				ff.Enable(features.EnableAlphaFunctionControlledDeletion)
				opts = append(opts, WithFeatures(ff))
			}
			r := newCompositionStatusReconciler(api, xr, tc.policy, runner, funcs, opts...)
			req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(xr)}
			if !tc.fresh {
				if _, err := r.Reconcile(ctx, req); err != nil {
					t.Fatal(err)
				}
				if err := api.Get(ctx, client.ObjectKeyFromObject(xr), xr); err != nil {
					t.Fatal(err)
				}
				if xr.GetCondition(xpv2.TypeSynced).Status != corev1.ConditionTrue {
					t.Fatal("XR should be synced before the function returns invalid status")
				}
				ext := composite.New(composite.WithGroupVersionKind(gvk))
				ext.SetName(xr.GetName())
				ext.Object[statusField] = map[string]any{"conditions": []any{external}}
				//nolint:staticcheck // Seed an external condition with an explicit empty message.
				if err := api.Status().Patch(ctx, ext, client.Apply, client.FieldOwner("external")); err != nil {
					t.Fatal(err)
				}
			}
			published := metav1.NewTime(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
			if tc.published {
				if err := api.Get(ctx, client.ObjectKeyFromObject(xr), xr); err != nil {
					t.Fatal(err)
				}
				xr.SetConnectionDetailsLastPublishedTime(&published)
				if err := api.Status().Update(ctx, xr); err != nil {
					t.Fatal(err)
				}
			}
			desiredStatus = tc.status
			desiredConditions = tc.conditions
			if len(tc.conditions) > 0 {
				desiredReady = fnv1.Ready_READY_FALSE
			}
			if _, err := r.Reconcile(ctx, req); err == nil || kerrors.IsInvalid(err) != tc.invalid {
				t.Fatalf("status reconcile error = %v, want error with Invalid=%t", err, tc.invalid)
			}
			message := errUpdateStatus
			if tc.invalid {
				message = errInvalidResources
			}
			wantUpdates := 1
			if tc.policy != "" {
				wantUpdates = 2 // deletionPolicy persistence + fallback ReconcileError
			}
			if updates != wantUpdates {
				t.Fatalf("got %d status updates, want %d", updates, wantUpdates)
			}
			if err := api.Get(ctx, client.ObjectKeyFromObject(xr), xr); err != nil {
				t.Fatal(err)
			}
			if tc.policy != "" && xr.GetDeletionPolicy() != tc.policy {
				t.Errorf("fallback lost deletionPolicy = %q, want %q", xr.GetDeletionPolicy(), tc.policy)
			}
			got := xr.GetCondition(xpv2.TypeSynced)
			if got.Status != corev1.ConditionFalse || got.Reason != xpv2.ReasonReconcileError || got.Message != message {
				t.Errorf("invalid status condition = %+v, want ReconcileError with a stable message", got)
			}
			if widgets, found, _ := kunstructured.NestedInt64(xr.Object, "status", "widgets"); (!tc.fresh && widgets != 1) || (tc.fresh && found) {
				t.Error("invalid desired status replaced the last good function status")
			}
			if !tc.fresh {
				conditions, _, _ := kunstructured.NestedSlice(xr.Object, statusField, "conditions")
				var gotExt any
				for _, item := range conditions {
					if m, ok := item.(map[string]any); ok && m["type"] == "External" {
						gotExt = m
					}
				}
				if diff := cmp.Diff(external, gotExt); diff != "" {
					t.Errorf("fallback modified external condition (-want +got):\n%s", diff)
				}
			}
			if len(tc.conditions) > 0 {
				if got := xr.GetCondition("BadCondition"); got.Status != corev1.ConditionUnknown || got.Reason != "" {
					t.Errorf("fallback persisted pending function condition: %+v", got)
				}
				if got := xr.GetClaimConditionTypes(); len(got) != 0 {
					t.Errorf("fallback persisted pending claimConditionTypes: %v", got)
				}
				if got := xr.GetCondition(xpv2.TypeReady); got.Status != corev1.ConditionTrue {
					t.Errorf("fallback overwrote prior Ready condition: %+v", got)
				}
			}
			if tc.published {
				if got := xr.GetConnectionDetailsLastPublishedTime(); got == nil || !published.Equal(got) {
					t.Error("failed status apply replaced the publication timestamp")
				}
			}
			warned := false
			for len(recorder.Events) > 0 {
				warned = strings.HasPrefix(<-recorder.Events, "Warning "+string(reasonCompose)+" ") || warned
			}
			if !warned {
				t.Error("failed status apply did not emit its warning event")
			}
			before := xr.DeepCopy()
			// Cross the timestamp's one-second precision.
			time.Sleep(1100 * time.Millisecond)
			if _, err := r.Reconcile(ctx, req); err == nil {
				t.Fatal("repeated invalid status should still fail")
			}
			if updates != wantUpdates {
				t.Errorf("repeated invalid reconcile issued extra status updates: got %d, want %d", updates, wantUpdates)
			}
			if err := api.Get(ctx, client.ObjectKeyFromObject(xr), xr); err != nil {
				t.Fatal(err)
			}
			assertUnchangedComposite(t, before, xr)
		})
	}
}

func TestCompletedPipelineDeletionStatus(t *testing.T) {
	external := map[string]any{
		"type": "External", "status": "True", "reason": "Concurrent",
		"message": "", "lastTransitionTime": "2026-01-01T00:00:00Z",
	}
	for name, tc := range map[string]struct {
		status        map[string]any
		conditions    []*fnv1.Condition
		invalid       bool
		compound      bool
		retry         bool
		completionErr string // "conflict" or "unavailable"
		last          bool
	}{
		"ValidCompletion": {},
		"TargetedConditionCompletion": {
			conditions: []*fnv1.Condition{{
				Type: "Cleanup", Status: fnv1.Status_STATUS_CONDITION_TRUE, Reason: "Complete",
				Target: new(fnv1.Target_TARGET_COMPOSITE),
			}},
		},
		"SchemaInvalidCompletion":       {status: map[string]any{"widgets": -1}, invalid: true},
		"MalformedCompletionConditions": {status: map[string]any{"conditions": "not a condition list"}, invalid: true},
		"WrongCompletionFieldType":      {status: map[string]any{"widgets": "not an integer"}},
		"FinalizerAndStatusFailure":     {compound: true},
		"RepeatedFinalizerFailure":      {retry: true},
		"CompletionConflict":            {retry: true, completionErr: "conflict"},
		"CompletionUnavailable":         {retry: true, completionErr: "unavailable"},
		"LastFinalizerFailure":          {retry: true, last: true},
	} {
		t.Run(name, func(t *testing.T) {
			api, gvk := newCompositionStatusAPI(t, v1.CompositeResourceScopeCluster)
			ctx := t.Context()
			xr := composite.New(composite.WithGroupVersionKind(gvk), composite.WithSchema(composite.SchemaModern))
			xr.SetName("deleting-widget")
			xr.SetAnnotations(map[string]string{meta.AnnotationKeyReconcileRequestedAt: "initial"})
			if err := api.Create(ctx, xr); err != nil {
				t.Fatal(err)
			}
			key := client.ObjectKeyFromObject(xr)
			read := func() *composite.Unstructured {
				t.Helper()
				x := composite.New(composite.WithGroupVersionKind(gvk), composite.WithSchema(composite.SchemaModern))
				if err := api.Get(ctx, key, x); err != nil {
					t.Fatal(err)
				}
				return x
			}
			desiredStatus := map[string]any{}
			if tc.compound || tc.retry {
				desiredStatus["widgets"] = 1
			}
			deletionPatches := 0
			statusUpdates := 0
			pipelineCalls := 0
			failing := false
			retrying := false
			injectCompletionErr := ""
			failurePatches := 0
			funcs := interceptor.Funcs{
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if failing && !slices.Contains(obj.GetFinalizers(), Finalizer) {
						return kerrors.NewServiceUnavailable("finalizer removal failed")
					}
					return c.Update(ctx, obj, opts...)
				},
				SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					statusUpdates++
					return c.SubResource(sub).Update(ctx, obj, opts...)
				},
				SubResourcePatch: func(ctx context.Context, c client.Client, sub string, obj client.Object, p client.Patch, opts ...client.SubResourcePatchOption) error {
					deletionPatches++
					if injectCompletionErr != "" && p.Type() == types.MergePatchType {
						mode := injectCompletionErr
						injectCompletionErr = ""
						current := read()
						current.Object[statusField].(map[string]any)["externalField"] = "from-other"
						conditions, _, _ := kunstructured.NestedSlice(current.Object, statusField, "conditions")
						_ = kunstructured.SetNestedSlice(current.Object, append(conditions, external), statusField, "conditions")
						if err := api.Status().Update(ctx, current); err != nil {
							return err
						}
						if mode == "unavailable" {
							return kerrors.NewServiceUnavailable("completion status unavailable")
						}
						// Forward the optimistic-lock merge patch so the real API server returns 409 Conflict.
					}
					if failing {
						failurePatches++
						if tc.compound && failurePatches == 2 {
							return kerrors.NewInternalError(errors.New("second status apply failed"))
						}
					}
					if retrying && obj.(*composite.Unstructured).GetCondition(xpv2.TypeSynced).Status == corev1.ConditionTrue {
						t.Error("finalizer retry prematurely reports Synced=True")
					}
					return c.SubResource(sub).Patch(ctx, obj, p, opts...)
				},
			}
			var desiredConditions []*fnv1.Condition
			runner := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
				pipelineCalls++
				return &fnv1.RunFunctionResponse{
					Desired:    &fnv1.State{Composite: &fnv1.Resource{Resource: MustStruct(map[string]any{"status": desiredStatus})}},
					Conditions: desiredConditions,
				}, nil
			})
			ff := &feature.Flags{}
			ff.Enable(features.EnableAlphaFunctionControlledDeletion)
			r := newCompositionStatusReconciler(api, xr, v1.CompositionDeletionPolicyPipeline, runner, funcs, WithFeatures(ff))
			req := reconcile.Request{NamespacedName: key}
			if _, err := r.Reconcile(ctx, req); err != nil {
				t.Fatal(err)
			}
			xr = read()
			if xr.GetCondition(xpv2.TypeReady).Status != corev1.ConditionTrue {
				t.Fatal("XR should be ready before deletion")
			}
			// Keep the XR available for inspection after Crossplane finishes deleting.
			if !tc.last {
				xr.SetFinalizers(append(xr.GetFinalizers(), "test.crossplane.io/retain"))
			}
			xr.SetAnnotations(map[string]string{meta.AnnotationKeyReconcileRequestedAt: "deleting"})
			if err := api.Update(ctx, xr); err != nil {
				t.Fatal(err)
			}
			if err := api.Delete(ctx, xr); err != nil {
				t.Fatal(err)
			}
			desiredConditions = tc.conditions
			if len(tc.conditions) > 0 {
				// Include a raw condition of the same type to verify targeted precedence.
				desiredStatus = map[string]any{
					"conditions": []any{map[string]any{
						"type": tc.conditions[0].GetType(), "status": "False", "reason": "Raw",
						"lastTransitionTime": "2026-01-01T00:00:00Z",
					}},
				}
			}
			if tc.compound || tc.retry {
				desiredStatus = map[string]any{"widgets": 2}
				failing = true
				if _, err := r.Reconcile(ctx, req); !kerrors.IsServiceUnavailable(err) {
					t.Fatalf("finalizer removal error = %v, want ServiceUnavailable", err)
				}
				xr = read()
				if widgets, _, _ := kunstructured.NestedInt64(xr.Object, "status", "widgets"); widgets != 2 {
					t.Errorf("fallback erased published completion status: widgets=%d, want 2", widgets)
				}
				if !slices.Contains(xr.GetFinalizers(), Finalizer) || failurePatches != 2 {
					t.Fatal("failed removal should retain the finalizer after reporting status")
				}
				if tc.retry {
					synced := xr.GetCondition(xpv2.TypeSynced)
					retrying = true
					time.Sleep(1100 * time.Millisecond)
					if _, err := r.Reconcile(ctx, req); !kerrors.IsServiceUnavailable(err) {
						t.Fatalf("retried finalizer removal error = %v, want ServiceUnavailable", err)
					}
					retrying = false
					xr = read()
					if got := xr.GetCondition(xpv2.TypeSynced); !got.Equal(synced) || !got.LastTransitionTime.Equal(&synced.LastTransitionTime) {
						t.Errorf("identical finalizer failure changed Synced: %+v -> %+v", synced, got)
					}
				}
				failing = false
			}
			if tc.status != nil {
				desiredStatus = tc.status
				beforeUpdates := statusUpdates
				if _, err := r.Reconcile(ctx, req); err == nil || kerrors.IsInvalid(err) != tc.invalid {
					t.Fatalf("completion error = %v, want error with Invalid=%t", err, tc.invalid)
				}
				message := errUpdateStatus
				if tc.invalid {
					message = errInvalidResources
				}
				xr = read()
				if !slices.Contains(xr.GetFinalizers(), Finalizer) {
					t.Fatal("invalid completion removed Crossplane's finalizer")
				}
				if statusUpdates != beforeUpdates+1 {
					t.Fatal("invalid completion should report failure using the last good status")
				}
				if got := xr.GetCondition(xpv2.TypeSynced); got.Status != corev1.ConditionFalse || got.Message != message {
					t.Errorf("failed completion Synced = %+v, want stable ReconcileError", got)
				}
				if got := xr.GetLastHandledReconcileAt(); got != "deleting" {
					t.Errorf("failed completion token = %q, want deleting", got)
				}
				before := xr.DeepCopy()
				time.Sleep(1100 * time.Millisecond)
				if _, err := r.Reconcile(ctx, req); err == nil {
					t.Fatal("repeated invalid completion should still fail")
				}
				xr = read()
				assertUnchangedComposite(t, before, xr)
				// A corrected pipeline result must still be able to complete deletion.
				desiredStatus = map[string]any{}
			}
			before := deletionPatches
			injectCompletionErr = tc.completionErr
			_, err := r.Reconcile(ctx, req)
			if tc.completionErr != "" {
				if (tc.completionErr == "conflict" && !kerrors.IsConflict(err)) || (tc.completionErr == "unavailable" && !kerrors.IsServiceUnavailable(err)) {
					t.Fatalf("completion error = %v, want %s", err, tc.completionErr)
				}
				xr = read()
				if slices.Contains(xr.GetFinalizers(), Finalizer) || xr.GetCondition(xpv2.TypeSynced).Status != corev1.ConditionFalse {
					t.Fatal("failed completion should leave only the retained XR's status unfinished")
				}
				calls := pipelineCalls
				if _, err := r.Reconcile(ctx, req); err != nil {
					t.Fatal(err)
				}
				if pipelineCalls != calls {
					t.Errorf("completion retry reran the pipeline: %d -> %d", calls, pipelineCalls)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tc.last {
				if err := api.Get(ctx, key, xr); !kerrors.IsNotFound(err) {
					t.Fatalf("completed XR still exists: %v", err)
				}
				return
			}
			xr = read()
			wantPatches := 1
			if tc.compound || tc.retry || tc.status != nil {
				wantPatches++
			}
			if tc.completionErr != "" {
				wantPatches++
			}
			if deletionPatches != before+wantPatches {
				t.Errorf("completed deletion used %d status applies, want %d", deletionPatches-before, wantPatches)
			}
			if tc.completionErr != "" {
				if got, _, _ := kunstructured.NestedString(xr.Object, statusField, "externalField"); got != "from-other" {
					t.Errorf("completion retry overwrote external status = %q, want from-other", got)
				}
				conditions, _, _ := kunstructured.NestedSlice(xr.Object, statusField, "conditions")
				var gotExt any
				for _, item := range conditions {
					if m, ok := item.(map[string]any); ok && m["type"] == "External" {
						gotExt = m
					}
				}
				if diff := cmp.Diff(external, gotExt); diff != "" {
					t.Errorf("completion retry modified external condition (-want +got):\n%s", diff)
				}
			}
			if diff := cmp.Diff([]string{"test.crossplane.io/retain"}, xr.GetFinalizers()); diff != "" {
				t.Errorf("remaining finalizers (-want +got):\n%s", diff)
			}
			ready := xr.GetCondition(xpv2.TypeReady)
			if ready.Status != corev1.ConditionFalse || ready.Reason != xpv2.ReasonDeleting {
				t.Errorf("completed deletion Ready = %+v, want False/Deleting", ready)
			}
			if xr.GetCondition(xpv2.TypeSynced).Status != corev1.ConditionTrue {
				t.Error("completed deletion should report successful reconciliation")
			}
			if got := xr.GetLastHandledReconcileAt(); got != "deleting" {
				t.Errorf("handled reconcile token = %q, want deleting", got)
			}
			for _, want := range tc.conditions {
				got := xr.GetCondition(xpv2.ConditionType(want.GetType()))
				if got.Status != corev1.ConditionTrue || got.Reason != xpv2.ConditionReason(want.GetReason()) {
					t.Errorf("completed deletion condition %s = %+v, want True/%s", want.GetType(), got, want.GetReason())
				}
			}
		})
	}
}

func TestCompositionStatusOwnedConditionEncoding(t *testing.T) {
	api, gvk := newCompositionStatusAPI(t, v1.CompositeResourceScopeLegacyCluster)
	xr := composite.New(composite.WithGroupVersionKind(gvk), composite.WithSchema(composite.SchemaLegacy))
	xr.SetName("encoded-condition")
	if err := api.Create(t.Context(), xr); err != nil {
		t.Fatal(err)
	}
	condition := xpv2.Condition{Type: "Checked<&>", Status: corev1.ConditionTrue, Reason: "Verified", LastTransitionTime: metav1.Now()}
	xr.SetConditions(condition, xpv2.Available())
	//nolint:staticcheck // Seed a condition owned by the existing status applier.
	if err := api.Status().Patch(t.Context(), xr, client.Apply, client.FieldOwner(FieldOwnerXR)); err != nil {
		t.Fatal(err)
	}
	before, _, _ := kunstructured.NestedFieldCopy(xr.Object, "status")
	r := NewReconciler(api, gvk)
	if err := r.writeCompositionStatus(t.Context(), xr, CompositionResult{DesiredStatus: map[string]any{}}, before); err != nil {
		t.Fatal(err)
	}
	if err := api.Get(t.Context(), client.ObjectKeyFromObject(xr), xr); err != nil {
		t.Fatal(err)
	}
	if got := xr.GetCondition(condition.Type); !got.Equal(condition) {
		t.Errorf("previously applied condition = %+v, want %+v", got, condition)
	}
}
