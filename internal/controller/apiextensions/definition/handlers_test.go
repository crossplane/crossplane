package definition

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
)

func TestCompositionRevisionMapFunc(t *testing.T) {
	type args struct {
		of     schema.GroupVersionKind
		schema composite.Schema
		reader client.Reader
		obj    client.Object
	}

	type want struct {
		requests []reconcile.Request
	}

	dog := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Dog"}
	dogList := dog.GroupVersion().WithKind("DogList")

	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoXRs": {
			reason: "If there are no XRs of the specified type, no reconciles should be returned.",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
						u, ok := list.(*kunstructured.UnstructuredList)
						if !ok {
							t.Errorf("list was not an UnstructuredList")
						} else if got := u.GroupVersionKind(); got != dogList {
							t.Errorf("list was not a DogList, got: %s", got)
						}
						if len(opts) != 0 {
							t.Errorf("unexpected list options: %#v", opts)
						}
						return nil
					},
				},
				obj: &v1.CompositionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dachshund-sadfa8",
						Labels: map[string]string{
							v1.LabelCompositionName: "dachshund",
						},
					},
					Spec: v1.CompositionRevisionSpec{
						CompositeTypeRef: v1.TypeReferenceTo(dog),
					},
				},
			},
			want: want{
				requests: []reconcile.Request{},
			},
		},
		"AutomaticManagementPolicy": {
			reason: "A reconcile should be returned for XRs with an automatic revision update policy.",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						var obj1 composite.Unstructured
						obj1.SetNamespace("ns")
						obj1.SetName("obj1")
						policy := xpv1.UpdateAutomatic
						obj1.SetCompositionUpdatePolicy(&policy)
						obj1.SetCompositionReference(&corev1.ObjectReference{Name: "dachshund"})

						list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{obj1.Unstructured}

						return nil
					},
				},
				obj: &v1.CompositionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dachshund-sadfa8",
						Labels: map[string]string{
							v1.LabelCompositionName: "dachshund",
						},
					},
					Spec: v1.CompositionRevisionSpec{
						CompositeTypeRef: v1.TypeReferenceTo(dog),
					},
				},
			},
			want: want{
				requests: []reconcile.Request{{NamespacedName: types.NamespacedName{
					Namespace: "ns",
					Name:      "obj1",
				}}},
			},
		},
		"ManualManagementPolicy": {
			reason: "A reconcile shouldn't be returned for XRs with a manual revision update policy.",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						var obj1 composite.Unstructured
						obj1.SetNamespace("ns")
						obj1.SetName("obj1")
						policy := xpv1.UpdateManual
						obj1.SetCompositionUpdatePolicy(&policy)
						obj1.SetCompositionReference(&corev1.ObjectReference{Name: "dachshund"})

						list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{obj1.Unstructured}

						return nil
					},
				},
				obj: &v1.CompositionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dachshund-sadfa8",
						Labels: map[string]string{
							v1.LabelCompositionName: "dachshund",
						},
					},
					Spec: v1.CompositionRevisionSpec{
						CompositeTypeRef: v1.TypeReferenceTo(dog),
					},
				},
			},
			want: want{
				requests: []reconcile.Request{},
			},
		},
		"Multiple": {
			reason: "Reconciles should be returned only for the XRs that reference the relevant Composition, and have an automatic composition revision update policy.",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						var obj1 composite.Unstructured
						obj1.SetNamespace("ns")
						obj1.SetName("obj1")
						automatic := xpv1.UpdateAutomatic
						obj1.SetCompositionUpdatePolicy(&automatic)
						obj1.SetCompositionReference(&corev1.ObjectReference{Name: "dachshund"})

						obj2 := obj1.DeepCopy()
						obj2.SetName("obj2")

						obj3 := obj1.DeepCopy()
						obj3.SetName("obj3")
						obj3.SetCompositionReference(&corev1.ObjectReference{Name: "bernese"})

						obj4 := obj1.DeepCopy()
						obj4.SetName("obj4")
						manual := xpv1.UpdateManual
						obj4.SetCompositionUpdatePolicy(&manual)

						list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{
							obj1.Unstructured,
							obj2.Unstructured,
							obj3.Unstructured,
						}

						return nil
					},
				},
				obj: &v1.CompositionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dachshund-sadfa8",
						Labels: map[string]string{
							v1.LabelCompositionName: "dachshund",
						},
					},
					Spec: v1.CompositionRevisionSpec{
						CompositeTypeRef: v1.TypeReferenceTo(dog),
					},
				},
			},
			want: want{
				requests: []reconcile.Request{
					{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "obj1"}},
					{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "obj2"}},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mapFunc := CompositionRevisionMapFunc(tc.args.of, tc.args.schema, tc.args.reader, logging.NewNopLogger())
			requests := mapFunc(context.TODO(), tc.args.obj)

			if diff := cmp.Diff(tc.want.requests, requests); diff != "" {
				t.Errorf("\n%s\nCompositionRevisionMapFunc(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourcesMapFunc(t *testing.T) {
	type args struct {
		of     schema.GroupVersionKind
		reader client.Reader
		obj    client.Object
	}

	type want struct {
		requests []reconcile.Request
	}

	dog := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Dog"}
	bucket := schema.GroupVersionKind{Group: "s3.aws.crossplane.io", Version: "v1alpha1", Kind: "Bucket"}

	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoXRs": {
			reason: "If there are no XRs that reference the composed resource, no reconciles should be returned.",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return nil
					},
				},
				obj: &kunstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": bucket.GroupVersion().String(),
						"kind":       bucket.Kind,
						"metadata": map[string]any{
							"name":      "my-bucket",
							"namespace": "default",
						},
					},
				},
			},
			want: want{
				requests: []reconcile.Request{},
			},
		},
		"SingleXR": {
			reason: "A reconcile should be returned for XRs that reference the composed resource.",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						var xr1 kunstructured.Unstructured
						xr1.SetName("xr1")
						xr1.SetNamespace("")

						list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{xr1}
						return nil
					},
				},
				obj: &kunstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": bucket.GroupVersion().String(),
						"kind":       bucket.Kind,
						"metadata": map[string]any{
							"name":      "my-bucket",
							"namespace": "default",
						},
					},
				},
			},
			want: want{
				requests: []reconcile.Request{{NamespacedName: types.NamespacedName{
					Name: "xr1",
				}}},
			},
		},
		"MultipleXRs": {
			reason: "Reconciles should be returned for all XRs that reference the composed resource.",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						var xr1 kunstructured.Unstructured
						xr1.SetName("xr1")
						xr1.SetNamespace("")

						var xr2 kunstructured.Unstructured
						xr2.SetName("xr2")
						xr2.SetNamespace("")

						list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{xr1, xr2}
						return nil
					},
				},
				obj: &kunstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": bucket.GroupVersion().String(),
						"kind":       bucket.Kind,
						"metadata": map[string]any{
							"name":      "my-bucket",
							"namespace": "default",
						},
					},
				},
			},
			want: want{
				requests: []reconcile.Request{
					{NamespacedName: types.NamespacedName{Name: "xr1"}},
					{NamespacedName: types.NamespacedName{Name: "xr2"}},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set the GVK on the object since MapFunc needs it
			tc.args.obj.GetObjectKind().SetGroupVersionKind(bucket)

			mapFunc := CompositeResourcesMapFunc(tc.args.of, tc.args.reader, logging.NewNopLogger())
			requests := mapFunc(context.TODO(), tc.args.obj)

			if diff := cmp.Diff(tc.want.requests, requests); diff != "" {
				t.Errorf("\n%s\nCompositeResourcesMapFunc(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSelfMapFunc(t *testing.T) {
	tests := map[string]struct {
		reason string
		obj    client.Object
		want   []reconcile.Request
	}{
		"ClusterScoped": {
			reason: "Should return a reconcile request for the object itself (cluster-scoped).",
			obj: &kunstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name": "my-object",
					},
				},
			},
			want: []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Name: "my-object",
				},
			}},
		},
		"Namespaced": {
			reason: "Should return a reconcile request for the object itself (namespaced).",
			obj: &kunstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name":      "my-object",
						"namespace": "my-namespace",
					},
				},
			},
			want: []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Name:      "my-object",
					Namespace: "my-namespace",
				},
			}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mapFunc := SelfMapFunc()
			requests := mapFunc(context.TODO(), tc.obj)

			if diff := cmp.Diff(tc.want, requests); diff != "" {
				t.Errorf("\n%s\nSelfMapFunc(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
