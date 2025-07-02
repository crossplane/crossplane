package xpkg

import (
	"context"
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
