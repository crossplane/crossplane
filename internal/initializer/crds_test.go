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
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	fs := afero.NewMemMapFs()
	f, _ := fs.Create("/crds/nonwebhookcrd.yaml")
	_, _ = f.WriteString(nonWebhookCRD)
	f, _ = fs.Create("/crds/webhookcrd.yaml")
	_, _ = f.WriteString(webhookCRD)
	f, _ = fs.Create("/webhook/tls/tls.crt")
	_, _ = f.WriteString("CABUNDLE")
	s := runtime.NewScheme()
	_ = extv1.AddToScheme(s)
	cases := map[string]struct {
		args
		want
	}{
		"SuccessWithoutTLSSecret": {
			args: args{
				opts: []CoreCRDsOption{
					WithFs(fs),
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
		"SuccessWithTLSSecret": {
			args: args{
				opts: []CoreCRDsOption{
					WithFs(fs),
					WithWebhookCertDir("/webhook/tls"),
				},
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
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
			args: args{
				opts: []CoreCRDsOption{
					WithFs(fs),
					WithWebhookCertDir("/olala"),
				},
			},
			want: want{
				err: errors.Wrapf(&os.PathError{Op: "open", Path: "/olala/tls.crt", Err: errors.Errorf("file does not exist")}, errReadTLSCertFmt, "/olala"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := NewCoreCRDs("/crds", s, tc.opts...).Run(context.TODO(), tc.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
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
