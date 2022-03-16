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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestWebhookConfigurations(t *testing.T) {
	type args struct {
		kube client.Client
		svc  admv1.ServiceReference
		opts []WebhookConfigurationsOption
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
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Success": {
			reason: "If a proper webhook TLS is given, then webhook configurations should have the configs injected and operations should succeed",
			args: args{
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
		"CertNotFound": {
			reason: "If TLS Secret cannot be found, then it should not proceed",
			args: args{
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
		"CertKeyEmpty": {
			reason: "If the TLS Secret does not have a CA bundle, then it should not proceed",
			args: args{
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
		"ApplyFailed": {
			reason: "If it cannot apply webhook configurations, then it should not proceed",
			args: args{
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
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := NewWebhookConfigurations(
				"/webhooks",
				sch,
				types.NamespacedName{},
				tc.args.svc,
				tc.opts...).Run(context.TODO(), tc.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
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
