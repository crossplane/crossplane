/*
Copyright 2022 The Crossplane Authors.

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

package initializer

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestSecretCAProvider(t *testing.T) {
	type args struct {
		kube client.Client
		ref  types.NamespacedName
	}

	type want struct {
		ca  []byte
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "It should return the CA bundle from the secret.",
			args: args{
				ref: types.NamespacedName{Name: "my-secret", Namespace: "ns"},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						s := &corev1.Secret{
							Data: map[string][]byte{"tls.crt": []byte("CABUNDLE")},
						}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
				},
			},
			want: want{
				ca: []byte("CABUNDLE"),
			},
		},
		"GetError": {
			reason: "It should return an error if the secret cannot be retrieved.",
			args: args{
				ref:  types.NamespacedName{Name: "my-secret", Namespace: "ns"},
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetWebhookSecret),
			},
		},
		"EmptyCert": {
			reason: "It should return an error if the secret has no tls.crt data.",
			args: args{
				ref:  types.NamespacedName{Name: "my-secret", Namespace: "ns"},
				kube: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			},
			want: want{
				err: errors.Errorf(errFmtNoTLSCrtInSecret, "ns/my-secret"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := &SecretCAProvider{SecretRef: tc.args.ref}
			ca, err := p.GetCABundle(context.TODO(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetCABundle(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ca, ca); diff != "" {
				t.Errorf("\n%s\nGetCABundle(...): -want ca, +got ca:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFileCAProvider(t *testing.T) {
	type want struct {
		ca  []byte
		err error
	}

	validDir := t.TempDir()
	validPath := validDir + "/ca.crt"
	_ = os.WriteFile(validPath, []byte("CABUNDLE"), 0o644)

	emptyDir := t.TempDir()
	emptyPath := emptyDir + "/ca.crt"
	_ = os.WriteFile(emptyPath, []byte{}, 0o644)

	cases := map[string]struct {
		reason string
		path   string
		want   want
	}{
		"Success": {
			reason: "It should return the CA bundle from the file.",
			path:   validPath,
			want: want{
				ca: []byte("CABUNDLE"),
			},
		},
		"FileNotFound": {
			reason: "It should return an error if the file does not exist.",
			path:   "/nonexistent/ca.crt",
			want: want{
				err: errors.Wrap(errors.New("open /nonexistent/ca.crt: no such file or directory"), errReadCABundleFile),
			},
		},
		"EmptyFile": {
			reason: "It should return an error if the file is empty.",
			path:   emptyPath,
			want: want{
				err: errors.New(errEmptyCABundleFile),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := &FileCAProvider{Path: tc.path}
			ca, err := p.GetCABundle(context.TODO(), nil)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetCABundle(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ca, ca); diff != "" {
				t.Errorf("\n%s\nGetCABundle(...): -want ca, +got ca:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWebhookConfigurations(t *testing.T) {
	type args struct {
		kube       client.Client
		caProvider WebhookCAProvider
		svc        admv1.ServiceReference
		opts       []WebhookConfigurationsOption
	}

	type want struct {
		err error
	}

	fs := afero.NewMemMapFs()
	f, _ := fs.Create("/webhooks/manifests.yaml")
	_, _ = f.WriteString(webhookConfigs)

	fsWithMixedTypes := afero.NewMemMapFs()
	f, _ = fsWithMixedTypes.Create("/webhooks/manifests.yaml")
	_, _ = f.WriteString(webhookCRD)

	secret := &corev1.Secret{
		Data: map[string][]byte{"tls.crt": []byte("CABUNDLE")},
	}
	sch := runtime.NewScheme()
	_ = admv1.AddToScheme(sch)
	_ = extv1.AddToScheme(sch)

	var p int32 = 1234

	svc := admv1.ServiceReference{
		Name:      "a",
		Namespace: "b",
		Port:      &p,
	}

	caDir := t.TempDir()
	caFilePath := caDir + "/ca.crt"
	_ = os.WriteFile(caFilePath, []byte("CABUNDLE"), 0o644)

	emptyCADir := t.TempDir()
	emptyCAPath := emptyCADir + "/ca.crt"
	_ = os.WriteFile(emptyCAPath, []byte{}, 0o644)

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"SecretCAProviderSuccess": {
			reason: "If a proper webhook TLS secret is given, then webhook configurations should have the configs injected and operations should succeed",
			args: args{
				caProvider: &SecretCAProvider{SecretRef: types.NamespacedName{}},
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				svc: svc,
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							secret.DeepCopyInto(s)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
						switch c := obj.(type) {
						case *admv1.ValidatingWebhookConfiguration:
							for _, w := range c.Webhooks {
								if !bytes.Equal(w.ClientConfig.CABundle, []byte("CABUNDLE")) {
									t.Errorf("unexpected certificate bundle content: %sch", string(w.ClientConfig.CABundle))
								}
							}
						case *admv1.MutatingWebhookConfiguration:
							for _, w := range c.Webhooks {
								if !bytes.Equal(w.ClientConfig.CABundle, []byte("CABUNDLE")) {
									t.Errorf("unexpected certificate bundle content: %sch", string(w.ClientConfig.CABundle))
								}
							}
						default:
							t.Error("unexpected type")
						}
						return nil
					},
				},
			},
		},
		"SecretCAProviderCertNotFound": {
			reason: "If TLS Secret cannot be found, then it should not proceed",
			args: args{
				caProvider: &SecretCAProvider{SecretRef: types.NamespacedName{}},
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetWebhookSecret),
			},
		},
		"SecretCAProviderCertKeyEmpty": {
			reason: "If the TLS Secret does not have a CA bundle, then it should not proceed",
			args: args{
				caProvider: &SecretCAProvider{SecretRef: types.NamespacedName{}},
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				},
			},
			want: want{
				err: errors.Errorf(errFmtNoTLSCrtInSecret, "/"),
			},
		},
		"SecretCAProviderApplyFailed": {
			reason: "If it cannot apply webhook configurations, then it should not proceed",
			args: args{
				caProvider: &SecretCAProvider{SecretRef: types.NamespacedName{}},
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							secret.DeepCopyInto(s)
							return nil
						}
						return errBoom
					},
				},
				svc: svc,
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, "cannot get object"), errApplyWebhookConfiguration),
			},
		},
		"NonWebhookType": {
			reason: "Only webhook configuration types can be processed",
			args: args{
				caProvider: &SecretCAProvider{SecretRef: types.NamespacedName{}},
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fsWithMixedTypes),
				},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							secret.DeepCopyInto(s)
							return nil
						}
						return errBoom
					},
				},
				svc: svc,
			},
			want: want{
				err: errors.Errorf("only MutatingWebhookConfiguration and ValidatingWebhookConfiguration kinds are accepted, got %s", "*v1.CustomResourceDefinition"),
			},
		},
		"FileCAProviderSuccess": {
			reason: "If a valid CA file is given via FileCAProvider, webhook configurations should be injected successfully",
			args: args{
				caProvider: &FileCAProvider{Path: caFilePath},
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				svc: svc,
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
						switch c := obj.(type) {
						case *admv1.ValidatingWebhookConfiguration:
							for _, w := range c.Webhooks {
								if !bytes.Equal(w.ClientConfig.CABundle, []byte("CABUNDLE")) {
									t.Errorf("unexpected certificate bundle content: %s", string(w.ClientConfig.CABundle))
								}
							}
						case *admv1.MutatingWebhookConfiguration:
							for _, w := range c.Webhooks {
								if !bytes.Equal(w.ClientConfig.CABundle, []byte("CABUNDLE")) {
									t.Errorf("unexpected certificate bundle content: %s", string(w.ClientConfig.CABundle))
								}
							}
						default:
							t.Error("unexpected type")
						}
						return nil
					},
				},
			},
		},
		"FileCAProviderFileNotFound": {
			reason: "If the CA file does not exist, it should return an error",
			args: args{
				caProvider: &FileCAProvider{Path: "/nonexistent/ca.crt"},
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				kube: &test.MockClient{},
			},
			want: want{
				err: errors.Wrap(errors.New("open /nonexistent/ca.crt: no such file or directory"), errReadCABundleFile),
			},
		},
		"FileCAProviderEmptyFile": {
			reason: "If the CA file is empty, it should return an error",
			args: args{
				caProvider: &FileCAProvider{Path: emptyCAPath},
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				kube: &test.MockClient{},
			},
			want: want{
				err: errors.New(errEmptyCABundleFile),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := NewWebhookConfigurations(
				"/webhooks",
				sch,
				tc.args.caProvider,
				tc.args.svc,
				tc.opts...).Run(context.TODO(), tc.kube)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%sch\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

const (
	webhookConfigs = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  creationTimestamp: null
  name: validating-webhook-configuration
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    service:
      name: webhook-service
      namespace: system
      path: /validate
  failurePolicy: Fail
  name: compositeresourcedefinitions
  rules:
  - apiGroups:
    - apiextensions.crossplane.io
    apiVersions:
    - v1
    operations:
    - CREATE
    - UPDATE
    - DELETE
    resources:
    - compositeresourcedefinitions
  sideEffects: None
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  creationTimestamp: null
  name: validating-webhook-configuration
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    service:
      name: webhook-service
      namespace: system
      path: /validate
  failurePolicy: Fail
  name: compositeresourcedefinitions
  rules:
  - apiGroups:
    - apiextensions.crossplane.io
    apiVersions:
    - v1
    operations:
    - CREATE
    - UPDATE
    - DELETE
    resources:
    - compositeresourcedefinitions
  sideEffects: None
`
)

func TestRemoveValidatingWebhooks(t *testing.T) {
	type params struct {
		configName   string
		webhookNames []string
	}

	type args struct {
		ctx  context.Context
		kube client.Client
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"NotFound": {
			reason: "If the config isn't found we should return early.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
				},
			},
		},
		"NoOp": {
			reason: "If the config doesn't have any entries to delete we should return early.",
			params: params{
				configName:   "some-config",
				webhookNames: []string{"delete-me"},
			},
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						obj.(*admv1.ValidatingWebhookConfiguration).Webhooks = []admv1.ValidatingWebhook{
							{Name: "dont-delete-me"},
						}
						return nil
					},
				},
			},
		},
		"DeleteError": {
			reason: "We should return any error we encounter deleting the config.",
			params: params{
				configName:   "some-config",
				webhookNames: []string{"delete-me"},
			},
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						obj.(*admv1.ValidatingWebhookConfiguration).Webhooks = []admv1.ValidatingWebhook{
							{Name: "delete-me"},
						}
						return nil
					},
					MockDelete: func(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
						return errBoom
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"UpdateError": {
			reason: "We should return any error we encounter updating the config.",
			params: params{
				configName:   "some-config",
				webhookNames: []string{"delete-me"},
			},
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						obj.(*admv1.ValidatingWebhookConfiguration).Webhooks = []admv1.ValidatingWebhook{
							{Name: "delete-me"},
							{Name: "dont-delete-me"},
						}
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return errBoom
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"Success": {
			reason: "We should remove only the named webhooks, and maintain the order of the rest.",
			params: params{
				configName:   "some-config",
				webhookNames: []string{"delete-me"},
			},
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						obj.(*admv1.ValidatingWebhookConfiguration).Webhooks = []admv1.ValidatingWebhook{
							{Name: "delete-me"},
							{Name: "dont-delete-me"},
							{Name: "also-dont-delete-me"},
						}
						return nil
					},
					MockUpdate: func(_ context.Context, got client.Object, _ ...client.UpdateOption) error {
						want := &admv1.ValidatingWebhookConfiguration{
							Webhooks: []admv1.ValidatingWebhook{
								{Name: "dont-delete-me"},
								{Name: "also-dont-delete-me"},
							},
						}

						if diff := cmp.Diff(want, got); diff != "" {
							t.Error(diff)
						}

						return nil
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := NewValidatingWebhookRemover(tc.params.configName, tc.params.webhookNames...).Run(tc.args.ctx, tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
