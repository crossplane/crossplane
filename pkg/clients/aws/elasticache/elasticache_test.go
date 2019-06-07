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

package elasticache

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplaneio/crossplane/pkg/apis/aws/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws"
)

const (
	namespace   = "coolNamespace"
	name        = "coolGroup"
	uid         = types.UID("definitely-a-uuid")
	id          = NamePrefix + "-efdd8494195d7940" // FNV-64a hash of uid
	description = "Crossplane managed " + v1alpha1.ReplicationGroupKindAPIVersion + " " + namespace + "/" + name

	cacheNodeType            = "n1.super.cool"
	atRestEncryptionEnabled  = true
	authToken                = "coolToken"
	autoFailoverEnabled      = true
	cacheParameterGroupName  = "coolParamGroup"
	cacheSubnetGroupName     = "coolSubnet"
	engineVersion            = "5.0.0"
	notificationTopicARN     = "arn:aws:sns:cooltopic"
	numCacheClusters         = 2
	numNodeGroups            = 2
	host                     = "coolhost"
	port                     = 6379
	maintenanceWindow        = "tomorrow"
	replicasPerNodeGroup     = 2
	snapshotName             = "coolSnapshot"
	snapshotRetentionLimit   = 1
	snapshotWindow           = "thedayaftertomorrow"
	transitEncryptionEnabled = true

	nodeGroupPrimaryAZ    = "us-cool-1a"
	nodeGroupReplicaCount = 2
	nodeGroupSlots        = "coolslots"

	cacheClusterID = id + "-0001"
)

var (
	cacheSecurityGroupNames  = []string{"coolGroup", "coolerGroup"}
	preferredCacheClusterAZs = []string{"us-cool-1a", "us-cool-1b"}
	securityGroupIDs         = []string{"coolID", "coolerID"}
	snapshotARNs             = []string{"arn:aws:s3:snappy"}

	nodeGroupAZs = []string{"us-cool-1a", "us-cool-1b"}

	meta             = metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid}
	replicationGroup = &v1alpha1.ReplicationGroup{
		ObjectMeta: meta,
		Spec: v1alpha1.ReplicationGroupSpec{
			CacheNodeType:            cacheNodeType,
			AtRestEncryptionEnabled:  atRestEncryptionEnabled,
			AutomaticFailoverEnabled: autoFailoverEnabled,
			CacheParameterGroupName:  cacheParameterGroupName,
			CacheSecurityGroupNames:  cacheSecurityGroupNames,
			CacheSubnetGroupName:     cacheSubnetGroupName,
			EngineVersion:            engineVersion,
			NodeGroupConfiguration: []v1alpha1.NodeGroupConfigurationSpec{
				{
					PrimaryAvailabilityZone:  nodeGroupPrimaryAZ,
					ReplicaAvailabilityZones: nodeGroupAZs,
					ReplicaCount:             nodeGroupReplicaCount,
					Slots:                    nodeGroupSlots,
				},
			},
			NotificationTopicARN:       notificationTopicARN,
			NumCacheClusters:           numCacheClusters,
			NumNodeGroups:              numNodeGroups,
			Port:                       port,
			PreferredCacheClusterAZs:   preferredCacheClusterAZs,
			PreferredMaintenanceWindow: maintenanceWindow,
			ReplicasPerNodeGroup:       replicasPerNodeGroup,
			SecurityGroupIDs:           securityGroupIDs,
			SnapshotARNs:               snapshotARNs,
			SnapshotName:               snapshotName,
			SnapshotRetentionLimit:     snapshotRetentionLimit,
			SnapshotWindow:             snapshotWindow,
			TransitEncryptionEnabled:   transitEncryptionEnabled,
		},
	}
)

func TestNewReplicationGroupID(t *testing.T) {
	cases := []struct {
		name   string
		object metav1.Object
		want   string
	}{
		{
			name:   "Successful",
			object: replicationGroup,
			want:   id,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewReplicationGroupID(tc.object)
			if got != tc.want {
				t.Errorf("NewReplicationGroupID(...): want %s, got %s", tc.want, got)
			}
		})
	}
}

func TestNewReplicationGroupDescription(t *testing.T) {
	cases := []struct {
		name  string
		group *v1alpha1.ReplicationGroup
		want  string
	}{
		{
			name:  "Successful",
			group: replicationGroup,
			want:  description,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewReplicationGroupDescription(tc.group)
			if got != tc.want {
				t.Errorf("NewReplicationGroupDescription(...): want %s, got %s", tc.want, got)
			}
		})
	}
}

func TestNewCreateReplicationGroupInput(t *testing.T) {
	cases := []struct {
		name      string
		group     *v1alpha1.ReplicationGroup
		authToken string
		want      *elasticache.CreateReplicationGroupInput
	}{
		{
			name:      "AllPossibleFields",
			group:     replicationGroup,
			authToken: authToken,
			want: &elasticache.CreateReplicationGroupInput{
				ReplicationGroupId:          aws.String(id, aws.FieldRequired),
				ReplicationGroupDescription: aws.String(description, aws.FieldRequired),
				Engine:                      aws.String(v1alpha1.CacheEngineRedis, aws.FieldRequired),
				CacheNodeType:               aws.String(cacheNodeType, aws.FieldRequired),
				AtRestEncryptionEnabled:     aws.Bool(atRestEncryptionEnabled),
				AuthToken:                   aws.String(authToken),
				AutomaticFailoverEnabled:    aws.Bool(autoFailoverEnabled),
				CacheParameterGroupName:     aws.String(cacheParameterGroupName),
				CacheSecurityGroupNames:     cacheSecurityGroupNames,
				CacheSubnetGroupName:        aws.String(cacheSubnetGroupName),
				EngineVersion:               aws.String(engineVersion),
				NodeGroupConfiguration: []elasticache.NodeGroupConfiguration{
					{
						PrimaryAvailabilityZone:  aws.String(nodeGroupPrimaryAZ),
						ReplicaAvailabilityZones: nodeGroupAZs,
						ReplicaCount:             aws.Int64(nodeGroupReplicaCount),
						Slots:                    aws.String(nodeGroupSlots),
					},
				},
				NotificationTopicArn:       aws.String(notificationTopicARN),
				NumCacheClusters:           aws.Int64(numCacheClusters),
				NumNodeGroups:              aws.Int64(numNodeGroups),
				Port:                       aws.Int64(port),
				PreferredCacheClusterAZs:   preferredCacheClusterAZs,
				PreferredMaintenanceWindow: aws.String(maintenanceWindow),
				ReplicasPerNodeGroup:       aws.Int64(replicasPerNodeGroup),
				SecurityGroupIds:           securityGroupIDs,
				SnapshotArns:               snapshotARNs,
				SnapshotName:               aws.String(snapshotName),
				SnapshotRetentionLimit:     aws.Int64(snapshotRetentionLimit),
				SnapshotWindow:             aws.String(snapshotWindow),
				TransitEncryptionEnabled:   aws.Bool(transitEncryptionEnabled),
			},
		},
		{
			name: "UnsetFieldsAreNilNotZeroType",
			group: &v1alpha1.ReplicationGroup{
				ObjectMeta: meta,
				Spec:       v1alpha1.ReplicationGroupSpec{CacheNodeType: cacheNodeType},
			},
			want: &elasticache.CreateReplicationGroupInput{
				ReplicationGroupId:          aws.String(id, aws.FieldRequired),
				ReplicationGroupDescription: aws.String(description, aws.FieldRequired),
				Engine:                      aws.String(v1alpha1.CacheEngineRedis, aws.FieldRequired),
				CacheNodeType:               aws.String(cacheNodeType, aws.FieldRequired),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewCreateReplicationGroupInput(tc.group, tc.authToken)

			if err := got.Validate(); err != nil {
				t.Errorf("NewCreateReplicationGroupInput(...): invalid input: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewCreateReplicationGroupInput(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNewModifyReplicationGroupInput(t *testing.T) {
	cases := []struct {
		name  string
		group *v1alpha1.ReplicationGroup
		want  *elasticache.ModifyReplicationGroupInput
	}{
		{
			name:  "AllPossibleFields",
			group: replicationGroup,
			want: &elasticache.ModifyReplicationGroupInput{
				ReplicationGroupId:         aws.String(id, aws.FieldRequired),
				ApplyImmediately:           aws.Bool(true),
				AutomaticFailoverEnabled:   aws.Bool(autoFailoverEnabled),
				CacheNodeType:              aws.String(cacheNodeType),
				CacheParameterGroupName:    aws.String(cacheParameterGroupName),
				CacheSecurityGroupNames:    cacheSecurityGroupNames,
				EngineVersion:              aws.String(engineVersion),
				NotificationTopicArn:       aws.String(notificationTopicARN),
				PreferredMaintenanceWindow: aws.String(maintenanceWindow),
				SecurityGroupIds:           securityGroupIDs,
				SnapshotRetentionLimit:     aws.Int64(snapshotRetentionLimit),
				SnapshotWindow:             aws.String(snapshotWindow),
			},
		},
		{
			name:  "UnsetFieldsAreNilNotZeroType",
			group: &v1alpha1.ReplicationGroup{ObjectMeta: meta},
			want: &elasticache.ModifyReplicationGroupInput{
				ReplicationGroupId: aws.String(id, aws.FieldRequired),
				ApplyImmediately:   aws.Bool(true),
			},
		},
		{
			name: "SuperfluousFields",
			group: &v1alpha1.ReplicationGroup{
				ObjectMeta: meta,

				// AtRestEncryptionEnabled cannot be modified
				Spec: v1alpha1.ReplicationGroupSpec{AtRestEncryptionEnabled: true},
			},
			want: &elasticache.ModifyReplicationGroupInput{
				ReplicationGroupId: aws.String(id, aws.FieldRequired),
				ApplyImmediately:   aws.Bool(true),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewModifyReplicationGroupInput(tc.group)

			if err := got.Validate(); err != nil {
				t.Errorf("NewModifyReplicationGroupInput(...): invalid input: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewModifyReplicationGroupInput(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNewDeleteReplicationGroupInput(t *testing.T) {
	cases := []struct {
		name  string
		group *v1alpha1.ReplicationGroup
		want  *elasticache.DeleteReplicationGroupInput
	}{
		{
			name:  "Successful",
			group: replicationGroup,
			want:  &elasticache.DeleteReplicationGroupInput{ReplicationGroupId: aws.String(id, aws.FieldRequired)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewDeleteReplicationGroupInput(tc.group)

			if err := got.Validate(); err != nil {
				t.Errorf("NewDeleteReplicationGroupInput(...): invalid input: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewDeleteReplicationGroupInput(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNewDescribeReplicationGroupsInput(t *testing.T) {
	cases := []struct {
		name  string
		group *v1alpha1.ReplicationGroup
		want  *elasticache.DescribeReplicationGroupsInput
	}{
		{
			name:  "Successful",
			group: replicationGroup,
			want:  &elasticache.DescribeReplicationGroupsInput{ReplicationGroupId: aws.String(id, aws.FieldRequired)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewDescribeReplicationGroupsInput(tc.group)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewDescribeReplicationGroupsInput(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNewDescribeCacheClustersInput(t *testing.T) {
	cases := []struct {
		name    string
		cluster string
		want    *elasticache.DescribeCacheClustersInput
	}{
		{
			name:    "Successful",
			cluster: cacheClusterID,
			want:    &elasticache.DescribeCacheClustersInput{CacheClusterId: aws.String(cacheClusterID, aws.FieldRequired)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewDescribeCacheClustersInput(tc.cluster)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewDescribeCacheClustersInput(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestReplicationGroupNeedsUpdate(t *testing.T) {
	cases := []struct {
		name string
		kube *v1alpha1.ReplicationGroup
		rg   elasticache.ReplicationGroup
		want bool
	}{
		{
			name: "NeedsFailoverEnabled",
			kube: replicationGroup,
			rg:   elasticache.ReplicationGroup{AutomaticFailover: elasticache.AutomaticFailoverStatusDisabled},
			want: true,
		},
		{
			name: "NeedsNewCacheNodeType",
			kube: replicationGroup,
			rg: elasticache.ReplicationGroup{
				AutomaticFailover: elasticache.AutomaticFailoverStatusEnabling,
				CacheNodeType:     aws.String("n1.insufficiently.cool"),
			},
			want: true,
		},
		{
			name: "NeedsNewSnapshotRetentionLimit",
			kube: replicationGroup,
			rg: elasticache.ReplicationGroup{
				AutomaticFailover:      elasticache.AutomaticFailoverStatusEnabling,
				CacheNodeType:          aws.String(cacheNodeType),
				SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit + 1),
			},
			want: true,
		},
		{
			name: "NeedsNewSnapshotWindow",
			kube: replicationGroup,
			rg: elasticache.ReplicationGroup{
				AutomaticFailover:      elasticache.AutomaticFailoverStatusEnabling,
				CacheNodeType:          aws.String(cacheNodeType),
				SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit),
				SnapshotWindow:         aws.String("yesterday"),
			},
			want: true,
		},
		{
			name: "NeedsNoUpdate",
			kube: replicationGroup,
			rg: elasticache.ReplicationGroup{
				AutomaticFailover:      elasticache.AutomaticFailoverStatusEnabling,
				CacheNodeType:          aws.String(cacheNodeType),
				SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit),
				SnapshotWindow:         aws.String(snapshotWindow),
			},
			want: false,
		},
		{
			// AWS autopopulates the snapshot window if we don't set it, so we
			// want to make sure we don't consider it to need an update if we
			// never specified a value in the first place.
			name: "NeedsNoUpdateSnapshotWindowAutoPopulated",
			kube: func() *v1alpha1.ReplicationGroup {
				g := replicationGroup.DeepCopy()
				g.Spec.SnapshotWindow = ""
				return g
			}(),
			rg: elasticache.ReplicationGroup{
				AutomaticFailover:      elasticache.AutomaticFailoverStatusEnabling,
				CacheNodeType:          aws.String(cacheNodeType),
				SnapshotRetentionLimit: aws.Int64(snapshotRetentionLimit),
				SnapshotWindow:         aws.String(snapshotWindow),
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReplicationGroupNeedsUpdate(tc.kube, tc.rg)
			if got != tc.want {
				t.Errorf("ReplicationGroupNeedsUpdate(...): want %t, got %t", tc.want, got)
			}
		})
	}
}

func TestCacheClusterNeedsUpdate(t *testing.T) {
	cases := []struct {
		name string
		kube *v1alpha1.ReplicationGroup
		cc   elasticache.CacheCluster
		want bool
	}{
		{
			name: "NeedsNewEngineVersion",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion: aws.String("4.0.0"),
			},
			want: true,
		},
		{
			name: "NeedsNewCacheParameterGroup",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion:       aws.String(engineVersion),
				CacheParameterGroup: &elasticache.CacheParameterGroupStatus{CacheParameterGroupName: aws.String("okaygroupiguess")},
			},
			want: true,
		},
		{
			name: "NeedsNewNotificationTopicARN",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion:             aws.String(engineVersion),
				CacheParameterGroup:       &elasticache.CacheParameterGroupStatus{CacheParameterGroupName: aws.String(cacheParameterGroupName)},
				NotificationConfiguration: &elasticache.NotificationConfiguration{TopicArn: aws.String("aws:arn:sqs:nope")},
			},
			want: true,
		},
		{
			name: "NeedsNewMaintenanceWindow",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion:              aws.String(engineVersion),
				CacheParameterGroup:        &elasticache.CacheParameterGroupStatus{CacheParameterGroupName: aws.String(cacheParameterGroupName)},
				NotificationConfiguration:  &elasticache.NotificationConfiguration{TopicArn: aws.String(notificationTopicARN)},
				PreferredMaintenanceWindow: aws.String("never!"),
			},
			want: true,
		},
		{
			name: "NeedsNewSecurityGroupIDs",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion:              aws.String(engineVersion),
				CacheParameterGroup:        &elasticache.CacheParameterGroupStatus{CacheParameterGroupName: aws.String(cacheParameterGroupName)},
				NotificationConfiguration:  &elasticache.NotificationConfiguration{TopicArn: aws.String(notificationTopicARN)},
				PreferredMaintenanceWindow: aws.String(maintenanceWindow),
				SecurityGroups: []elasticache.SecurityGroupMembership{
					{SecurityGroupId: aws.String("notaverysecuregroupid")},
					{SecurityGroupId: aws.String("evenlesssecuregroupid")},
				},
			},
			want: true,
		},
		{
			name: "NeedsSecurityGroupIDs",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion:              aws.String(engineVersion),
				CacheParameterGroup:        &elasticache.CacheParameterGroupStatus{CacheParameterGroupName: aws.String(cacheParameterGroupName)},
				NotificationConfiguration:  &elasticache.NotificationConfiguration{TopicArn: aws.String(notificationTopicARN)},
				PreferredMaintenanceWindow: aws.String(maintenanceWindow),
			},
			want: true,
		},
		{
			name: "NeedsNewSecurityGroupNames",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion:              aws.String(engineVersion),
				CacheParameterGroup:        &elasticache.CacheParameterGroupStatus{CacheParameterGroupName: aws.String(cacheParameterGroupName)},
				NotificationConfiguration:  &elasticache.NotificationConfiguration{TopicArn: aws.String(notificationTopicARN)},
				PreferredMaintenanceWindow: aws.String(maintenanceWindow),
				SecurityGroups: func() []elasticache.SecurityGroupMembership {
					ids := make([]elasticache.SecurityGroupMembership, len(securityGroupIDs))
					for i, id := range securityGroupIDs {
						ids[i] = elasticache.SecurityGroupMembership{SecurityGroupId: aws.String(id)}
					}
					return ids
				}(),
				CacheSecurityGroups: []elasticache.CacheSecurityGroupMembership{
					{CacheSecurityGroupName: aws.String("notaverysecuregroup")},
					{CacheSecurityGroupName: aws.String("evenlesssecuregroup")},
				},
			},
			want: true,
		},
		{
			name: "NeedsSecurityGroupNames",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion:              aws.String(engineVersion),
				CacheParameterGroup:        &elasticache.CacheParameterGroupStatus{CacheParameterGroupName: aws.String(cacheParameterGroupName)},
				NotificationConfiguration:  &elasticache.NotificationConfiguration{TopicArn: aws.String(notificationTopicARN)},
				PreferredMaintenanceWindow: aws.String(maintenanceWindow),
				SecurityGroups: func() []elasticache.SecurityGroupMembership {
					ids := make([]elasticache.SecurityGroupMembership, len(securityGroupIDs))
					for i, id := range securityGroupIDs {
						ids[i] = elasticache.SecurityGroupMembership{SecurityGroupId: aws.String(id)}
					}
					return ids
				}(),
			},
			want: true,
		},
		{
			name: "NeedsNoUpdate",
			kube: replicationGroup,
			cc: elasticache.CacheCluster{
				EngineVersion:              aws.String(engineVersion),
				CacheParameterGroup:        &elasticache.CacheParameterGroupStatus{CacheParameterGroupName: aws.String(cacheParameterGroupName)},
				NotificationConfiguration:  &elasticache.NotificationConfiguration{TopicArn: aws.String(notificationTopicARN)},
				PreferredMaintenanceWindow: aws.String(maintenanceWindow),
				SecurityGroups: func() []elasticache.SecurityGroupMembership {
					ids := make([]elasticache.SecurityGroupMembership, len(securityGroupIDs))
					for i, id := range securityGroupIDs {
						ids[i] = elasticache.SecurityGroupMembership{SecurityGroupId: aws.String(id)}
					}
					return ids
				}(),
				CacheSecurityGroups: func() []elasticache.CacheSecurityGroupMembership {
					names := make([]elasticache.CacheSecurityGroupMembership, len(cacheSecurityGroupNames))
					for i, n := range cacheSecurityGroupNames {
						names[i] = elasticache.CacheSecurityGroupMembership{CacheSecurityGroupName: aws.String(n)}
					}
					return names
				}(),
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CacheClusterNeedsUpdate(tc.kube, tc.cc)
			if got != tc.want {
				t.Errorf("CacheClusterNeedsUpdate(...): want %t, got %t", tc.want, got)
			}
		})
	}

}

func TestConnectionEndpoint(t *testing.T) {
	cases := []struct {
		name string
		rg   elasticache.ReplicationGroup
		want Endpoint
	}{
		{
			name: "ClusterModeEnabled",
			rg: elasticache.ReplicationGroup{
				ClusterEnabled: aws.Bool(true),
				ConfigurationEndpoint: &elasticache.Endpoint{
					Address: aws.String(host),
					Port:    aws.Int64(port),
				},
			},
			want: Endpoint{Address: host, Port: port},
		},
		{
			name: "ClusterModeEnabledMissingConfigurationEndpoint",
			rg: elasticache.ReplicationGroup{
				ClusterEnabled: aws.Bool(true),
			},
			want: Endpoint{},
		},
		{
			name: "ClusterModeDisabled",
			rg: elasticache.ReplicationGroup{
				NodeGroups: []elasticache.NodeGroup{{
					PrimaryEndpoint: &elasticache.Endpoint{
						Address: aws.String(host),
						Port:    aws.Int64(port),
					}},
				},
			},
			want: Endpoint{Address: host, Port: port},
		},
		{
			name: "ClusterModeDisabledMissingPrimaryEndpoint",
			rg:   elasticache.ReplicationGroup{NodeGroups: []elasticache.NodeGroup{{}}},
			want: Endpoint{},
		},
		{
			name: "ClusterModeDisabledMissingNodeGroups",
			rg:   elasticache.ReplicationGroup{},
			want: Endpoint{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ConnectionEndpoint(tc.rg)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ConnectionEndpoint(...): -want, +got:\n%s", diff)
			}
		})
	}
}
