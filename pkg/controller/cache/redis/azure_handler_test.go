/*
Copyright 2018 The Crossplane Authors.

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

package redis

import (
	"reflect"
	"testing"

	"github.com/go-test/deep"
	"github.com/pkg/errors"

	azurecachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/cache/v1alpha1"
	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
)

func TestResolveAzureClassValues(t *testing.T) {
	cases := []struct {
		name  string
		claim corev1alpha1.ResourceClaim
		want  error
	}{
		{
			name:  "EngineVersionUnset",
			claim: &cachev1alpha1.RedisCluster{},
		},
		{
			name:  "EngineVersionValid",
			claim: &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimVersion32}},
		},
		{
			name:  "EngineVersionInvalid",
			claim: &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimVersion40}},
			want:  errors.Errorf("Azure supports only Redis version %s", azurecachev1alpha1.SupportedRedisVersion),
		},
		{
			name:  "NotARedisCache",
			claim: &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "8.0"}},
			want:  errors.Errorf("unexpected claim type: %s", reflect.TypeOf(&storagev1alpha1.MySQLInstance{})),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveAzureClassValues(tc.claim)
			if diff := deep.Equal(tc.want, got); diff != nil {
				t.Errorf("want != got:\n%s", diff)
			}
		})
	}
}
