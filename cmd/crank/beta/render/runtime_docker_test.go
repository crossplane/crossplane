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

package render

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

type mockPullClient struct {
	MockPullImage func(_ context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error)
}

func (m *mockPullClient) ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error) {
	return m.MockPullImage(ctx, ref, options)
}

var _ pullClient = &mockPullClient{}

func TestGetRuntimeDocker(t *testing.T) {
	type args struct {
		fn pkgv1.Function
	}
	type want struct {
		rd  *RuntimeDocker
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessAllSet": {
			reason: "should return a RuntimeDocker with all fields set according to the supplied Function's annotations",
			args: args{
				fn: pkgv1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							AnnotationKeyRuntimeDockerCleanup:    string(AnnotationValueRuntimeDockerCleanupOrphan),
							AnnotationKeyRuntimeDockerPullPolicy: string(AnnotationValueRuntimeDockerPullPolicyAlways),
							AnnotationKeyRuntimeDockerImage:      "test-image-from-annotation",
						},
					},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "test-package",
						},
					},
				},
			},
			want: want{
				rd: &RuntimeDocker{
					Image:      "test-image-from-annotation",
					Cleanup:    AnnotationValueRuntimeDockerCleanupOrphan,
					PullPolicy: AnnotationValueRuntimeDockerPullPolicyAlways,
				},
			},
		},
		"SuccessDefaults": {
			reason: "should return a RuntimeDocker with default fields set if no annotation are set",
			args: args{
				fn: pkgv1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "test-package",
						},
					},
				},
			},
			want: want{
				rd: &RuntimeDocker{
					Image:      "test-package",
					Cleanup:    AnnotationValueRuntimeDockerCleanupRemove,
					PullPolicy: AnnotationValueRuntimeDockerPullPolicyIfNotPresent,
				},
			},
		},
		"ErrorUnknownAnnotationValueCleanup": {
			reason: "should return an error if the supplied Function has an unknown cleanup annotation value",
			args: args{
				fn: pkgv1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							AnnotationKeyRuntimeDockerCleanup: "wrong",
						},
					},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "test-package",
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ErrorUnknownAnnotationPullPolicy": {
			reason: "should return an error if the supplied Function has an unknown pull policy annotation value",
			args: args{
				fn: pkgv1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							AnnotationKeyRuntimeDockerPullPolicy: "wrong",
						},
					},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "test-package",
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"AnnotationsCleanupSetToStop": {
			reason: "should return a RuntimeDocker with all fields set according to the supplied Function's annotations",
			args: args{
				fn: pkgv1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							AnnotationKeyRuntimeDockerCleanup: string(AnnotationValueRuntimeDockerCleanupStop),
						},
					},
					Spec: pkgv1.FunctionSpec{
						PackageSpec: pkgv1.PackageSpec{
							Package: "test-package",
						},
					},
				},
			},
			want: want{
				rd: &RuntimeDocker{
					Image:      "test-package",
					Cleanup:    AnnotationValueRuntimeDockerCleanupStop,
					PullPolicy: AnnotationValueRuntimeDockerPullPolicyIfNotPresent,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rd, err := GetRuntimeDocker(tc.args.fn, logging.NewNopLogger())
			if diff := cmp.Diff(tc.want.rd, rd, cmpopts.IgnoreUnexported(RuntimeDocker{})); diff != "" {
				t.Errorf("\n%s\nGetRuntimeDocker(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetRuntimeDocker(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
