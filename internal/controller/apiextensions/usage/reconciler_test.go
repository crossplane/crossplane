/*
Copyright 2023 The Crossplane Authors.

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

package usage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

type fakeSelectorResolver struct {
	resourceSelectorFn func(ctx context.Context, u *v1alpha1.Usage) error
}

func (f fakeSelectorResolver) resolveSelectors(ctx context.Context, u *v1alpha1.Usage) error {
	return f.resourceSelectorFn(ctx, u)
}

func TestReconcile(t *testing.T) {
	now := metav1.Now()
	reason := "protected"
	const resourceName = "using"

	type args struct {
		mgr  manager.Manager
		opts []ReconcilerOption
	}
	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UsageNotFound": {
			reason: "We should not return an error if the Usage was not found.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"CannotResolveSelectors": {
			reason: "We should return an error if we cannot resolve selectors.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								o := obj.(*v1alpha1.Usage)
								o.Spec.Of.ResourceSelector = &v1alpha1.ResourceSelector{MatchLabels: map[string]string{"foo": "bar"}}
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errResolveSelectors),
			},
		},
		"CannotAddFinalizer": {
			reason: "We should return an error if we cannot add finalizer.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								o := obj.(*v1alpha1.Usage)
								o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddFinalizer),
			},
		},
		"CannotAddDetailsAnnotation": {
			reason: "We should return an error if we cannot add details annotation.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								o := obj.(*v1alpha1.Usage)
								o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(errBoom),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddDetailsAnnotation),
			},
		},
		"CannotGetUsedResource": {
			reason: "We should return an error if we cannot get used resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								switch o := obj.(type) {
								case *v1alpha1.Usage:
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
								case *composed.Unstructured:
									return errBoom
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetUsed),
			},
		},
		"CannotUpdateUsedWithInUseLabel": {
			reason: "We should return an error if we cannot update used resource with in-use label",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								switch o := obj.(type) {
								case *v1alpha1.Usage:
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
								case *composed.Unstructured:
									return nil
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								if _, ok := obj.(*composed.Unstructured); ok {
									return errBoom
								}
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddInUseLabel),
			},
		},
		"CannotGetUsingResource": {
			reason: "We should return an error if we cannot get using resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								switch o := obj.(type) {
								case *v1alpha1.Usage:
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "used"}
									o.Spec.By = &v1alpha1.Resource{
										ResourceRef: &v1alpha1.ResourceRef{Name: resourceName},
									}
								case *composed.Unstructured:
									if o.GetName() == resourceName {
										return errBoom
									}
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetUsing),
			},
		},
		"CannotAddOwnerRefToUsage": {
			reason: "We should return an error if we cannot add owner reference to the Usage.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "used"}
									o.Spec.By = &v1alpha1.Resource{
										ResourceRef: &v1alpha1.ResourceRef{Name: resourceName},
									}
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetName() == resourceName {
										o.SetAPIVersion("v1")
										o.SetKind("AnotherKind")
										o.SetUID("some-uid")
									}
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								if u, ok := obj.(*v1alpha1.Usage); ok {
									if u.GetOwnerReferences() != nil {
										return errBoom
									}
								}
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddOwnerToUsage),
			},
		},
		"SuccessWithUsingResource": {
			reason: "We should return no error once we have successfully reconciled the usage resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "used"}
									o.Spec.By = &v1alpha1.Resource{
										ResourceRef: &v1alpha1.ResourceRef{Name: resourceName},
									}
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetName() == resourceName {
										o.SetAPIVersion("v1")
										o.SetKind("AnotherKind")
										o.SetUID("some-uid")
									}
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									if o.GetOwnerReferences() != nil {
										owner := o.GetOwnerReferences()[0]
										if owner.APIVersion != "v1" || owner.Kind != "AnotherKind" || owner.UID != "some-uid" {
											t.Errorf("expected owner reference to be set on usage properly")
										}
									}
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
								o := obj.(*v1alpha1.Usage)
								if o.Status.GetCondition(xpv1.TypeReady).Status != corev1.ConditionTrue {
									t.Fatalf("expected ready condition to be true")
								}
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"SuccessNoUsingResource": {
			reason: "We should return no error once we have successfully reconciled the usage resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
									o.Spec.Reason = &reason
									return nil
								}
								if _, ok := obj.(*composed.Unstructured); ok {
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetLabels()[inUseLabelKey] != "true" {
										t.Fatalf("expected %s label to be true", inUseLabelKey)
									}
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
								o := obj.(*v1alpha1.Usage)
								if o.Status.GetCondition(xpv1.TypeReady).Status != corev1.ConditionTrue {
									t.Fatalf("expected ready condition to be true")
								}
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"PauseReconcile": {
			reason: "Pause reconciliation if the pause annotation is set.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								obj.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "used"}
									o.Spec.By = &v1alpha1.Resource{
										ResourceRef: &v1alpha1.ResourceRef{Name: resourceName},
									}
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetName() == resourceName {
										o.SetAPIVersion("v1")
										o.SetKind("AnotherKind")
										o.SetUID("some-uid")
									}
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
								rp := xpv1.ReconcilePaused()
								c := obj.(*v1alpha1.Usage).Status.GetCondition(rp.Type)
								if c.Status != rp.Status || c.Reason != rp.Reason {
									return fmt.Errorf("Expected status: %v but got: %v", rp, c)
								}
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ResumeReconcile": {
			reason: "resume reconciliation by remove the pause annotation.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "used"}
									o.Spec.By = &v1alpha1.Resource{
										ResourceRef: &v1alpha1.ResourceRef{Name: resourceName},
									}
									o.Status.SetConditions(xpv1.ReconcilePaused())
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetName() == resourceName {
										o.SetAPIVersion("v1")
										o.SetKind("AnotherKind")
										o.SetUID("some-uid")
									}
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									if o.GetOwnerReferences() != nil {
										owner := o.GetOwnerReferences()[0]
										if owner.APIVersion != "v1" || owner.Kind != "AnotherKind" || owner.UID != "some-uid" {
											t.Errorf("expected owner reference to be set on usage properly")
										}
									}
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
								o := obj.(*v1alpha1.Usage)
								if o.Status.GetCondition(xpv1.TypeReady).Status != corev1.ConditionTrue {
									t.Fatalf("expected ready condition to be true")
								}
								if o.Status.GetCondition(xpv1.TypeSynced).Status != corev1.ConditionUnknown {
									t.Fatal("expected paused condition to be gone")
								}
								return nil
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"CannotRemoveFinalizerOnDelete": {
			reason: "We should return an error if we cannot remove the finalizer on delete.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.SetDeletionTimestamp(&now)
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
									return nil
								}
								if _, ok := obj.(*composed.Unstructured); ok {
									return kerrors.NewNotFound(schema.GroupResource{}, "")
								}
								return errors.New("unexpected object type")
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errRemoveFinalizer),
			},
		},
		"CannotGetUsedOnDelete": {
			reason: "We should return an error if we cannot get used resource on delete.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.SetDeletionTimestamp(&now)
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
									return nil
								}
								if _, ok := obj.(*composed.Unstructured); ok {
									return errBoom
								}
								return errors.New("unexpected object type")
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetUsed),
			},
		},
		"CannotGetUsingOnDelete": {
			reason: "We should return an error if we cannot get using resource on delete.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.SetDeletionTimestamp(&now)
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "used"}
									o.Spec.By = &v1alpha1.Resource{
										APIVersion:  "v1",
										Kind:        "AnotherKind",
										ResourceRef: &v1alpha1.ResourceRef{Name: resourceName},
									}
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetName() == "used" {
										o.SetLabels(map[string]string{inUseLabelKey: "true"})
									}
									return errBoom
								}
								return errors.New("unexpected object type")
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetUsing),
			},
		},
		"CannotListUsagesOnDelete": {
			reason: "We should return an error if we cannot list usages on delete.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.SetDeletionTimestamp(&now)
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									o.SetLabels(map[string]string{inUseLabelKey: "true"})
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								return errBoom
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListUsages),
			},
		},
		"CannotRemoveLabelOnDelete": {
			reason: "We should return an error if we cannot remove in use label on delete.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.SetDeletionTimestamp(&now)
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									o.SetLabels(map[string]string{inUseLabelKey: "true"})
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								return errBoom
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errRemoveInUseLabel),
			},
		},
		"SuccessfulDeleteUsedResourceGone": {
			reason: "We should return no error once we have successfully deleted the usage resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.SetDeletionTimestamp(&now)
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
									return nil
								}
								if _, ok := obj.(*composed.Unstructured); ok {
									return kerrors.NewNotFound(schema.GroupResource{}, "")
								}
								return errors.New("unexpected object type")
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"SuccessfulDeleteUsedLabelRemoved": {
			reason: "We should return no error once we have successfully deleted the usage resource by removing in use label.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.SetDeletionTimestamp(&now)
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "cool"}
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									o.SetLabels(map[string]string{inUseLabelKey: "true"})
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetLabels()[inUseLabelKey] != "" {
										t.Errorf("expected in use label to be removed")
									}
									return nil
								}
								return errors.New("unexpected object type")
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"SuccessfulWaitWhenUsingStillThere": {
			reason: "We should wait until the using resource is deleted.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(xpresource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*v1alpha1.Usage); ok {
									o.SetDeletionTimestamp(&now)
									o.SetLabels(map[string]string{xcrd.LabelKeyNamePrefixForComposed: "some-composite"})
									o.Spec.Of.ResourceRef = &v1alpha1.ResourceRef{Name: "used"}
									o.Spec.By = &v1alpha1.Resource{
										APIVersion:  "v1",
										Kind:        "AnotherKind",
										ResourceRef: &v1alpha1.ResourceRef{Name: resourceName},
									}
									return nil
								}
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetName() == "used" {
										o.SetLabels(map[string]string{inUseLabelKey: "true"})
									}
									o.SetLabels(map[string]string{
										xcrd.LabelKeyNamePrefixForComposed: "some-composite",
									})
									return nil
								}
								return errors.New("unexpected object type")
							}),
							MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composed.Unstructured); ok {
									if o.GetLabels()[inUseLabelKey] != "" {
										t.Errorf("expected in use label to be removed")
									}
									return nil
								}
								return errors.New("unexpected object type")
							}),
						},
					}),
					WithSelectorResolver(fakeSelectorResolver{
						resourceSelectorFn: func(ctx context.Context, u *v1alpha1.Usage) error {
							return nil
						},
					}),
					WithFinalizer(xpresource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ xpresource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: 30 * time.Second},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, tc.args.opts...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want result, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
