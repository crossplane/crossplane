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
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestEnqueueForCompositionRevisionFunc(t *testing.T) {
	type args struct {
		of     schema.GroupVersionKind
		reader client.Reader
		event  kevent.CreateEvent
	}
	type want struct {
		added []interface{}
	}

	dog := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Dog"}
	dogList := dog.GroupVersion().WithKind("DogList")

	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoXRs": {
			reason: "If there are no XRs of the specified type, no reconciles should be enqueued.",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
						// test parameters only here, not in the later tests for brevity.
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
			},
		},
		"AutomaticManagementPolicy": {
			reason: "A reconcile should be enqueued for XRs with an automatic revision update policy.",
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
				event: kevent.CreateEvent{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
			want: want{
				added: []interface{}{reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: "ns",
					Name:      "obj1",
				}}},
			},
		},
		"ManualManagementPolicy": {
			reason: "A reconcile shouldn't be enqueued for XRs with a manual revision update policy.",
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
				event: kevent.CreateEvent{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
			want: want{},
		},
		"OtherComposition": {
			reason: "A reconcile shouldn't be enqueued for an XR that references a different Composition",
			args: args{
				of: dog,
				reader: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						var obj1 composite.Unstructured
						obj1.SetNamespace("ns")
						obj1.SetName("obj1")
						policy := xpv1.UpdateAutomatic
						obj1.SetCompositionUpdatePolicy(&policy)
						obj1.SetCompositionReference(&corev1.ObjectReference{Name: "bernese"})

						list.(*kunstructured.UnstructuredList).Items = []kunstructured.Unstructured{obj1.Unstructured}

						return nil
					},
				},
				event: kevent.CreateEvent{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
			want: want{},
		},
		"Multiple": {
			reason: "Reconciles should be enqueued only for the XRs that reference the relevant Composition, and have an automatic composition revision update policy.",
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
				event: kevent.CreateEvent{
					Object: &v1.CompositionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dachshund-sadfa8",
							Labels: map[string]string{
								v1.LabelCompositionName: "dachshund",
							},
						},
					},
				},
			},
			want: want{
				added: []interface{}{
					reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "obj1"}},
					reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "obj2"}},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fns := EnqueueForCompositionRevision(resource.CompositeKind(tc.args.of), tc.args.reader, logging.NewNopLogger())
			q := rateLimitingQueueMock{}
			fns.Create(context.TODO(), tc.args.event, &q)

			if diff := cmp.Diff(tc.want.added, q.added); diff != "" {
				t.Errorf("\n%s\nfns.Create(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

type rateLimitingQueueMock struct {
	workqueue.RateLimitingInterface
	added []interface{}
}

func (f *rateLimitingQueueMock) Add(item interface{}) {
	f.added = append(f.added, item)
}
