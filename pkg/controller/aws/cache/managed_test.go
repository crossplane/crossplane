/*
Copyright 2019 The Crossplane Authors.

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
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
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

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/aws/apis/cache/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/aws/apis/v1alpha1"
	elasticacheclient "github.com/crossplaneio/crossplane/pkg/clients/aws/elasticache"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/elasticache/fake"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "coolNamespace"
	name      = "coolGroup"
	uid       = types.UID("definitely-a-uuid")
	id        = elasticacheclient.NamePrefix + "-efdd8494195d7940" // FNV-64a hash of uid

	cacheNodeType            = "n1.super.cool"
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

	objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid}

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

type testCase struct {
	name         string
	e            resource.ExternalClient
	r            *v1alpha1.ReplicationGroup
	want         *v1alpha1.ReplicationGroup
	tokenCreated bool
	returnsErr   bool
}

type replicationGroupModifier func(*v1alpha1.ReplicationGroup)

func withConditions(c ...corev1alpha1.Condition) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.ConditionedStatus.Conditions = c }
}

func withBindingPhase(p corev1alpha1.BindingPhase) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.SetBindingPhase(p) }
}

func withState(s string) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.State = s }
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

func withAuth() replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Spec.AuthEnabled = true }
}

func withMemberClusters(members []string) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.MemberClusters = members }
}

func withClusterEnabled(e bool) replicationGroupModifier {
	return func(r *v1alpha1.ReplicationGroup) { r.Status.ClusterEnabled = e }
}

func replicationGroup(rm ...replicationGroupModifier) *v1alpha1.ReplicationGroup {
	r := &v1alpha1.ReplicationGroup{
		ObjectMeta: objectMeta,
		Spec: v1alpha1.ReplicationGroupSpec{
			ResourceSpec: corev1alpha1.ResourceSpec{
				ProviderReference:                &corev1.ObjectReference{Namespace: namespace, Name: providerName},
				WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: connectionSecretName},
			},
			ReplicationGroupParameters: v1alpha1.ReplicationGroupParameters{
				AutomaticFailoverEnabled:   autoFailoverEnabled,
				CacheNodeType:              cacheNodeType,
				CacheParameterGroupName:    cacheParameterGroupName,
				EngineVersion:              engineVersion,
				PreferredMaintenanceWindow: maintenanceWindow,
				SnapshotRetentionLimit:     snapshotRetentionLimit,
				SnapshotWindow:             snapshotWindow,
				TransitEncryptionEnabled:   transitEncryptionEnabled,
			},
		},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ resource.ExternalClient = &external{}
var _ resource.ExternalConnecter = &connecter{}

func TestCreate(t *testing.T) {
	cases := []testCase{
		{
			name: "SuccessfulCreate",
			e: &external{client: &fake.MockClient{
				MockCreateReplicationGroupRequest: func(_ *elasticache.CreateReplicationGroupInput) elasticache.CreateReplicationGroupRequest {
					return elasticache.CreateReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Data: &elasticache.CreateReplicationGroupOutput{}},
					}
				},
			}},
			r: replicationGroup(withAuth()),
			want: replicationGroup(
				withAuth(),
				withConditions(corev1alpha1.Creating()),
				withGroupName(id),
			),
			tokenCreated: true,
		},
		{
			name: "FailedCreate",
			e: &external{client: &fake.MockClient{
				MockCreateReplicationGroupRequest: func(_ *elasticache.CreateReplicationGroupInput) elasticache.CreateReplicationGroupRequest {
					return elasticache.CreateReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(),
			want: replicationGroup(
				withConditions(corev1alpha1.Creating()),
				withGroupName(id),
			),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			creation, err := tc.e.Create(ctx, tc.r)
			if tc.returnsErr != (err != nil) {
				t.Errorf("tc.e.Create(...) error: want: %t got: %t", tc.returnsErr, err != nil)
			}

			if tc.tokenCreated != (len(creation.ConnectionDetails[corev1alpha1.ResourceCredentialsSecretPasswordKey]) != 0) {
				t.Errorf("tc.e.Create(...) token creation: want: %t got: %t", tc.tokenCreated, len(creation.ConnectionDetails) != 0)
			}
			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestObserve(t *testing.T) {
	cases := []testCase{
		{
			name: "SuccessfulObserveWhileGroupCreating",
			e: &external{client: &fake.MockClient{
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
			r: replicationGroup(withGroupName(name)),
			want: replicationGroup(
				withState(v1alpha1.StatusCreating),
				withGroupName(name),
				withConditions(corev1alpha1.Creating()),
			),
		},
		{
			name: "SuccessfulObserveWhileGroupDeleting",
			e: &external{client: &fake.MockClient{
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
			),
			want: replicationGroup(
				withGroupName(name),
				withState(v1alpha1.StatusDeleting),
				withConditions(corev1alpha1.Deleting()),
			),
		},
		{
			name: "SuccessfulObserveWhileGroupModifying",
			e: &external{client: &fake.MockClient{
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
			),
			want: replicationGroup(
				withState(v1alpha1.StatusModifying),
				withGroupName(name),
			),
		},
		{
			name: "SuccessfulObserveAfterCreationCompleted",
			e: &external{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{
									ClusterEnabled:        aws.Bool(true),
									Status:                aws.String(v1alpha1.StatusAvailable),
									ConfigurationEndpoint: &elasticache.Endpoint{Address: aws.String(host), Port: aws.Int64(port)},
								}},
							},
						},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.Creating()),
				withClusterEnabled(true),
			),
			want: replicationGroup(
				withGroupName(name),
				withState(v1alpha1.StatusAvailable),
				withConditions(corev1alpha1.Available()),
				withBindingPhase(corev1alpha1.BindingPhaseUnbound),
				withEndpoint(host),
				withPort(port),
				withClusterEnabled(true),
			),
			tokenCreated: true,
		},
		{
			name: "FailedDescribeReplicationGroups",
			e: &external{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.Available()),
			),
			want: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.Available()),
			),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observation, err := tc.e.Observe(ctx, tc.r)
			if tc.returnsErr != (err != nil) {
				t.Errorf("tc.e.Observe(...) error: want: %t got: %t", tc.returnsErr, err != nil)
			}

			if tc.tokenCreated != (len(observation.ConnectionDetails[corev1alpha1.ResourceCredentialsSecretEndpointKey]) != 0) {
				t.Errorf("tc.e.Observe(...) token creation: want: %t got: %t", tc.tokenCreated, len(observation.ConnectionDetails) != 0)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	cases := []testCase{
		{
			name: "SuccessfulGroupDoesNotNeedUpdate",
			e: &external{client: &fake.MockClient{
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
				withMemberClusters([]string{cacheClusterID}),
			),
			want: replicationGroup(
				withGroupName(name),
				withMemberClusters([]string{cacheClusterID}),
			),
		},
		{
			name: "SuccessfulGroupNeedsUpdate",
			e: &external{client: &fake.MockClient{
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
			),
			want: replicationGroup(
				withGroupName(name),
			),
		},
		{
			name: "SuccessfulGroupDoesNotNeedButCacheClustersNeedUpdate",
			e: &external{client: &fake.MockClient{
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
				withMemberClusters([]string{cacheClusterID}),
			),
			want: replicationGroup(
				withGroupName(name),
				withMemberClusters([]string{cacheClusterID}),
			),
		},
		{
			name: "FailedDescribeReplicationGroups",
			e: &external{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.Creating()),
			),
			want: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.Creating()),
			),
			returnsErr: true,
		},
		{
			name: "FailedDescribeCacheClusters",
			e: &external{client: &fake.MockClient{
				MockDescribeReplicationGroupsRequest: func(_ *elasticache.DescribeReplicationGroupsInput) elasticache.DescribeReplicationGroupsRequest {
					return elasticache.DescribeReplicationGroupsRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Data: &elasticache.DescribeReplicationGroupsOutput{
								ReplicationGroups: []elasticache.ReplicationGroup{{
									Status:                 aws.String(v1alpha1.StatusAvailable),
									AutomaticFailover:      elasticache.AutomaticFailoverStatusEnabled,
									CacheNodeType:          aws.String(cacheNodeType),
									SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit),
									SnapshotWindow:         aws.String(snapshotWindow),
									ClusterEnabled:         aws.Bool(true),
									MemberClusters:         []string{cacheClusterID},
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
				MockModifyReplicationGroupRequest: func(_ *elasticache.ModifyReplicationGroupInput) elasticache.ModifyReplicationGroupRequest {
					return elasticache.ModifyReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Data: &elasticache.ModifyReplicationGroupOutput{}},
					}
				},
			}},
			r: replicationGroup(
				withGroupName(name),
				withClusterEnabled(true),
				withMemberClusters([]string{cacheClusterID}),
				withState(v1alpha1.StatusAvailable),
			),
			want: replicationGroup(
				withGroupName(name),
				withClusterEnabled(true),
				withMemberClusters([]string{cacheClusterID}),
				withState(v1alpha1.StatusAvailable),
			),
			returnsErr: true,
		},
		{
			name: "FailedModifyReplicationGroup",
			e: &external{client: &fake.MockClient{
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
				withConditions(corev1alpha1.Available()),
				withMemberClusters([]string{cacheClusterID}),
			),
			want: replicationGroup(
				withGroupName(name),
				withConditions(corev1alpha1.Available()),
				withMemberClusters([]string{cacheClusterID}),
			),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			update, err := tc.e.Update(ctx, tc.r)
			if tc.returnsErr != (err != nil) {
				t.Errorf("tc.e.Update(...) error: want: %t got: %t", tc.returnsErr, err != nil)
			}

			if tc.tokenCreated != (len(update.ConnectionDetails[corev1alpha1.ResourceCredentialsSecretPasswordKey]) != 0) {
				t.Errorf("tc.e.Update(...) token creation: want: %t got: %t", tc.tokenCreated, len(update.ConnectionDetails) != 0)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []testCase{
		{
			name: "Successful",
			e: &external{client: &fake.MockClient{
				MockDeleteReplicationGroupRequest: func(_ *elasticache.DeleteReplicationGroupInput) elasticache.DeleteReplicationGroupRequest {
					return elasticache.DeleteReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Data: &elasticache.DeleteReplicationGroupOutput{}},
					}
				},
			}},
			r: replicationGroup(),
			want: replicationGroup(
				withConditions(corev1alpha1.Deleting()),
			),
			returnsErr: false,
		},
		{
			name: "SuccessfulNotFound",
			e: &external{client: &fake.MockClient{
				MockDeleteReplicationGroupRequest: func(_ *elasticache.DeleteReplicationGroupInput) elasticache.DeleteReplicationGroupRequest {
					return elasticache.DeleteReplicationGroupRequest{
						Request: &aws.Request{
							HTTPRequest: &http.Request{},
							Error: awserr.New(
								elasticache.ErrCodeReplicationGroupNotFoundFault,
								"NotFound",
								fmt.Errorf("NotFound"))},
					}
				},
			}},
			r: replicationGroup(),
			want: replicationGroup(
				withConditions(corev1alpha1.Deleting()),
			),
			returnsErr: false,
		},
		{
			name: "Failed",
			e: &external{client: &fake.MockClient{
				MockDeleteReplicationGroupRequest: func(_ *elasticache.DeleteReplicationGroupInput) elasticache.DeleteReplicationGroupRequest {
					return elasticache.DeleteReplicationGroupRequest{
						Request: &aws.Request{HTTPRequest: &http.Request{}, Error: errorBoom},
					}
				},
			}},
			r: replicationGroup(),
			want: replicationGroup(
				withConditions(corev1alpha1.Deleting()),
			),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.e.Delete(ctx, tc.r)

			if tc.returnsErr != (err != nil) {
				t.Errorf("tc.csd.Delete(...) error: want: %t got: %t", tc.returnsErr, err != nil)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnect(t *testing.T) {
	cases := []struct {
		name    string
		conn    *connecter
		i       *v1alpha1.ReplicationGroup
		want    resource.ExternalClient
		wantErr error
	}{
		{
			name: "SuccessfulConnect",
			conn: &connecter{
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: providerName}:
							*obj.(*awsv1alpha1.Provider) = provider
						case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
							*obj.(*corev1.Secret) = providerSecret
						}
						return nil
					},
				},
				newClientFn: func(_ []byte, _ string) (elasticacheclient.Client, error) { return &fake.MockClient{}, nil },
			},
			i:    replicationGroup(),
			want: &external{client: &fake.MockClient{}},
		},
		{
			name: "FailedToGetProvider",
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return kerrors.NewNotFound(schema.GroupResource{}, providerName)
				}},
				newClientFn: func(_ []byte, _ string) (elasticacheclient.Client, error) { return &fake.MockClient{}, nil },
			},
			i:       replicationGroup(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider %s/%s:  \"%s\" not found", namespace, providerName, providerName)),
		},
		{
			name: "FailedToGetProviderSecret",
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*awsv1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return kerrors.NewNotFound(schema.GroupResource{}, providerSecretName)
					}
					return nil
				}},
				newClientFn: func(_ []byte, _ string) (elasticacheclient.Client, error) { return &fake.MockClient{}, nil },
			},
			i:       replicationGroup(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider secret %s/%s:  \"%s\" not found", namespace, providerSecretName, providerSecretName)),
		},
		{
			name: "FailedToCreateElastiCacheClient",
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*awsv1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClientFn: func(_ []byte, _ string) (elasticacheclient.Client, error) { return nil, errorBoom },
			},
			i:       replicationGroup(),
			want:    &external{},
			wantErr: errors.Wrap(errorBoom, errNewClient),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := tc.conn.Connect(ctx, tc.i)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.conn.Connect(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, got, test.EquateConditions(), cmp.AllowUnexported(external{})); diff != "" {
				t.Errorf("tc.conn.Connect(...): -want, +got:\n%s", diff)
			}
		})
	}
}
