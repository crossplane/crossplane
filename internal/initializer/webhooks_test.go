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

	admv1 "k8s.io/api/admissionregistration/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestWebhookConfigurations(t *testing.T) {
	type args struct {
		kube client.Client
		path string
		opts []WebhookConfigurationsOption
	}
	type want struct {
		err error
	}
	fs := afero.NewMemMapFs()
	f, _ := fs.Create("/webhooks/manifests.yaml")
	_, _ = f.WriteString(validatingWebhookConfig)
	f, _ = fs.Create("/webhook/tls/tls.crt")
	_, _ = f.WriteString("CABUNDLE")
	s := runtime.NewScheme()
	_ = admv1.AddToScheme(s)
	cases := map[string]struct {
		args
		want
	}{
		"Success": {
			args: args{
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				path: "/webhook/tls/tls.crt",
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
						vc, ok := obj.(*admv1.ValidatingWebhookConfiguration)
						if !ok {
							t.Error("unexpected type")
						}
						for _, w := range vc.Webhooks {
							if !bytes.Equal(w.ClientConfig.CABundle, []byte("CABUNDLE")) {
								t.Errorf("unexpected ca bundle content: %s", string(w.ClientConfig.CABundle))
							}
						}
						return nil
					},
				},
			},
		},
		"CertNotFound": {
			args: args{
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				path: "/some/path.crt",
			},
			want: want{
				err: errors.Wrapf(&os.PathError{Op: "open", Path: "/some/path.crt", Err: errors.Errorf("file does not exist")}, errReadTLSCertFmt, "/some/path.crt"),
			},
		},
		"ApplyFailed": {
			args: args{
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				path: "/webhook/tls/tls.crt",
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, "cannot get object"), errApplyWebhookConfiguration),
			},
		},
		"UnknownType": {
			args: args{
				opts: []WebhookConfigurationsOption{
					WithWebhookConfigurationsFs(fs),
				},
				path: "/webhook/tls/tls.crt",
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, "cannot get object"), errApplyWebhookConfiguration),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := NewWebhookConfigurations("/webhooks", s, tc.args.path, tc.opts...).Run(context.TODO(), tc.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}

const (
	validatingWebhookConfig = `
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
`
)
