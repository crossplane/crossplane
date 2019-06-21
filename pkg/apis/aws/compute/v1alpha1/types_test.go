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

package v1alpha1

import (
	"context"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
	name      = "test-cluster"
)

var (
	c   client.Client
	ctx = context.TODO()
)

var _ resource.Managed = &EKSCluster{}

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, SchemeBuilder.SchemeBuilder, test.CRDs())
	c = t.StartClient()
	t.StopAndExit(m.Run())
}

func TestEKSCluster(t *testing.T) {
	autoscaleSize := 1
	volSize := 20
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &EKSCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: EKSClusterSpec{
			Region:           "us-west-2",
			ClusterVersion:   "1.1.1",
			RoleARN:          "test-arn",
			SubnetIds:        []string{"one", "two"},
			SecurityGroupIds: []string{"sg-1", "sg-2"},
			WorkerNodes: WorkerNodesSpec{
				KeyName:                          "test-key-name",
				NodeImageID:                      "ami-id-test",
				NodeInstanceType:                 "t2.small",
				NodeAutoScalingGroupMinSize:      &autoscaleSize,
				NodeAutoScalingGroupMaxSize:      &autoscaleSize,
				NodeVolumeSize:                   &volSize,
				BootstrapArguments:               "test-bootstrap",
				NodeGroupName:                    "special-group-name",
				ClusterControlPlaneSecurityGroup: "sg-cluster-sec-group",
			},
			ResourceSpec: corev1alpha1.ResourceSpec{
				ReclaimPolicy: corev1alpha1.ReclaimRetain,
			},
		},
	}
	g := NewGomegaWithT(t)

	// Test Create
	fetched := &EKSCluster{}
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())

	// Test create w/invalid region
	badRegion := created.DeepCopy()
	badRegion.Spec.Region = "bad-region"
	g.Expect(c.Create(ctx, badRegion)).To(MatchError(ContainSubstring("spec.region in body should be one of [us-west-2 us-east-1 eu-west-1]")))

	// Test create w/invalid instance type
	badInstanceType := created.DeepCopy()
	badInstanceType.Spec.WorkerNodes.NodeInstanceType = "xs-bad-type"
	g.Expect(c.Create(ctx, badInstanceType)).To(MatchError(ContainSubstring("spec.workerNodes.nodeInstanceType in body should be one of [t2.small")))
}

func TestNewEKSClusterSpec(t *testing.T) {
	g := NewGomegaWithT(t)

	m := make(map[string]string)
	exp := &EKSClusterSpec{
		ResourceSpec: corev1alpha1.ResourceSpec{
			ReclaimPolicy: corev1alpha1.ReclaimRetain,
		},
	}
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val := "test-region"
	m["region"] = val
	exp.Region = EKSRegion(val)
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "test-arm"
	m["roleARN"] = val
	exp.RoleARN = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "test-vpc"
	m["vpcId"] = val
	exp.VpcID = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "test-subnet-1-id,test-subnet-2-id"
	m["subnetIds"] = val
	exp.SubnetIds = append(exp.SubnetIds, strings.Split(val, ",")...)
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "test-sg-1-id,test-sg-2-id"
	m["securityGroupIds"] = val
	exp.SecurityGroupIds = append(exp.SecurityGroupIds, strings.Split(val, ",")...)
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "1.10.1"
	m["clusterVersion"] = val
	exp.ClusterVersion = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "key-name-test"
	m["workerKeyName"] = val
	exp.WorkerNodes.KeyName = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "test-node-image-id"
	m["workerNodeImageId"] = val
	exp.WorkerNodes.NodeImageID = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "test-node-instance-type"
	m["workerNodeInstanceType"] = val
	exp.WorkerNodes.NodeInstanceType = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	minSize := 5
	val = strconv.Itoa(minSize)
	m["workerNodeAutoScalingGroupMinSize"] = ""
	exp.WorkerNodes.NodeAutoScalingGroupMinSize = nil
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))
	m["workerNodeAutoScalingGroupMinSize"] = val
	exp.WorkerNodes.NodeAutoScalingGroupMinSize = &minSize
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	maxSize := 10
	val = strconv.Itoa(maxSize)
	m["workerNodeAutoScalingGroupMaxSize"] = ""
	exp.WorkerNodes.NodeAutoScalingGroupMaxSize = nil
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))
	m["workerNodeAutoScalingGroupMaxSize"] = val
	exp.WorkerNodes.NodeAutoScalingGroupMaxSize = &maxSize
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	volSize := 20
	val = strconv.Itoa(volSize)
	m["workerNodeVolumeSize"] = ""
	exp.WorkerNodes.NodeVolumeSize = nil
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))
	m["workerNodeVolumeSize"] = val
	exp.WorkerNodes.NodeVolumeSize = &volSize
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "test-bootstrap-args"
	m["workerBootstrapArguments"] = val
	exp.WorkerNodes.BootstrapArguments = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "test-node-group-name"
	m["workerNodeGroupName"] = val
	exp.WorkerNodes.NodeGroupName = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))

	val = "cp-security-group"
	m["workerClusterControlPlaneSecurityGroup"] = val
	exp.WorkerNodes.ClusterControlPlaneSecurityGroup = val
	g.Expect(NewEKSClusterSpec(m)).To(Equal(exp))
}
