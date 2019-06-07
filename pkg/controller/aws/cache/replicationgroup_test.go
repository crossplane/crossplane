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

package cache

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/aws/cache/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	elasticacheclient "github.com/crossplaneio/crossplane/pkg/clients/aws/elasticache"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/elasticache/fake"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "coolNamespace"
	name      = "coolGroup"
	uid       = types.UID("definitely-a-uuid")
	id        = elasticacheclient.NamePrefix + "-efdd8494195d7940" // FNV-64a hash of uid

	cacheNodeType            = "n1.super.cool"
	authToken                = "coolToken"
	autoFailoverEnabled      = true
	cacheParameterGroupName  = "coolParamGroup"
	engineVersion            = "5.0.0"
	port                     = 6379
	host                     = "172.16.0.1"
	maintenanceWindow        = "tomorrow"
	snapshotRetentionLimit   = 1
	snapshotWindow           = "thedayaftertomorrow"
	transitEncryptionEnabled = true

	cacheClusterID = id + "-0001"

	providerName       = "cool-aws"
	providerSecretName = "cool-aws-secret"
	providerSecretKey  = "credentials"
	providerSecretData = "definitelyini"

	connectionSecretName = "cool-connection-secret"
)

var (
	ctx       = context.Background()
	errorBoom = errors.New("boom")

	objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid, Finalizers: []string{}}

	provider = awsv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerName},
		Spec: awsv1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: providerSecretName},
				Key:                  providerSecretKey,
			},
		},
	}

	providerSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerSecretName},
		Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	}
)

type replicationGroupModifier func(*v1alpha1.ReplicationGroup)

func withConditions(c ...corev1alpha1.DeprecatedCondition) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.DeprecatedConditionedStatus.Conditions = c }
}

func withState(s string) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.State = s }
}

func withFinalizers(f ...string) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.ObjectMeta.Finalizers = f }
}

func withReclaimPolicy(p corev1alpha1.ReclaimPolicy) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Spec.ReclaimPolicy = p }
}

func withGroupName(n string) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.GroupName = n }
}

func withEndpoint(e string) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.Endpoint = e }
}

func withPort(p int) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.Port = p }
}

func withDeletionTimestamp(t time.Time) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t} }
}

func withAuth() replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Spec.AuthEnabled = true }
}

func withMemberClusters(members []string) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.MemberClusters = members }
}

func replicationGroup(rm ...replicationGroupModifier) *v1alpha1.ReplicationGroup {
	r := &v1alpha1.ReplicationGroup{
		ObjectMeta: objectMeta,
		Spec: v1alpha1.ReplicationGroupSpec{
			AutomaticFailoverEnabled:   autoFailoverEnabled,
			CacheNodeType:              cacheNodeType,
			CacheParameterGroupName:    cacheParameterGroupName,
			EngineVersion:              engineVersion,
			PreferredMaintenanceWindow: maintenanceWindow,
			SnapshotRetentionLimit:     snapshotRetentionLimit,
			SnapshotWindow:             snapshotWindow,
			TransitEncryptionEnabled:   transitEncryptionEnabled,
			ProviderRef:                corev1.LocalObjectReference{Name: providerName},
			ConnectionSecretRef:        corev1.LocalObjectReference{Name: connectionSecretName},
		},
		Status: v1alpha1.ReplicationGroupStatus{
			ClusterEnabled: true,
		},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

func TestCreate(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		r           *v1alpha1.ReplicationGroup
		want        *v1alpha1.ReplicationGroup
		wantRequeue bool
	}{
		{
			name: "SuccessfulCreate",
			csd: &elastiCache{client: &fake.MockClient{
				MockCreateReplicationGroupRequest: func(_ *elasticache.CreateReplicationGroupInput) elasticache.CreateReplicationGroupRequest {
					return elasticache.CreateReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Data: &elasticache.CreateReplicationGroupOutput{}},
					}
				},
			}},
			r: replicationGroup(withAuth()),
			want: replicationGroup(
				withAuth(),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
				withFinalizers(finalizerName),
				withGroupName(id),
			),
			wantRequeue: true,
		},
		{
			name: "FailedCreate",
			csd: &elastiCache{client: &fake.MockClient{
				MockCreateReplicationGroupRequest: func(_ *elasticache.CreateReplicationGroupInput) elasticache.CreateReplicationGroupRequest {
					return elasticache.CreateReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(),
			want: replicationGroup(withConditions(
				corev1alpha1.DeprecatedCondition{
					Type:    corev1alpha1.DeprecatedFailed,
					Status:  corev1.ConditionTrue,
					Reason:  reasonCreatingResource,
					Message: errorBoom.Error(),
				},
			)),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue, _ := tc.csd.Create(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.Create(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := cmp.Diff(tc.want, tc.r); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSync(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		r           *v1alpha1.ReplicationGroup
		want        *v1alpha1.ReplicationGroup
		wantRequeue bool
	}{
		{
			name: "SuccessfulSyncWhileGroupCreating",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{Status: aws.String(v1alpha1.StatusCreating)}},
							},
						},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonCreatingResource,
						Message: errorBoom.Error(),
					},
				),
			),
			want: replicationGroup(
				withState(v1alpha1.StatusCreating),
				withGroupName(name),
				withConditions(
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionFalse,
						Reason:  reasonCreatingResource,
						Message: errorBoom.Error(),
					},
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue},
				),
			),
			wantRequeue: true,
		},
		{
			name: "SuccessfulSyncWhileGroupDeleting",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{Status: aws.String(v1alpha1.StatusDeleting)}},
							},
						},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			want: replicationGroup(
				withGroupName(name),
				withState(v1alpha1.StatusDeleting),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "SuccessfulSyncWhileGroupModifying",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{Status: aws.String(v1alpha1.StatusModifying)}},
							},
						},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue}),
			),
			want: replicationGroup(
				withState(v1alpha1.StatusModifying),
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionFalse}),
			),
			wantRequeue: true,
		},
		{
			name: "SuccessfulSyncWhileGroupAvailableAndDoesNotNeedUpdate",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{
									Status:                 aws.String(v1alpha1.StatusAvailable),
									MemberClusters:         []string{cacheClusterID},
									AutomaticFailover:      elasticache.AutomaticFailoverStatusEnabled,
									CacheNodeType:          aws.String(cacheNodeType),
									SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit),
									SnapshotWindow:         aws.String(snapshotWindow),
									ClusterEnabled:         aws.Bool(true),
									ConfigurationEndpoint:  &elasticache.Endpoint{Address: aws.String(host), Port: aws.Int64(port)},
								}},
							},
						},
					}
				},
				MockDescribeCacheClustersRequest: func(_ *elasticache.DescribeCacheClustersInput) elasticache.DescribeCacheClustersRequest {
					return elasticache.DescribeCacheClustersRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeCacheClustersOutput{
								CacheClusters: []elasticache.CacheCluster{{
									EngineVersion:              aws.String(engineVersion),
									PreferredMaintenanceWindow: aws.String(maintenanceWindow),
								}},
							},
						},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
			),
			want: replicationGroup(
				withState(v1alpha1.StatusAvailable),
				withGroupName(name),
				withConditions(
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionFalse},
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue},
				),
				withPort(port),
				withEndpoint(host),
				withMemberClusters([]string{cacheClusterID}),
			),
			wantRequeue: false,
		},
		{
			name: "SuccessfulSyncWhileGroupAvailableAndNeedsUpdate",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{
									Status:                 aws.String(v1alpha1.StatusAvailable),
									MemberClusters:         []string{cacheClusterID},
									AutomaticFailover:      elasticache.AutomaticFailoverStatusDisabled, // This field needs updating.
									CacheNodeType:          aws.String(cacheNodeType),
									SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit),
									SnapshotWindow:         aws.String(snapshotWindow),
									ClusterEnabled:         aws.Bool(true),
									ConfigurationEndpoint:  &elasticache.Endpoint{Address: aws.String(host), Port: aws.Int64(port)},
								}},
							},
						},
					}
				},
				MockDescribeCacheClustersRequest: func(_ *elasticache.DescribeCacheClustersInput) elasticache.DescribeCacheClustersRequest {
					return elasticache.DescribeCacheClustersRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeCacheClustersOutput{
								CacheClusters: []elasticache.CacheCluster{{
									EngineVersion:              aws.String(engineVersion),
									PreferredMaintenanceWindow: aws.String(maintenanceWindow),
								}},
							},
						},
					}
				},
				MockModifyReplicationGroupRequest: func(_ *elasticache.ModifyReplicationGroupInput) elasticache.ModifyReplicationGroupRequest {
					return elasticache.ModifyReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Data: &elasticache.ModifyReplicationGroupOutput{}},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
			),
			want: replicationGroup(
				withState(v1alpha1.StatusAvailable),
				withGroupName(name),
				withConditions(
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionFalse},
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue},
				),
				withPort(port),
				withEndpoint(host),
				withMemberClusters([]string{cacheClusterID}),
			),
			wantRequeue: false,
		},
		{
			name: "SuccessfulSyncWhileGroupAvailableAndCacheClustersNeedUpdate",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{
									Status:                 aws.String(v1alpha1.StatusAvailable),
									MemberClusters:         []string{cacheClusterID},
									AutomaticFailover:      elasticache.AutomaticFailoverStatusEnabled,
									CacheNodeType:          aws.String(cacheNodeType),
									SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit),
									SnapshotWindow:         aws.String(snapshotWindow),
									ClusterEnabled:         aws.Bool(true),
									ConfigurationEndpoint:  &elasticache.Endpoint{Address: aws.String(host), Port: aws.Int64(port)},
								}},
							},
						},
					}
				},
				MockDescribeCacheClustersRequest: func(_ *elasticache.DescribeCacheClustersInput) elasticache.DescribeCacheClustersRequest {
					return elasticache.DescribeCacheClustersRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeCacheClustersOutput{
								CacheClusters: []elasticache.CacheCluster{{
									EngineVersion:              aws.String(engineVersion),
									PreferredMaintenanceWindow: aws.String("never!"), // This field needs to be updated.
								}},
							},
						},
					}
				},
				MockModifyReplicationGroupRequest: func(_ *elasticache.ModifyReplicationGroupInput) elasticache.ModifyReplicationGroupRequest {
					return elasticache.ModifyReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Data: &elasticache.ModifyReplicationGroupOutput{}},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
			),
			want: replicationGroup(
				withState(v1alpha1.StatusAvailable),
				withGroupName(name),
				withConditions(
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionFalse},
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue},
				),
				withPort(port),
				withEndpoint(host),
				withMemberClusters([]string{cacheClusterID}),
			),
			wantRequeue: false,
		},
		{
			name: "FailedDescribeReplicationGroups",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
			),
			want: replicationGroup(
				withGroupName(name),
				withConditions(
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue},
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonSyncingResource,
						Message: errorBoom.Error(),
					},
				),
			),
			wantRequeue: true,
		},
		{
			name: "FailedDescribeCacheClusters",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{
									Status:         aws.String(v1alpha1.StatusAvailable),
									ClusterEnabled: aws.Bool(true),
									MemberClusters: []string{cacheClusterID},
								}},
							},
						},
					}
				},
				MockDescribeCacheClustersRequest: func(_ *elasticache.DescribeCacheClustersInput) elasticache.DescribeCacheClustersRequest {
					return elasticache.DescribeCacheClustersRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
			),
			want: replicationGroup(
				withState(v1alpha1.StatusAvailable),
				withGroupName(name),
				withConditions(
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionFalse},
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue},
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonSyncingResource,
						Message: errors.Wrapf(errorBoom, "cannot describe cache cluster %s", cacheClusterID).Error(),
					},
				),
				withMemberClusters([]string{cacheClusterID}),
			),
			wantRequeue: true,
		},
		{
			name: "FailedModifyReplicationGroup",
			csd: &elastiCache{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{
									Status:                 aws.String(v1alpha1.StatusAvailable),
									MemberClusters:         []string{cacheClusterID},
									AutomaticFailover:      elasticache.AutomaticFailoverStatusEnabled,
									CacheNodeType:          aws.String(cacheNodeType),
									SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit),
									SnapshotWindow:         aws.String(snapshotWindow),
									ClusterEnabled:         aws.Bool(true),
									ConfigurationEndpoint:  &elasticache.Endpoint{Address: aws.String(host), Port: aws.Int64(port)},
								}},
							},
						},
					}
				},
				MockDescribeCacheClustersRequest: func(_ *elasticache.DescribeCacheClustersInput) elasticache.DescribeCacheClustersRequest {
					return elasticache.DescribeCacheClustersRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeCacheClustersOutput{
								CacheClusters: []elasticache.CacheCluster{{
									EngineVersion:              aws.String(engineVersion),
									PreferredMaintenanceWindow: aws.String("never!"), // This field needs to be updated.
								}},
							},
						},
					}
				},
				MockModifyReplicationGroupRequest: func(_ *elasticache.ModifyReplicationGroupInput) elasticache.ModifyReplicationGroupRequest {
					return elasticache.ModifyReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
			),
			want: replicationGroup(
				withState(v1alpha1.StatusAvailable),
				withGroupName(name),
				withConditions(
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionFalse},
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue},
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonSyncingResource,
						Message: errorBoom.Error(),
					},
				),
				withPort(port),
				withEndpoint(host),
				withMemberClusters([]string{cacheClusterID}),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Sync(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.Sync(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := cmp.Diff(tc.want, tc.r); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		r           *v1alpha1.ReplicationGroup
		want        *v1alpha1.ReplicationGroup
		wantRequeue bool
	}{
		{
			name: "ReclaimRetainSuccessfulDelete",
			csd:  &elastiCache{},
			r:    replicationGroup(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimRetain)),
			want: replicationGroup(
				withReclaimPolicy(corev1alpha1.ReclaimRetain),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "ReclaimDeleteSuccessfulDelete",
			csd: &elastiCache{client: &fake.MockClient{
				MockDeleteReplicationGroupRequest: func(_ *elasticache.DeleteReplicationGroupInput) elasticache.DeleteReplicationGroupRequest {
					return elasticache.DeleteReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Data: &elasticache.DeleteReplicationGroupOutput{}},
					}
				},
			}},
			r: replicationGroup(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimDelete)),
			want: replicationGroup(
				withReclaimPolicy(corev1alpha1.ReclaimDelete),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "ReclaimDeleteFailedDelete",
			csd: &elastiCache{client: &fake.MockClient{
				MockDeleteReplicationGroupRequest: func(_ *elasticache.DeleteReplicationGroupInput) elasticache.DeleteReplicationGroupRequest {
					return elasticache.DeleteReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimDelete)),
			want: replicationGroup(
				withFinalizers(finalizerName),
				withReclaimPolicy(corev1alpha1.ReclaimDelete),
				withConditions(
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonDeletingResource,
						Message: errorBoom.Error(),
					},
				),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Delete(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.Delete(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := cmp.Diff(tc.want, tc.r); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnect(t *testing.T) {
	cases := []struct {
		name    string
		conn    connecter
		i       *v1alpha1.ReplicationGroup
		want    createsyncdeleter
		wantErr error
	}{
		{
			name: "SuccessfulConnect",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*awsv1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClient: func(_ []byte, _ string) (elasticacheclient.Client, error) { return &fake.MockClient{}, nil },
			},
			i:    replicationGroup(),
			want: &elastiCache{client: &fake.MockClient{}},
		},
		{
			name: "FailedToGetProvider",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return kerrors.NewNotFound(schema.GroupResource{}, providerName)
				}},
				newClient: func(_ []byte, _ string) (elasticacheclient.Client, error) { return &fake.MockClient{}, nil },
			},
			i:       replicationGroup(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider %s/%s:  \"%s\" not found", namespace, providerName, providerName)),
		},
		{
			name: "FailedToGetProviderSecret",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*awsv1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return kerrors.NewNotFound(schema.GroupResource{}, providerSecretName)
					}
					return nil
				}},
				newClient: func(_ []byte, _ string) (elasticacheclient.Client, error) { return &fake.MockClient{}, nil },
			},
			i:       replicationGroup(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider secret %s/%s:  \"%s\" not found", namespace, providerSecretName, providerSecretName)),
		},
		{
			name: "FailedToCreateElastiCacheClient",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*awsv1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClient: func(_ []byte, _ string) (elasticacheclient.Client, error) { return nil, errorBoom },
			},
			i:       replicationGroup(),
			want:    &elastiCache{},
			wantErr: errors.Wrap(errorBoom, "cannot create new AWS Replication Group client"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := tc.conn.Connect(ctx, tc.i)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.conn.Connect(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(elastiCache{})); diff != "" {
				t.Errorf("tc.conn.Connect(...): -want, +got:\n%s", diff)
			}
		})
	}
}

type mockConnector struct {
	MockConnect func(ctx context.Context, i *v1alpha1.ReplicationGroup) (createsyncdeleter, error)
}

func (c *mockConnector) Connect(ctx context.Context, i *v1alpha1.ReplicationGroup) (createsyncdeleter, error) {
	return c.MockConnect(ctx, i)
}

type mockCSD struct {
	MockCreate func(ctx context.Context, g *v1alpha1.ReplicationGroup) (bool, string)
	MockSync   func(ctx context.Context, g *v1alpha1.ReplicationGroup) bool
	MockDelete func(ctx context.Context, g *v1alpha1.ReplicationGroup) bool
}

func (csd *mockCSD) Create(ctx context.Context, g *v1alpha1.ReplicationGroup) (bool, string) {
	return csd.MockCreate(ctx, g)
}

func (csd *mockCSD) Sync(ctx context.Context, g *v1alpha1.ReplicationGroup) bool {
	return csd.MockSync(ctx, g)
}

func (csd *mockCSD) Delete(ctx context.Context, g *v1alpha1.ReplicationGroup) bool {
	return csd.MockDelete(ctx, g)
}

func TestReconcile(t *testing.T) {
	cases := []struct {
		name    string
		rec     *Reconciler
		req     reconcile.Request
		want    reconcile.Result
		wantErr error
	}{
		{
			name: "SuccessfulDelete",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.ReplicationGroup) (createsyncdeleter, error) {
					return &mockCSD{MockDelete: func(_ context.Context, _ *v1alpha1.ReplicationGroup) bool { return false }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.ReplicationGroup) = *(replicationGroup(withGroupName(name), withDeletionTimestamp(time.Now())))
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "SuccessfulCreate",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.ReplicationGroup) (createsyncdeleter, error) {
					return &mockCSD{MockCreate: func(_ context.Context, _ *v1alpha1.ReplicationGroup) (bool, string) { return true, "" }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: name}:
							*obj.(*v1alpha1.ReplicationGroup) = *(replicationGroup())
						case client.ObjectKey{Namespace: namespace, Name: connectionSecretName}:
							return kerrors.NewNotFound(schema.GroupResource{}, connectionSecretName)
						}
						return nil
					},
					MockCreate: func(_ context.Context, _ runtime.Object) error { return nil },
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "SuccessfulSync",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.ReplicationGroup) (createsyncdeleter, error) {
					return &mockCSD{
						MockSync: func(_ context.Context, _ *v1alpha1.ReplicationGroup) bool { return false },
					}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: name}:
							*obj.(*v1alpha1.ReplicationGroup) = *(replicationGroup(withGroupName(name), withEndpoint(host)))
						case client.ObjectKey{Namespace: namespace, Name: connectionSecretName}:
							*obj.(*corev1.Secret) = corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: connectionSecretName},
								Data:       map[string][]byte{corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(authToken)},
							}
						}
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "FailedToGetNonexistentResource",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, name)
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "FailedToGetExtantResource",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errorBoom
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: false},
			wantErr: errors.Wrapf(errorBoom, "cannot get resource %s/%s", namespace, name),
		},
		{
			name: "FailedToConnect",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.ReplicationGroup) (createsyncdeleter, error) {
					return nil, errorBoom
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.ReplicationGroup) = *(replicationGroup())
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := replicationGroup(withConditions(
							corev1alpha1.DeprecatedCondition{
								Type:    corev1alpha1.DeprecatedFailed,
								Status:  corev1.ConditionTrue,
								Reason:  reasonFetchingClient,
								Message: errorBoom.Error(),
							},
						))
						got := obj.(*v1alpha1.ReplicationGroup)
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("kube.Update(...): -want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "FailedToGetConnectionSecretDuringCreate",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.ReplicationGroup) (createsyncdeleter, error) {
					return &mockCSD{MockCreate: func(_ context.Context, _ *v1alpha1.ReplicationGroup) (bool, string) { return true, authToken }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case types.NamespacedName{Namespace: namespace, Name: connectionSecretName}:
							return errorBoom
						case types.NamespacedName{Namespace: namespace, Name: name}:
							*obj.(*v1alpha1.ReplicationGroup) = *(replicationGroup())
						}
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := replicationGroup(
							withConditions(
								corev1alpha1.DeprecatedCondition{
									Type:    corev1alpha1.DeprecatedFailed,
									Status:  corev1.ConditionTrue,
									Reason:  reasonSyncingSecret,
									Message: errors.Wrapf(errorBoom, "cannot get secret %s/%s", namespace, connectionSecretName).Error(),
								},
							))
						got := obj.(*v1alpha1.ReplicationGroup)
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("kube.Update(...): -want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "FailedToGetConnectionSecretDuringSync",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.ReplicationGroup) (createsyncdeleter, error) {
					return nil, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case types.NamespacedName{Namespace: namespace, Name: connectionSecretName}:
							return errorBoom
						case types.NamespacedName{Namespace: namespace, Name: name}:
							*obj.(*v1alpha1.ReplicationGroup) = *(replicationGroup(withGroupName(name)))
						}
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := replicationGroup(
							withGroupName(name),
							withConditions(
								corev1alpha1.DeprecatedCondition{
									Type:    corev1alpha1.DeprecatedFailed,
									Status:  corev1.ConditionTrue,
									Reason:  reasonSyncingSecret,
									Message: errors.Wrapf(errorBoom, "cannot get secret %s/%s", namespace, connectionSecretName).Error(),
								},
							))
						got := obj.(*v1alpha1.ReplicationGroup)
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("kube.Update(...): -want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, gotErr := tc.rec.Reconcile(tc.req)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, gotResult); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnectionSecretWithPassword(t *testing.T) {
	cases := []struct {
		name     string
		r        *v1alpha1.ReplicationGroup
		password string
		want     *corev1.Secret
	}{
		{
			name:     "Successful",
			r:        replicationGroup(withEndpoint(host)),
			password: authToken,
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      connectionSecretName,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: v1alpha1.APIVersion,
						Kind:       v1alpha1.ReplicationGroupKind,
						Name:       name,
						UID:        uid,
					}},
				},
				Data: map[string][]byte{
					corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(host),
					corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(authToken),
				},
			},
		},
		{
			name:     "EmptyPassword",
			r:        replicationGroup(withEndpoint(host)),
			password: "",
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      connectionSecretName,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: v1alpha1.APIVersion,
						Kind:       v1alpha1.ReplicationGroupKind,
						Name:       name,
						UID:        uid,
					}},
				},
				Data: map[string][]byte{
					corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(host),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectionSecretWithPassword(tc.r, tc.password)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("connectionSecretWithPassword(...): -want, +got:\n%s", diff)
			}
		})
	}
}
