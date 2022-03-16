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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
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

func TestCoreCRDs(t *testing.T) {
	type args struct {
		kube client.Client
		opts []CoreCRDsOption
	}
	type want struct {
		err error
	}
	fsWithoutConversionCRD := afero.NewMemMapFs()
	f, _ := fsWithoutConversionCRD.Create("/crds/nonwebhookcrd.yaml")
	_, _ = f.WriteString(nonWebhookCRD)

	fsWithConversionCRD := afero.NewMemMapFs()
	f, _ = fsWithConversionCRD.Create("/crds/webhookcrd.yaml")
	_, _ = f.WriteString(webhookCRD)

	fsMixedCRDs := afero.NewMemMapFs()
	f, _ = fsMixedCRDs.Create("/crds/webhookcrd.yaml")
	_, _ = f.WriteString(webhookCRD)
	f, _ = fsMixedCRDs.Create("/crds/nonwebhookcrd.yaml")
	_, _ = f.WriteString(nonWebhookCRD)

	secret := &corev1.Secret{
		Data: map[string][]byte{"tls.crt": []byte("CABUNDLE")},
	}
	s := runtime.NewScheme()
	_ = extv1.AddToScheme(s)
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"SuccessWithoutTLSSecret": {
			reason: "If no webhook TLS secret is given, then all operations should succeed if CRDs do not have webhook strategy",
			args: args{
				opts: []CoreCRDsOption{
					WithFs(fsWithoutConversionCRD),
				},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
						return nil
					},
				},
			},
		},
		"CRDWithConversionWithoutTLSSecret": {
			reason: "CRDs with webhook conversion requires enabling webhooks, otherwise all apiserver requests will fail.",
			args: args{
				opts: []CoreCRDsOption{
					WithFs(fsWithConversionCRD),
				},
			},
			want: want{
				err: errors.Errorf(errFmtCRDWithConversionWithoutTLS, "crontabsconverts.stable.example.com"),
			},
		},
		"SuccessWithTLSSecret": {
			reason: "If TLS Secret is given and populated correctly, then CA bundle should be injected and apply operations should succeed",
			args: args{
				opts: []CoreCRDsOption{
					WithFs(fsMixedCRDs),
					WithWebhookTLSSecretRef(types.NamespacedName{}),
				},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if s, ok := obj.(*corev1.Secret); ok {
							secret.DeepCopyInto(s)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
						crd := obj.(*extv1.CustomResourceDefinition)
						switch crd.Name {
						case "crontabs.stable.example.com":
							if crd.Spec.Conversion != nil {
								t.Error("\nCA is injected into a non-webhook CRD")
							}
							return nil
						case "crontabsconverts.stable.example.com":
							if diff := cmp.Diff(crd.Spec.Conversion.Webhook.ClientConfig.CABundle, []byte("CABUNDLE")); diff != "" {
								t.Errorf("\n%s", diff)
							}
							return nil
						}
						t.Error("unexpected crd")
						return nil
					},
				},
			},
		},
		"TLSSecretGivenButNotFound": {
			reason: "If TLS secret name is given, then it has to be found",
			args: args{
				opts: []CoreCRDsOption{
					WithWebhookTLSSecretRef(types.NamespacedName{}),
				},
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetWebhookSecret),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := NewCoreCRDs("/crds", s, tc.opts...).Run(context.TODO(), tc.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

const (
	nonWebhookCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: crontabs.stable.example.com
spec:
  group: stable.example.com
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                cronSpec:
                  type: string
  scope: Namespaced
  names:
    plural: crontabs
    singular: crontab
    kind: CronTab
    shortNames:
    - ct
`
	webhookCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: crontabsconverts.stable.example.com
spec:
  preserveUnknownFields: false
  group: stable.example.com
  versions:
  - name: v1beta1
    served: true
    storage: false
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              hostPort:
                type: string
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              host:
                type: string
              port:
                type: string
  conversion:
    strategy: Webhook
    webhookClientConfig:
      service:
        namespace: crd-conversion-webhook
        name: crd-conversion-webhook
        path: /crdconvert
  scope: Namespaced
  names:
    plural: crontabsconverts
    singular: crontabsconvert
    kind: CronTabsConvert
    shortNames:
    - ctc
`
)
