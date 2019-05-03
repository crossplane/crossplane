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

	"github.com/crossplaneio/crossplane/pkg/apis/aws/cache/v1alpha1"
	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
)

const claimversion99 = "9.9"

var (
	awsClassVersion32 = string(v1alpha1.LatestSupportedPatchVersion[claimVersion32])
)

func TestResolveAWSClassValues(t *testing.T) {
	cases := []struct {
		name    string
		class   *v1alpha1.ReplicationGroupSpec
		claim   corev1alpha1.ResourceClaim
		want    *v1alpha1.ReplicationGroupSpec
		wantErr error
	}{
		{
			name:    "ClassUnsetClaimUnset",
			class:   &v1alpha1.ReplicationGroupSpec{},
			claim:   &cachev1alpha1.RedisCluster{},
			want:    &v1alpha1.ReplicationGroupSpec{},
			wantErr: nil,
		},
		{
			name:    "ClassSetClaimUnset",
			class:   &v1alpha1.ReplicationGroupSpec{EngineVersion: awsClassVersion32},
			claim:   &cachev1alpha1.RedisCluster{},
			want:    &v1alpha1.ReplicationGroupSpec{EngineVersion: awsClassVersion32},
			wantErr: nil,
		},
		{
			name:    "ClassUnsetClaimSet",
			class:   &v1alpha1.ReplicationGroupSpec{},
			claim:   &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimVersion32}},
			want:    &v1alpha1.ReplicationGroupSpec{EngineVersion: awsClassVersion32},
			wantErr: nil,
		},
		{
			name:    "ClassUnsetClaimSetUnsupported",
			class:   &v1alpha1.ReplicationGroupSpec{},
			claim:   &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimversion99}},
			want:    &v1alpha1.ReplicationGroupSpec{},
			wantErr: errors.WithStack(errors.Errorf("cannot resolve class claim values: minor version %s is not currently supported", claimversion99)),
		},
		{
			name:    "ClassSetClaimSetMatching",
			class:   &v1alpha1.ReplicationGroupSpec{EngineVersion: awsClassVersion32},
			claim:   &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimVersion32}},
			want:    &v1alpha1.ReplicationGroupSpec{EngineVersion: awsClassVersion32},
			wantErr: nil,
		},
		{
			name:    "ClassSetClaimSetConflict",
			class:   &v1alpha1.ReplicationGroupSpec{EngineVersion: awsClassVersion32},
			claim:   &cachev1alpha1.RedisCluster{Spec: cachev1alpha1.RedisClusterSpec{EngineVersion: claimVersion40}},
			want:    &v1alpha1.ReplicationGroupSpec{EngineVersion: awsClassVersion32},
			wantErr: errors.WithStack(errors.Errorf("cannot resolve class claim values: class version %s is not a patch of claim version %s", awsClassVersion32, claimVersion40)),
		},
		{
			name:    "NotARedisCache",
			class:   &v1alpha1.ReplicationGroupSpec{},
			claim:   &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "8.0"}},
			want:    &v1alpha1.ReplicationGroupSpec{},
			wantErr: errors.Errorf("unexpected claim type: %s", reflect.TypeOf(&storagev1alpha1.MySQLInstance{})),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := resolveAWSClassInstanceValues(tc.class, tc.claim)
			if diff := cmp.Diff(tc.wantErr, gotErr); diff != "" {
				t.Errorf("want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, tc.class); diff != "" {
				t.Errorf("want != got:\n%s", diff)
			}
		})
	}
}
