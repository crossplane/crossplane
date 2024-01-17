// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package claim

import (
	"context"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func FuzzPropagateConnection(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		f := fuzz.NewConsumer(data)
		cp := &fake.Composite{}
		cm := &fake.CompositeClaim{}
		err := f.GenerateStruct(cp)
		if err != nil {
			return
		}

		err = f.GenerateStruct(cm)
		if err != nil {
			return
		}

		mgcsdata := make(map[string][]byte)
		err = f.FuzzMap(&mgcsdata)
		if err != nil {
			return
		}

		c := resource.ClientApplicator{
			Client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					// The managed secret has some data when we get it.
					s := resource.ConnectionSecretFor(cp, schema.GroupVersionKind{})
					s.Data = mgcsdata

					*o.(*corev1.Secret) = *s
					return nil
				}),
			},
			Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
				return nil
			}),
		}
		api := &APIConnectionPropagator{client: c}
		_, _ = api.PropagateConnection(context.Background(), cm, cp)
	})
}
