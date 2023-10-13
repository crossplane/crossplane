package render

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

type mockPullClient struct {
	MockPullImage func(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error)
}

func (m *mockPullClient) ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error) {
	return m.MockPullImage(ctx, ref, options)
}

var _ pullClient = &mockPullClient{}

func TestGetRuntimeDocker(t *testing.T) {
	type args struct {
		fn v1beta1.Function
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
				fn: v1beta1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							AnnotationKeyRuntimeDockerCleanup:    string(AnnotationValueRuntimeDockerCleanupOrphan),
							AnnotationKeyRuntimeDockerPullPolicy: string(AnnotationValueRuntimeDockerPullPolicyAlways),
							AnnotationKeyRuntimeDockerImage:      "test-image-from-annotation",
						},
					},
					Spec: v1beta1.FunctionSpec{
						PackageSpec: v1.PackageSpec{
							Package: "test-package",
						},
					},
				},
			},
			want: want{
				rd: &RuntimeDocker{
					Image:      "test-image-from-annotation",
					Stop:       false,
					PullPolicy: AnnotationValueRuntimeDockerPullPolicyAlways,
				},
			},
		},
		"SuccessDefaults": {
			reason: "should return a RuntimeDocker with default fields set if no annotation are set",
			args: args{
				fn: v1beta1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
					Spec: v1beta1.FunctionSpec{
						PackageSpec: v1.PackageSpec{
							Package: "test-package",
						},
					},
				},
			},
			want: want{
				rd: &RuntimeDocker{
					Image:      "test-package",
					Stop:       true,
					PullPolicy: AnnotationValueRuntimeDockerPullPolicyIfNotPresent,
				},
			},
		},
		"ErrorUnknownAnnotationValueCleanup": {
			reason: "should return an error if the supplied Function has an unknown cleanup annotation value",
			args: args{
				fn: v1beta1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							AnnotationKeyRuntimeDockerCleanup: "wrong",
						},
					},
					Spec: v1beta1.FunctionSpec{
						PackageSpec: v1.PackageSpec{
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
				fn: v1beta1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							AnnotationKeyRuntimeDockerPullPolicy: "wrong",
						},
					},
					Spec: v1beta1.FunctionSpec{
						PackageSpec: v1.PackageSpec{
							Package: "test-package",
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rd, err := GetRuntimeDocker(tc.args.fn)
			if diff := cmp.Diff(tc.want.rd, rd); diff != "" {
				t.Errorf("\n%s\nGetRuntimeDocker(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetRuntimeDocker(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
