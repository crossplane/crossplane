/*
Copyright 2023 The Crossplane Authors.

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
