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

package v1alpha1

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
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
	base := &EKSCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: EKSClusterSpec{
			EKSClusterParameters: EKSClusterParameters{
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
			},
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference: &core.ObjectReference{},
				ReclaimPolicy:     runtimev1alpha1.ReclaimRetain,
			},
		},
	}
	g := NewGomegaWithT(t)

	// Test Create
	fetched := &EKSCluster{}
	created := base.DeepCopy()
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
	badRegion := base.DeepCopy()
	badRegion.Spec.Region = "bad-region"
	g.Expect(c.Create(ctx, badRegion)).To(MatchError(ContainSubstring("spec.region in body should be one of [us-west-2 us-east-1 eu-west-1]")))

	// Test create w/invalid instance type
	badInstanceType := base.DeepCopy()
	badInstanceType.Spec.WorkerNodes.NodeInstanceType = "xs-bad-type"
	g.Expect(c.Create(ctx, badInstanceType)).To(MatchError(ContainSubstring("spec.workerNodes.nodeInstanceType in body should be one of [t2.small")))
}
