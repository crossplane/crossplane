/*
Copyright 2020 The Crossplane Authors.

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

package manager

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

func TestPackagePodManagerSync(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		manager PodManager
		pkg     v1alpha1.Package
	}

	type want struct {
		err  error
		hash string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrGetPod": {
			reason: "Failure to get an existing pod should return an error.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{
				err: errBoom,
			},
		},
		"SuccessfulCreatePod": {
			reason: "Creating a pod should return an empty hash.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(nil),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{},
		},
		"ErrCreatePod": {
			reason: "Failure to create a pod should return an error.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(errBoom),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{
				err: errBoom,
			},
		},
		"SuccessfulDeleteUnhealthyPod": {
			reason: "Unhealthy pods should be delete successfully.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							p := o.(*corev1.Pod)
							pod := corev1.Pod{
								Status: corev1.PodStatus{
									Phase: corev1.PodFailed,
								},
							}
							*p = pod
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(nil),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{},
		},
		"ErrDeleteUnhealthyPod": {
			reason: "Failure to delete an unhealthy pod should return an error.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							p := o.(*corev1.Pod)
							pod := corev1.Pod{
								Status: corev1.PodStatus{
									Phase: corev1.PodFailed,
								},
							}
							*p = pod
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(errBoom),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{
				err: errBoom,
			},
		},
		"SuccessfulDeletePodWrongContainerStatus": {
			reason: "A pod that does not have one container status should be deleted successfully.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							p := o.(*corev1.Pod)
							pod := corev1.Pod{
								Status: corev1.PodStatus{
									Phase: corev1.PodSucceeded,
								},
							}
							*p = pod
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(nil),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{},
		},
		"ErrDeletePodWrongContainerStatus": {
			reason: "Failure to delete a succeeded pod that does one container status should return error.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							p := o.(*corev1.Pod)
							pod := corev1.Pod{
								Status: corev1.PodStatus{
									Phase: corev1.PodSucceeded,
								},
							}
							*p = pod
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(errBoom),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{
				err: errBoom,
			},
		},
		"SuccessfulDeletePodNoHash": {
			reason: "A pod that does not have an image ID for its container should be deleted successfully.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							p := o.(*corev1.Pod)
							pod := corev1.Pod{
								Status: corev1.PodStatus{
									Phase:             corev1.PodSucceeded,
									ContainerStatuses: []corev1.ContainerStatus{{}},
								},
							}
							*p = pod
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(nil),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{},
		},
		"ErrDeletePodNoHash": {
			reason: "Failure to delete a pod that has no image ID for its container status should return error.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							p := o.(*corev1.Pod)
							pod := corev1.Pod{
								Status: corev1.PodStatus{
									Phase:             corev1.PodSucceeded,
									ContainerStatuses: []corev1.ContainerStatus{{}},
								},
							}
							*p = pod
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(errBoom),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{
				err: errBoom,
			},
		},
		"Successful": {
			reason: "A successful run should return a hash and no error.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							p := o.(*corev1.Pod)
							pod := corev1.Pod{
								Status: corev1.PodStatus{
									Phase:             corev1.PodSucceeded,
									ContainerStatuses: []corev1.ContainerStatus{{ImageID: "1234567"}},
								},
							}
							*p = pod
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(nil),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: want{
				hash: "1234567",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			want, err := tc.args.manager.Sync(context.TODO(), tc.args.pkg)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nmanager.Sync(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.hash, want); diff != "" {
				t.Errorf("\n%s\nmanager.Sync(...): -want hash, +got hash:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackagePodManagerGarbageCollect(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		manager PodManager
		src     string
		pkg     v1alpha1.Package
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"ErrGCPod": {
			reason: "Failure to delete pod should return error.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockDelete: test.NewMockDeleteFn(errBoom),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
			want: errBoom,
		},
		"SuccessfulGCPod": {
			reason: "Successfully deleting a pod should not return error.",
			args: args{
				manager: &PackagePodManager{
					client: &test.MockClient{
						MockDelete: test.NewMockDeleteFn(nil),
					},
				},
				pkg: &v1alpha1.Provider{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.manager.GarbageCollect(context.TODO(), tc.args.src, tc.args.pkg)

			if diff := cmp.Diff(tc.want, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nmanager.Run(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
