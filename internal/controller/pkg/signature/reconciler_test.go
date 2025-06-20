package signature

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	xpkgfake "github.com/crossplane/crossplane/internal/xpkg/fake"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	imageConfigName := "test-config"

	type args struct {
		client client.Client
		opts   []ReconcilerOption
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
		"RevisionNotFound": {
			reason: "If the revision does not exist, we should not return an error.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
			},
		},
		"FailedToGetRevision": {
			reason: "If we fail to get the revision, we should return an error.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
				},
				client: &test.MockClient{
					MockGet:          test.NewMockGetFn(errBoom),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetRevision),
			},
		},
		"IgnoreInactiveRevision": {
			reason: "If the revision is not active, we should skip verification.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision(withDesiredState(v1.PackageRevisionInactive))
						return nil
					}),
				},
			},
		},
		"IgnoreAlreadyVerified": {
			reason: "An already verified revision should be ignored.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision(withConditions(v1.VerificationSucceeded(imageConfigName)))
						return nil
					}),
				},
			},
		},
		"IgnoreAlreadySkipped": {
			reason: "An already skipped revision should be ignored.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision(withConditions(v1.VerificationSkipped()))
						return nil
					}),
				},
			},
		},
		"FailedToGetImageVerificationConfig": {
			reason: "If we fail to get the image verification config, we should return an error.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockImageVerificationConfigFor: xpkgfake.NewMockConfigStoreImageVerificationConfigForFn("", nil, errBoom),
					}),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision()
						return nil
					}),
					MockStatusUpdate: func(_ context.Context, o client.Object, _ ...client.SubResourceUpdateOption) error {
						want := testRevision(withConditions(v1.VerificationIncomplete(errors.Wrap(errBoom, errGetVerificationConfig))))
						if diff := cmp.Diff(&want, o); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			want: want{err: errors.Wrap(errBoom, errGetVerificationConfig)},
		},
		"WaitForImageResolution": {
			reason: "We should wait if the revision controller has not yet resolved the source.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision(withResolvedSource(""))
						return nil
					}),
				},
			},
		},
		"NoMatchingVerificationConfig": {
			reason: "If there is no matching image verification config, we should skip verification.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockImageVerificationConfigFor: xpkgfake.NewMockConfigStoreImageVerificationConfigForFn("", nil, nil),
					}),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision()
						return nil
					}),
					MockStatusUpdate: func(_ context.Context, o client.Object, _ ...client.SubResourceUpdateOption) error {
						want := testRevision(withConditions(v1.VerificationSkipped()))

						if diff := cmp.Diff(&want, o); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
		},
		"FailedToParseImageSource": {
			reason: "If we fail to parse the image source, we should return an error.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockImageVerificationConfigFor: xpkgfake.NewMockConfigStoreImageVerificationConfigForFn(imageConfigName, &v1beta1.ImageVerification{
							Provider: v1beta1.ImageVerificationProviderCosign,
							Cosign:   &v1beta1.CosignVerificationConfig{},
						}, nil),
					}),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision(withResolvedSource("0"))
						return nil
					}),
					MockStatusUpdate: func(_ context.Context, o client.Object, _ ...client.SubResourceUpdateOption) error {
						want := testRevision(
							withResolvedSource("0"),
							withConditions(v1.VerificationIncomplete(errors.Wrap(errors.New("could not parse reference: 0"), errParseReference))),
							withAppliedImageConfigRef(imageConfigName),
						)
						if diff := cmp.Diff(&want, o); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			want: want{err: errors.Wrap(errors.New("could not parse reference: 0"), errParseReference)},
		},
		"ErrGetConfigPullSecrets": {
			reason: "If we fail to get the pull secret for the image config, we should return an error.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn(imageConfigName, "", errBoom),
						MockImageVerificationConfigFor: xpkgfake.NewMockConfigStoreImageVerificationConfigForFn(imageConfigName, &v1beta1.ImageVerification{
							Provider: v1beta1.ImageVerificationProviderCosign,
							Cosign:   &v1beta1.CosignVerificationConfig{},
						}, nil),
					}),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision()
						return nil
					}),
					MockStatusUpdate: func(_ context.Context, o client.Object, _ ...client.SubResourceUpdateOption) error {
						want := testRevision(
							withConditions(v1.VerificationIncomplete(errors.Wrap(errBoom, errGetConfigPullSecret))),
							withAppliedImageConfigRef(imageConfigName),
						)

						if diff := cmp.Diff(&want, o); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			want: want{err: errors.Wrap(errBoom, errGetConfigPullSecret)},
		},
		"AllPullSecretsPassed": {
			reason: "Validate that we pass all the pull secrets, both from package api and image config, to the validator.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn(imageConfigName, "pull-secret-from-image-config", nil),
						MockImageVerificationConfigFor: xpkgfake.NewMockConfigStoreImageVerificationConfigForFn(imageConfigName, &v1beta1.ImageVerification{
							Provider: v1beta1.ImageVerificationProviderCosign,
							Cosign:   &v1beta1.CosignVerificationConfig{},
						}, nil),
					}),
					WithValidator(&MockValidator{
						ValidateFn: func(_ context.Context, _ name.Reference, _ *v1beta1.ImageVerification, pullSecrets ...string) error {
							expected := []string{"pull-secret-from-package", "pull-secret-from-image-config"}

							if diff := cmp.Diff(expected, pullSecrets); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}

							return nil
						},
					}),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision(withPullSecrets([]corev1.LocalObjectReference{{Name: "pull-secret-from-package"}}))
						return nil
					}),
					MockStatusUpdate: func(_ context.Context, o client.Object, _ ...client.SubResourceUpdateOption) error {
						want := testRevision(
							withPullSecrets([]corev1.LocalObjectReference{{Name: "pull-secret-from-package"}}),
							withConditions(v1.VerificationSucceeded(imageConfigName)),
							withAppliedImageConfigRef(imageConfigName),
						)

						if diff := cmp.Diff(&want, o); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
		},
		"FailedVerification": {
			reason: "If verification fails, we should mark the revision as failed.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn(imageConfigName, "", nil),
						MockImageVerificationConfigFor: xpkgfake.NewMockConfigStoreImageVerificationConfigForFn(imageConfigName, &v1beta1.ImageVerification{
							Provider: v1beta1.ImageVerificationProviderCosign,
							Cosign:   &v1beta1.CosignVerificationConfig{},
						}, nil),
					}),
					WithValidator(&MockValidator{
						ValidateFn: func(_ context.Context, _ name.Reference, _ *v1beta1.ImageVerification, _ ...string) error {
							return errBoom
						},
					}),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision()
						return nil
					}),
					MockStatusUpdate: func(_ context.Context, o client.Object, _ ...client.SubResourceUpdateOption) error {
						want := testRevision(
							withConditions(v1.VerificationFailed(imageConfigName, errBoom)),
							withAppliedImageConfigRef(imageConfigName),
						)

						if diff := cmp.Diff(&want, o); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			want: want{err: errors.Wrap(errBoom, errFailedVerification)},
		},
		"SuccessfulVerification": {
			reason: "A successful verification should return a result with no error.",
			args: args{
				opts: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn(imageConfigName, "", nil),
						MockImageVerificationConfigFor: xpkgfake.NewMockConfigStoreImageVerificationConfigForFn(imageConfigName, &v1beta1.ImageVerification{
							Provider: v1beta1.ImageVerificationProviderCosign,
							Cosign:   &v1beta1.CosignVerificationConfig{},
						}, nil),
					}),
					WithValidator(&MockValidator{
						ValidateFn: func(_ context.Context, _ name.Reference, _ *v1beta1.ImageVerification, _ ...string) error {
							return nil
						},
					}),
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						*o.(*v1.ConfigurationRevision) = testRevision()
						return nil
					}),
					MockStatusUpdate: func(_ context.Context, o client.Object, _ ...client.SubResourceUpdateOption) error {
						want := testRevision(
							withConditions(v1.VerificationSucceeded(imageConfigName)),
							withAppliedImageConfigRef(imageConfigName),
						)

						if diff := cmp.Diff(&want, o); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.client, tc.args.opts...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

type MockValidator struct {
	ValidateFn func(ctx context.Context, ref name.Reference, config *v1beta1.ImageVerification, pullSecrets ...string) error
}

func (v *MockValidator) Validate(ctx context.Context, ref name.Reference, config *v1beta1.ImageVerification, pullSecrets ...string) error {
	return v.ValidateFn(ctx, ref, config, pullSecrets...)
}

type revisionOption func(r *v1.ConfigurationRevision)

func withResolvedSource(s string) revisionOption {
	return func(r *v1.ConfigurationRevision) {
		r.SetResolvedSource(s)
	}
}

func withDesiredState(s v1.PackageRevisionDesiredState) revisionOption {
	return func(r *v1.ConfigurationRevision) {
		r.SetDesiredState(s)
	}
}

func withConditions(c ...xpv1.Condition) revisionOption {
	return func(r *v1.ConfigurationRevision) {
		r.SetConditions(c...)
	}
}

func withPullSecrets(pullSecrets []corev1.LocalObjectReference) revisionOption {
	return func(r *v1.ConfigurationRevision) {
		r.SetPackagePullSecrets(pullSecrets)
	}
}

func withAppliedImageConfigRef(name string) revisionOption {
	return func(r *v1.ConfigurationRevision) {
		r.SetAppliedImageConfigRefs(v1.ImageConfigRef{
			Name:   name,
			Reason: v1.ImageConfigReasonVerify,
		})
	}
}

func testRevision(opts ...revisionOption) v1.ConfigurationRevision {
	r := v1.ConfigurationRevision{}
	r.SetResolvedSource("xpkg.crossplane.io/crossplane/signature-verification-unit-test:v0.0.1")
	r.SetDesiredState(v1.PackageRevisionActive)

	for _, o := range opts {
		o(&r)
	}

	return r
}
