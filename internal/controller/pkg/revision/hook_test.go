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

package revision

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/initializer"
)

var (
	crossplane  = "v0.11.1"
	providerDep = "crossplane/provider-aws"
	functionDep = "crossplane/function-exec"
	versionDep  = "v0.1.1"

	caSecret           = "crossplane-root-ca"
	tlsServerSecret    = "server-secret"
	tlsClientSecret    = "client-secret"
	tlsSecretNamespace = "crossplane-system"
)

const (
	caCert = `-----BEGIN CERTIFICATE-----
MIIDkTCCAnmgAwIBAgICB+YwDQYJKoZIhvcNAQELBQAwWjEOMAwGA1UEBhMFRWFy
dGgxDjAMBgNVBAgTBUVhcnRoMQ4wDAYDVQQHEwVFYXJ0aDETMBEGA1UEChMKQ3Jv
c3NwbGFuZTETMBEGA1UEAxMKQ3Jvc3NwbGFuZTAeFw0yMzAzMjIxNTMyNTNaFw0z
MzAzMjIxNTMyNTNaMFoxDjAMBgNVBAYTBUVhcnRoMQ4wDAYDVQQIEwVFYXJ0aDEO
MAwGA1UEBxMFRWFydGgxEzARBgNVBAoTCkNyb3NzcGxhbmUxEzARBgNVBAMTCkNy
b3NzcGxhbmUwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDNmbFbNF32
pLxELihBec72qf9fIUl12saK8s6FqvH0uv1vGUbrGMkhvzbdIHo8AJ5N5KKADRe4
ZfDQBESIryFZscbTUkPIlSLWanmBuV3OojZM+G7j38cmN1Kag/fPQ5x5FNg5FhPC
3JCgl3Z/qDLcDDqx/GBgkyfEM11GkLzsJOt/8+8EjcE+mdgwQs3yV4hqUUh3RrM0
wqVDzENfP3PKtnygSQAgp3VxqbHwR2cueemSLClq0JQwNsnpQC+T+Cq8tWkZjdw8
LMJtdbtnOLvM6ofKQA0Sdi4XqaZML1nh0Cv/mGLR9dSDI5Uxl4kGySRE5d0xXC2C
ZUwP6fBuTpaxAgMBAAGjYTBfMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTAD
AQH/MB0GA1UdDgQWBBQ2WbFrZwIu4lWA5tA+l/zWWCV5CDAdBgNVHREEFjAUghJj
cm9zc3BsYW5lLXJvb3QtY2EwDQYJKoZIhvcNAQELBQADggEBAGE4rcSZdWO3E4QY
BfjxBuJfM8VZUP1kllV+IrFO+PhCAFcUSOCdfJcMbdAXbA/m7f2jTHq8isDOYLfn
50/40+myheH/ZAQibC7go1VpjrZHQfanaGEFZPri0ftpQjZ2guCxrxgNA9qZa2Kz
4H1dW4eQCWZnkUOUmBwdp2RN5E0oWVrvqixdcUjmMqGyajkueScuKih6EUYnfUWO
A0N4+bBummJYPRnLNoUsKnEUsUXyQKp2jnYgGH90O71VO6r86tsvhOivwSKVq6E6
r2bka16dVPncliiFI4NBng/SFGyOSE0O1Er/BY38KEALYe7J4mLzr4NxEtib2soM
hs0Mt0k=
-----END CERTIFICATE-----`
	caKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAzZmxWzRd9qS8RC4oQXnO9qn/XyFJddrGivLOharx9Lr9bxlG
6xjJIb823SB6PACeTeSigA0XuGXw0AREiK8hWbHG01JDyJUi1mp5gbldzqI2TPhu
49/HJjdSmoP3z0OceRTYORYTwtyQoJd2f6gy3Aw6sfxgYJMnxDNdRpC87CTrf/Pv
BI3BPpnYMELN8leIalFId0azNMKlQ8xDXz9zyrZ8oEkAIKd1camx8EdnLnnpkiwp
atCUMDbJ6UAvk/gqvLVpGY3cPCzCbXW7Zzi7zOqHykANEnYuF6mmTC9Z4dAr/5hi
0fXUgyOVMZeJBskkROXdMVwtgmVMD+nwbk6WsQIDAQABAoIBAQDExbrDomvnuaRh
0JdAixb0ZqD9Z/tJq3fn1hioP4JQioIxyUxhhxhAjyQwIHw8Xw8jV5Xa3iz8k7wV
KnB5LLvLf2TeLVaoa2urML5X1JQeRouXwRFIUIzmW35YWcNbf8cK71M9145UKgrV
WADWjqEWjzHB1NxcsZoWol48Qhw+GCRP88QN1CyVIXQqFWm+b8YraeUDpBt9FY3R
mrEk4WjcIsQH7fGGIwgQBxzGuZ9iVzHfJUBVUUU92wHr9i3mNPQhfmZqWEkvHhGd
JVgRxIPlyVbTtQ3Zto+nYf53f92YLYORHcUuCOazELjAErhPLjv9LDZZVVYbYbse
vXxNldnBAoGBAO13F3BcxKdFXb7e11zizHaQAoq1QlFdJYq4Shqgj5qp+BZrysEJ
Ai+KpOF3SyvAR4lCHeRDRePKX6abNIdF/ZHIlWP+MNuu35cNEqQE69214kyHlFj2
syOqz2O/CAXNoUeGwFv5prN54MpN4jaXxiXztguT7vtfV1PBUz9Rx9/JAoGBAN2l
5PBweyxC4UxG1ICsPmwE5J436sdgGMaVxnaJ76eB9PrIaadcSwkmZQfsdbMJgV8f
pj6dGdwJOS/rs/CTvlZ3FYCg6L2BKYb/9IMXuMta3VuJR7KpFYRUbkHw9KYacp7y
Pq2B1dmn8xY+83PBQSg4NzqDig3MBc0KtTE3GIOpAoGAcZIzs5mqtBWI8HDDr7kI
8OuPS6fFQAS8n8vkJTgFdoM0FAUZw5j7YqF8mhjj6tjbXdoxUaqbEocHmDdCuC/R
RpgYWuqHk4nfhe7Kq4dvB2qmANQXLzVOGBDpf1suCxh9uifIeDS+dbgkupzlRBby
vdQBjSgDdFX0/inIFtCWN4ECgYEA3RjE3Mt3MtmsIAhvpcMrqVjgLKueuS80x7NT
+57wvuk11IviSJ4aA5CXK2ZGqkeLE7ZggQj5aLKSpyi5n/vg3COCAYOBZrfXEuFz
qOka309OjCbOrHtaCVynd4PCp4auW7tNpopjJfEQ3VoCQ6+9LT+WZ/oa1lR0XOqX
f/Zzr7ECgYBo/oyGxVlOZ51k27m0SB0WQohmxaTDpLl0841vVX62jQpEPr0jropj
CoRJv9VaKVXp8dgkULxiy0C35iGbCLVK5o/qROcRMJlw1rfCM6Gxv7LppqwvmYHI
aAJ/I/MBEGIitV7G1MRwVz56Yvv8cP/mQ712faD7iwBHC9bqO6umCA==
-----END RSA PRIVATE KEY-----`
)

func TestHookPre(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		hook Hooks
		pkg  runtime.Object
		rev  v1.PackageRevision
	}

	type want struct {
		err error
		rev v1.PackageRevision
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrNotProvider": {
			reason: "Should return error if not provider.",
			args: args{
				hook: &ProviderHooks{},
			},
			want: want{
				err: errors.New(errNotProvider),
			},
		},
		"ErrNotProviderRevision": {
			reason: "Should return error if the supplied package revision is not a provider revision.",
			args: args{
				hook: &ProviderHooks{},
				pkg:  &pkgmetav1.Provider{},
			},
			want: want{
				err: errors.New(errNotProviderRevision),
			},
		},
		"ProviderActive": {
			reason: "Should only update status if provider revision is active.",
			args: args{
				hook: &ProviderHooks{},
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
		},
		"Configuration": {
			reason: "Should always update status for configuration revisions.",
			args: args{
				hook: &ConfigurationHooks{},
				pkg: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
		},
		"ErrProviderDeleteDeployment": {
			reason: "Should return error if we fail to delete deployment for inactive provider revision.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								switch o.(type) {
								case *appsv1.Deployment:
									return errBoom
								case *corev1.ServiceAccount:
									return nil
								}
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
				err: errors.Wrap(errBoom, errDeleteProviderDeployment),
			},
		},
		"ErrProviderDeleteSA": {
			reason: "Should return error if we fail to delete service account for inactive provider revision.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								switch o.(type) {
								case *appsv1.Deployment:
									return nil
								case *corev1.ServiceAccount:
									return errBoom
								}
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
				err: errors.Wrap(errBoom, errDeleteProviderSA),
			},
		},
		"SuccessfulProviderDelete": {
			reason: "Should update status and not return error when deployment and service account deleted successfully.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
			},
		},
		"ErrNotFunction": {
			reason: "Should return error if not function.",
			args: args{
				hook: &FunctionHooks{},
			},
			want: want{
				err: errors.New(errNotFunction),
			},
		},
		"FunctionActive": {
			reason: "Should only update status if function revision is active.",
			args: args{
				hook: &FunctionHooks{},
				pkg: &pkgmetav1alpha1.Function{
					Spec: pkgmetav1alpha1.FunctionSpec{
						MetaSpec: pkgmetav1alpha1.MetaSpec{
							Crossplane: &pkgmetav1alpha1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1alpha1.Dependency{{
								Function: &functionDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
		},
		"ErrFunctionDeleteDeployment": {
			reason: "Should return error if we fail to delete deployment for inactive function revision.",
			args: args{
				hook: &FunctionHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								switch o.(type) {
								case *appsv1.Deployment:
									return errBoom
								case *corev1.ServiceAccount:
									return nil
								}
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{
					Spec: pkgmetav1alpha1.FunctionSpec{
						MetaSpec: pkgmetav1alpha1.MetaSpec{
							Crossplane: &pkgmetav1alpha1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1alpha1.Dependency{{
								Function: &functionDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
				err: errors.Wrap(errBoom, errDeleteFunctionDeployment),
			},
		},
		"ErrFunctionDeleteSA": {
			reason: "Should return error if we fail to delete service account for inactive function revision.",
			args: args{
				hook: &FunctionHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								switch o.(type) {
								case *appsv1.Deployment:
									return nil
								case *corev1.ServiceAccount:
									return errBoom
								}
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{
					Spec: pkgmetav1alpha1.FunctionSpec{
						MetaSpec: pkgmetav1alpha1.MetaSpec{
							Crossplane: &pkgmetav1alpha1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1alpha1.Dependency{{
								Function: &functionDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
				err: errors.Wrap(errBoom, errDeleteFunctionSA),
			},
		},
		"SuccessfulFunctionDelete": {
			reason: "Should update status and not return error when deployment and service account deleted successfully.",
			args: args{
				hook: &FunctionHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{
					Spec: pkgmetav1alpha1.FunctionSpec{
						MetaSpec: pkgmetav1alpha1.MetaSpec{
							Crossplane: &pkgmetav1alpha1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1alpha1.Dependency{{
								Provider: &functionDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionInactive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.hook.Pre(context.TODO(), tc.args.pkg, tc.args.rev)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestHookPost(t *testing.T) {
	errBoom := errors.New("boom")
	saName := "crossplane"
	namespace := "crossplane-system"

	type args struct {
		hook Hooks
		pkg  runtime.Object
		rev  v1.PackageRevision
	}

	type want struct {
		err error
		rev v1.PackageRevision
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrNotProvider": {
			reason: "Should return error if not provider.",
			args: args{
				hook: &ProviderHooks{},
			},
			want: want{
				err: errors.New(errNotProvider),
			},
		},
		"ProviderInactive": {
			reason: "Should do nothing if provider revision is inactive.",
			args: args{
				hook: &ProviderHooks{},
				pkg:  &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
		},
		"ErrGetSA": {
			reason: "Should return error if we fail to get core Crossplane ServiceAccount.",
			args: args{
				hook: &ProviderHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							switch o.(type) {
							case *appsv1.Deployment:
								return nil
							case *corev1.ServiceAccount:
								return errBoom
							}
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									return errBoom
								default:
									return nil
								}
							},
						},
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
				err: errors.Wrap(errBoom, errGetServiceAccount),
			},
		},
		"ErrProviderApplySA": {
			reason: "Should return error if we fail to apply service account for active providerrevision.",
			args: args{
				hook: &ProviderHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							switch o.(type) {
							case *appsv1.Deployment:
								return nil
							case *corev1.Secret:
								return nil
							case *corev1.ServiceAccount:
								return errBoom
							}
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch o := obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									*o = corev1.ServiceAccount{
										ImagePullSecrets: []corev1.LocalObjectReference{{}},
									}
									return nil
								default:
									return errBoom
								}
							},
						},
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
				err: errors.Wrap(errBoom, errApplyProviderSA),
			},
		},
		"ErrProviderGetControllerConfigDeployment": {
			reason: "Should return error if we fail to get controller config for active provider revision.",
			args: args{
				hook: &ProviderHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch obj.(type) {
								case *v1alpha1.ControllerConfig:
									if key.Name != "custom-config" {
										t.Errorf("unexpected Controller Config name: %s", key.Name)
									}
									return errBoom
								default:
									return nil
								}
							},
						},
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						ControllerConfigReference: &v1.ControllerConfigReference{
							Name: "custom-config",
						},
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
						ControllerConfigReference: &v1.ControllerConfigReference{
							Name: "custom-config",
						},
					},
				},
				err: errors.Wrap(errBoom, errGetControllerConfig),
			},
		},
		"ErrProviderApplyDeployment": {
			reason: "Should return error if we fail to apply deployment for active provider revision.",
			args: args{
				hook: &ProviderHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							switch o.(type) {
							case *appsv1.Deployment:
								return errBoom
							case *corev1.ServiceAccount:
								return nil
							}
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch o := obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									*o = corev1.ServiceAccount{
										ImagePullSecrets: []corev1.LocalObjectReference{{}},
									}
									return nil
								case *corev1.Secret:
									if key.Name != initializer.RootCACertSecretName && key.Name != tlsServerSecret && key.Name != tlsClientSecret {
										t.Errorf("unexpected Secret name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									s := &corev1.Secret{
										Data: map[string][]byte{
											corev1.TLSCertKey:       []byte(caCert),
											corev1.TLSPrivateKeyKey: []byte(caKey),
										},
									}
									s.DeepCopyInto(obj.(*corev1.Secret))
									return nil
								default:
									return errBoom
								}
							},
						},
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
				err: errors.Wrap(errBoom, errApplyProviderDeployment),
			},
		},
		"ErrProviderUnavailableDeployment": {
			reason: "Should return error if deployment is unavailable for provider revision.",
			args: args{
				hook: &ProviderHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							d, ok := o.(*appsv1.Deployment)
							if !ok {
								return nil
							}
							d.Status.Conditions = []appsv1.DeploymentCondition{{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionFalse,
								Message: errBoom.Error(),
							}}
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch o := obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									*o = corev1.ServiceAccount{
										ImagePullSecrets: []corev1.LocalObjectReference{{}},
									}
									return nil
								case *corev1.Secret:
									if key.Name != caSecret && key.Name != tlsServerSecret && key.Name != tlsClientSecret {
										t.Errorf("unexpected Secret name: %s", key.Name)
									}
									if key.Namespace != tlsSecretNamespace {
										t.Errorf("unexpected Secret Namespace: %s", key.Namespace)
									}
									*o = corev1.Secret{
										Data: map[string][]byte{
											corev1.TLSCertKey:       []byte(caCert),
											corev1.TLSPrivateKeyKey: []byte(caKey),
										},
									}
									return nil
								default:
									return errBoom
								}
							},
						},
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
				err: errors.Errorf("%s: %s", errUnavailableProviderDeployment, errBoom.Error()),
			},
		},
		"SuccessfulProviderApply": {
			reason: "Should not return error if successfully applied service account and deployment for active provider revision.",
			args: args{
				hook: &ProviderHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch o := obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									*o = corev1.ServiceAccount{
										ImagePullSecrets: []corev1.LocalObjectReference{{}},
									}
									return nil
								case *corev1.Secret:
									if key.Name != caSecret && key.Name != tlsServerSecret && key.Name != tlsClientSecret {
										t.Errorf("unexpected Secret name: %s", key.Name)
									}
									if key.Namespace != tlsSecretNamespace {
										t.Errorf("unexpected Secret Namespace: %s", key.Namespace)
									}
									*o = corev1.Secret{
										Data: map[string][]byte{
											corev1.TLSCertKey:       []byte(caCert),
											corev1.TLSPrivateKeyKey: []byte(caKey),
										},
									}
									return nil
								default:
									return errBoom
								}
							},
						},
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
						TLSClientSecretName: &tlsClientSecret,
					},
				},
			},
		},
		"ErrNotFunction": {
			reason: "Should return error if not function.",
			args: args{
				hook: &FunctionHooks{},
			},
			want: want{
				err: errors.New(errNotFunction),
			},
		},
		"FunctionInactive": {
			reason: "Should do nothing if function revision is inactive.",
			args: args{
				hook: &FunctionHooks{},
				pkg:  &pkgmetav1alpha1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
		},
		"ErrGetFunctionSA": {
			reason: "Should return error if we fail to get core Crossplane ServiceAccount.",
			args: args{
				hook: &FunctionHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							switch o.(type) {
							case *appsv1.Deployment:
								return nil
							case *corev1.ServiceAccount:
								return errBoom
							}
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									return errBoom
								default:
									return nil
								}
							},
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
				err: errors.Wrap(errBoom, errGetServiceAccount),
			},
		},
		"ErrFunctionApplySA": {
			reason: "Should return error if we fail to apply service account for active function revision.",
			args: args{
				hook: &FunctionHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							switch o.(type) {
							case *appsv1.Deployment:
								return nil
							case *corev1.Secret:
								return nil
							case *corev1.ServiceAccount:
								return errBoom
							}
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch o := obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									*o = corev1.ServiceAccount{
										ImagePullSecrets: []corev1.LocalObjectReference{{}},
									}
									return nil
								default:
									return errBoom
								}
							},
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
				err: errors.Wrap(errBoom, errApplyFunctionSA),
			},
		},
		"ErrFunctionGetControllerConfigDeployment": {
			reason: "Should return error if we fail to get controller config for active function revision.",
			args: args{
				hook: &FunctionHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch obj.(type) {
								case *v1alpha1.ControllerConfig:
									if key.Name != "custom-config" {
										t.Errorf("unexpected Controller Config name: %s", key.Name)
									}
									return errBoom
								default:
									return nil
								}
							},
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						ControllerConfigReference: &v1.ControllerConfigReference{
							Name: "custom-config",
						},
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
						ControllerConfigReference: &v1.ControllerConfigReference{
							Name: "custom-config",
						},
					},
				},
				err: errors.Wrap(errBoom, errGetControllerConfig),
			},
		},
		"ErrFunctionApplyDeployment": {
			reason: "Should return error if we fail to apply deployment for active function revision.",
			args: args{
				hook: &FunctionHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							switch o.(type) {
							case *appsv1.Deployment:
								return errBoom
							case *corev1.ServiceAccount:
								return nil
							}
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch o := obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									*o = corev1.ServiceAccount{
										ImagePullSecrets: []corev1.LocalObjectReference{{}},
									}
									return nil
								case *corev1.Secret:
									if key.Name != initializer.RootCACertSecretName && key.Name != tlsServerSecret && key.Name != tlsClientSecret {
										t.Errorf("unexpected Secret name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									s := &corev1.Secret{
										Data: map[string][]byte{
											corev1.TLSCertKey:       []byte(caCert),
											corev1.TLSPrivateKeyKey: []byte(caKey),
										},
									}
									s.DeepCopyInto(obj.(*corev1.Secret))
									return nil
								default:
									return errBoom
								}
							},
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
				err: errors.Wrap(errBoom, errApplyFunctionDeployment),
			},
		},
		"ErrFunctionUnavailableDeployment": {
			reason: "Should return error if deployment is unavailable for function revision.",
			args: args{
				hook: &FunctionHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							d, ok := o.(*appsv1.Deployment)
							if !ok {
								return nil
							}
							d.Status.Conditions = []appsv1.DeploymentCondition{{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionFalse,
								Message: errBoom.Error(),
							}}
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch o := obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									*o = corev1.ServiceAccount{
										ImagePullSecrets: []corev1.LocalObjectReference{{}},
									}
									return nil
								case *corev1.Secret:
									if key.Name != caSecret && key.Name != tlsServerSecret && key.Name != tlsClientSecret {
										t.Errorf("unexpected Secret name: %s", key.Name)
									}
									if key.Namespace != tlsSecretNamespace {
										t.Errorf("unexpected Secret Namespace: %s", key.Namespace)
									}
									*o = corev1.Secret{
										Data: map[string][]byte{
											corev1.TLSCertKey:       []byte(caCert),
											corev1.TLSPrivateKeyKey: []byte(caKey),
										},
									}
									return nil
								default:
									return errBoom
								}
							},
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{},
				rev: &v1beta1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "my-function",
						},
					},
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "my-function",
						},
					},
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
					},
					Status: v1beta1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{},
						Endpoint:              fmt.Sprintf(serviceEndpointFmt, "my-function", namespace, servicePort),
					},
				},
				err: errors.Errorf("%s: %s", errUnavailableFunctionDeployment, errBoom.Error()),
			},
		},
		"SuccessfulFunctionApply": {
			reason: "Should not return error if successfully applied service account and deployment for active function revision.",
			args: args{
				hook: &FunctionHooks{
					namespace:      namespace,
					serviceAccount: saName,
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
						Client: &test.MockClient{
							MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
								switch o := obj.(type) {
								case *corev1.ServiceAccount:
									if key.Name != saName {
										t.Errorf("unexpected ServiceAccount name: %s", key.Name)
									}
									if key.Namespace != namespace {
										t.Errorf("unexpected ServiceAccount Namespace: %s", key.Namespace)
									}
									*o = corev1.ServiceAccount{
										ImagePullSecrets: []corev1.LocalObjectReference{{}},
									}
									return nil
								case *corev1.Secret:
									if key.Name != caSecret && key.Name != tlsServerSecret && key.Name != tlsClientSecret {
										t.Errorf("unexpected Secret name: %s", key.Name)
									}
									if key.Namespace != tlsSecretNamespace {
										t.Errorf("unexpected Secret Namespace: %s", key.Namespace)
									}
									*o = corev1.Secret{
										Data: map[string][]byte{
											corev1.TLSCertKey:       []byte(caCert),
											corev1.TLSPrivateKeyKey: []byte(caKey),
										},
									}
									return nil
								default:
									return errBoom
								}
							},
						},
					},
				},
				pkg: &pkgmetav1alpha1.Function{},
				rev: &v1beta1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "my-function",
						},
					},
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.LabelParentPackage: "my-function",
						},
					},
					Spec: v1.PackageRevisionSpec{
						DesiredState:        v1.PackageRevisionActive,
						TLSServerSecretName: &tlsServerSecret,
					},
					Status: v1beta1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{},
						Endpoint:              fmt.Sprintf(serviceEndpointFmt, "my-function", namespace, servicePort),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.hook.Post(context.TODO(), tc.args.pkg, tc.args.rev)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Post(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Post(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
