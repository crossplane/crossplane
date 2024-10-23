package xpkg

import (
	"context"
	"crypto"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/sigstore/policy-controller/pkg/apis/policy/v1alpha1"
	cosign "github.com/sigstore/policy-controller/pkg/webhook/clusterimagepolicy"
	"knative.dev/pkg/apis"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

var errBoom = errors.New("boom")

func TestImageConfigStoreBestMatch(t *testing.T) {
	type args struct {
		client  client.Client
		image   string
		isValid isValidConfig
	}
	type want struct {
		config *v1beta1.ImageConfig
		err    error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"ErrListConfig": {
			args: args{
				image: "registry1.com/acme-co/configuration-foo",
				isValid: func(_ *v1beta1.ImageConfig) bool {
					return true
				},
				client: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListImageConfigs),
			},
		},
		"SingleConfig": {
			args: args{
				image: "registry1.com/acme-co/configuration-foo",
				isValid: func(_ *v1beta1.ImageConfig) bool {
					return true
				},
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						*list.(*v1beta1.ImageConfigList) = v1beta1.ImageConfigList{
							Items: []v1beta1.ImageConfig{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry1.com/acme-co"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
							},
						}
						return nil
					},
				},
			},
			want: want{
				config: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry1",
					},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "registry1.com/acme-co"},
						},
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "test"},
							},
						},
					},
				},
			},
		},
		"SingleConfigMultiPrefix": {
			args: args{
				image: "registry1.com/acme-co/configuration-foo",
				isValid: func(_ *v1beta1.ImageConfig) bool {
					return true
				},
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						*list.(*v1beta1.ImageConfigList) = v1beta1.ImageConfigList{
							Items: []v1beta1.ImageConfig{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry1.com/some-other"},
											{Prefix: "registry1.com/acme-co"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
							},
						}
						return nil
					},
				},
			},
			want: want{
				config: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry1",
					},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "registry1.com/some-other"},
							{Prefix: "registry1.com/acme-co"},
						},
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "test"},
							},
						},
					},
				},
			},
		},
		"MultiConfig": {
			args: args{
				image: "registry1.com/acme-co/configuration-foo",
				isValid: func(_ *v1beta1.ImageConfig) bool {
					return true
				},
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						*list.(*v1beta1.ImageConfigList) = v1beta1.ImageConfigList{
							Items: []v1beta1.ImageConfig{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry2",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry2.com/acme-co"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry1.com/acme-co"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
							},
						}
						return nil
					},
				},
			},
			want: want{
				config: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry1",
					},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "registry1.com/acme-co"},
						},
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "test"},
							},
						},
					},
				},
			},
		},
		"MultiConfigMultiPrefix": {
			args: args{
				image: "registry1.com/acme-co/configuration-foo",
				isValid: func(_ *v1beta1.ImageConfig) bool {
					return true
				},
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						*list.(*v1beta1.ImageConfigList) = v1beta1.ImageConfigList{
							Items: []v1beta1.ImageConfig{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry2",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry2.com/some-other"},
											{Prefix: "registry2.com/acme-co"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry1.com/some-other"},
											{Prefix: "registry1.com/acme-co"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
							},
						}
						return nil
					},
				},
			},
			want: want{
				config: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry1",
					},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "registry1.com/some-other"},
							{Prefix: "registry1.com/acme-co"},
						},
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "test"},
							},
						},
					},
				},
			},
		},
		"MultiConfigMultiMatchFindsBest": {
			args: args{
				image: "registry1.com/acme-co/configuration-foo",
				isValid: func(_ *v1beta1.ImageConfig) bool {
					return true
				},
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						*list.(*v1beta1.ImageConfigList) = v1beta1.ImageConfigList{
							Items: []v1beta1.ImageConfig{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1-base",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry1.com"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1-with-org",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry1.com/some-other"},
											{Prefix: "registry1.com/acme-co"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1-full-image-ref",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry1.com/some-other"},
											{Prefix: "registry1.com/acme-co/configuration-foo"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
							},
						}
						return nil
					},
				},
			},
			want: want{
				config: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry1-full-image-ref",
					},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "registry1.com/some-other"},
							{Prefix: "registry1.com/acme-co/configuration-foo"},
						},
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "test"},
							},
						},
					},
				},
			},
		},
		"SkipsInvalid": {
			args: args{
				image: "registry1.com/acme-co/configuration-foo",
				isValid: func(c *v1beta1.ImageConfig) bool {
					return c.Spec.Registry != nil
				},
				client: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						*list.(*v1beta1.ImageConfigList) = v1beta1.ImageConfigList{
							Items: []v1beta1.ImageConfig{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1-no-pull-secret",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											// Best match but invalid, no pull secret defined
											{Prefix: "registry1.com/acme-co"},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "registry1",
									},
									Spec: v1beta1.ImageConfigSpec{
										MatchImages: []v1beta1.ImageMatch{
											{Prefix: "registry1.com"},
										},
										Registry: &v1beta1.RegistryConfig{
											Authentication: &v1beta1.RegistryAuthentication{
												PullSecretRef: corev1.LocalObjectReference{Name: "test"},
											},
										},
									},
								},
							},
						}
						return nil
					},
				},
			},
			want: want{
				config: &v1beta1.ImageConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry1",
					},
					Spec: v1beta1.ImageConfigSpec{
						MatchImages: []v1beta1.ImageMatch{
							{Prefix: "registry1.com"},
						},
						Registry: &v1beta1.RegistryConfig{
							Authentication: &v1beta1.RegistryAuthentication{
								PullSecretRef: corev1.LocalObjectReference{Name: "test"},
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &ImageConfigStore{
				client: tc.args.client,
			}
			got, err := s.bestMatch(context.Background(), tc.args.image, tc.args.isValid)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("bestMatch() error -want +got: %s", diff)
			}
			if diff := cmp.Diff(tc.want.config, got, cmp.AllowUnexported(v1beta1.ImageConfig{})); diff != "" {
				t.Errorf("bestMatch() config -want +got: %s", diff)
			}
		})
	}
}

func TestCosignPolicy(t *testing.T) {
	secretNamespace := "test-namespace"
	type args struct {
		from    *v1beta1.CosignVerificationConfig
		secrets []client.Object
	}
	type want struct {
		out *cosign.ClusterImagePolicy
		err error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"KeyfullPolicy": {
			args: args{
				from: &v1beta1.CosignVerificationConfig{
					Authorities: []v1beta1.CosignAuthority{
						{
							Name: "verify signed with key",
							Key: &v1beta1.KeyRef{
								SecretRef: &v1beta1.LocalSecretKeySelector{
									LocalSecretReference: xpv1.LocalSecretReference{
										Name: "cosign-public-key",
									},
									Key: "cosign.pub",
								},
							},
						},
					},
				},
				secrets: []client.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cosign-public-key",
							Namespace: secretNamespace,
						},
						Data: map[string][]byte{
							"cosign.pub": []byte(`-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDjVkz6tbJd1RGzAdx1hmZHWh52J4
VG1xGzbFfDEhojXmFodEXLotz8qe1okeSpP0yZxjM0a8z08ljrxLgiFaPg==
-----END PUBLIC KEY-----`),
						},
					},
				},
			},
			want: want{
				out: &cosign.ClusterImagePolicy{
					Authorities: []cosign.Authority{
						{
							Name: "verify signed with key",
							Key: &cosign.KeyRef{
								Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDjVkz6tbJd1RGzAdx1hmZHWh52J4
VG1xGzbFfDEhojXmFodEXLotz8qe1okeSpP0yZxjM0a8z08ljrxLgiFaPg==
-----END PUBLIC KEY-----`,
								HashAlgorithm:     "sha256",
								HashAlgorithmCode: crypto.SHA256,
							},
						},
					},
				},
			},
		},
		"KeylessPolicy": {
			args: args{
				from: &v1beta1.CosignVerificationConfig{
					Authorities: []v1beta1.CosignAuthority{
						{
							Name: "verify signed keyless",
							Keyless: &v1beta1.KeylessRef{
								URL: mustParseURL("https://fulcio.sigstore.dev"),
								Identities: []v1beta1.Identity{
									{
										Issuer:  "https://github.com/login/oauth",
										Subject: "turkenh@gmail.com",
									},
								},
							},
							CTLog: &v1beta1.TLog{
								URL: mustParseURL("https://rekor.sigstore.dev"),
							},
						},
					},
				},
			},
			want: want{
				out: &cosign.ClusterImagePolicy{
					Authorities: []cosign.Authority{
						{
							Name: "verify signed keyless",
							Keyless: &cosign.KeylessRef{
								URL: mustParseURL("https://fulcio.sigstore.dev"),
								Identities: []v1alpha1.Identity{
									{
										Issuer:  "https://github.com/login/oauth",
										Subject: "turkenh@gmail.com",
									},
								},
							},
							CTLog: &v1alpha1.TLog{
								URL: mustParseURL("https://rekor.sigstore.dev"),
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		kube := fake.NewClientBuilder().WithObjects(tc.args.secrets...).Build()

		t.Run(name, func(t *testing.T) {
			got, err := cosignPolicy(context.Background(), kube, secretNamespace, tc.args.from)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("cosignPolicy() error -want +got: %s", diff)
			}

			if diff := cmp.Diff(tc.want.out, got, ignorePublicKeys()); diff != "" {
				t.Errorf("cosignPolicy() out -want +got: %s", diff)
			}
		})
	}
}

func ignorePublicKeys() cmp.Option {
	return cmp.FilterPath(func(p cmp.Path) bool {
		if p.String() == "Authorities.Key.PublicKeys" {
			return true
		}
		return false
	}, cmp.Ignore())
}

func mustParseURL(raw string) *apis.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	au := apis.URL(*u)
	return &au
}
