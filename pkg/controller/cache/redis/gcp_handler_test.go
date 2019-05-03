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

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	gcpcachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/cache/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
)

const (
	claimVersion32 = "3.2"
	claimVersion40 = "4.0"

	gcpClassVersion32 = "REDIS_3_2"
	gcpClassVersion40 = "REDIS_4_0"
)

func TestResolveGCPClassInstanceValues(t *testing.T) {
	cases := []struct {
		name    string
		class   *gcpcachev1alpha1.CloudMemorystoreInstanceSpec
		claim   corev1alpha1.ResourceClaim
		want    *gcpcachev1alpha1.CloudMemorystoreInstanceSpec
		wantErr error
	}{
		{
			name:  "ClassUnsetClaimUnset",
			class: &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{},
			claim: &cachev1alpha1.RedisCluster{},
			want:  &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{},
		},
		{
			name:  "ClassSetClaimUnset",
			class: &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{RedisVersion: gcpClassVersion32},
			claim: &cachev1alpha1.RedisCluster{},
			want:  &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{RedisVersion: gcpClassVersion32},
		},
		{
			name:  "ClassUnsetClaimSet",
			class: &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{},
			claim: &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimVersion32}},
			want:  &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{RedisVersion: gcpClassVersion32},
		},
		{
			name:  "ClassSetClaimSetMatching",
			class: &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{RedisVersion: gcpClassVersion32},
			claim: &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimVersion32}},
			want:  &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{RedisVersion: gcpClassVersion32},
		},
		{
			name:    "ClassSetClaimSetConflict",
			class:   &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{RedisVersion: gcpClassVersion32},
			claim:   &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimVersion40}},
			want:    &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{},
			wantErr: errors.WithStack(errors.Errorf("cannot resolve class claim values: claim value [%s] does not match the one defined in the resource class [%s]", gcpClassVersion40, gcpClassVersion32)),
		},
		{
			name:    "NotARedisCache",
			class:   &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{},
			claim:   &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "8.0"}},
			want:    &gcpcachev1alpha1.CloudMemorystoreInstanceSpec{},
			wantErr: errors.Errorf("unexpected claim type: %s", reflect.TypeOf(&storagev1alpha1.MySQLInstance{})),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := resolveGCPClassInstanceValues(tc.class, tc.claim)
			if diff := cmp.Diff(tc.wantErr, gotErr); diff != "" {
				t.Errorf("want error != got error:\n %+v", diff)
			}

			if diff := cmp.Diff(tc.want, tc.class); diff != "" {
				t.Errorf("want != got:\n %+v", diff)
			}
		})
	}
}
