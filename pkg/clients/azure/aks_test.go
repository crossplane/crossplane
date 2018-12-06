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

package azure

import (
	"context"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	. "github.com/onsi/gomega"
)

func TestNewAKSClient(t *testing.T) {
	g := NewGomegaWithT(t)
	ctx := context.Background()

	suffix, err := util.GenerateHex(3)
	g.Expect(err).NotTo(HaveOccurred())
	name := "test-" + suffix
	url := "https://" + name + ".crossplane.io"

	// load local creds or skip the test
	data := AzureCredsDataOrSkip(t)
	config, err := NewClientCredentialsConfig(data)
	g.Expect(err).NotTo(HaveOccurred())

	// Create AD application
	appClient, err := NewApplicationClient(config)
	g.Expect(err).NotTo(HaveOccurred())
	password := NewPasswordCredential(name)

	app, err := appClient.CreateApplication(ctx, name, url, password)
	g.Expect(err).NotTo(HaveOccurred())
	defer appClient.DeleteApplication(ctx, *app.ObjectID)

	// Create Service Principal
	spClient, err := NewServicePrincipalClient(config)
	g.Expect(err).NotTo(HaveOccurred())
	_, err = spClient.CreateServicePrincipal(ctx, *app.AppID)
	g.Expect(err).NotTo(HaveOccurred())

	// Create Cluster
	aksClient, err := NewAKSClient(config)
	g.Expect(err).NotTo(HaveOccurred())

	spec := v1alpha1.AKSClusterSpec{
		ResourceGroupName: "group-westus-1",
		Location:          "Central US",
		NodeCount:         to.IntPtr(1),
		DNSNamePrefix:     "crossplane",
		Version:           "1.11.4",
		DisableRBAC:       false,
		NodeVMSize:        "Standard_B2s",
	}

	future, err := aksClient.CreateCluster(ctx, name, *app.AppID, *password.Value, spec)
	g.Expect(err).NotTo(HaveOccurred())

	cluster, err := aksClient.GetCluster(ctx, spec.ResourceGroupName, name)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(to.String(cluster.Name)).To(Equal(name))
	g.Expect(to.String(cluster.KubernetesVersion)).To(Equal(spec.Version))
	g.Expect(*cluster.ProvisioningState).To(Equal(v1alpha1.ClusterStateCreating))

	done, err := aksClient.DoneWithContext(ctx, future)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(done).To(BeFalse())

	g.Expect(aksClient.WaitForCompletion(ctx, future)).To(Succeed())

	done, err = aksClient.DoneWithContext(ctx, future)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(done).To(BeTrue())

	kubeconfig, err := aksClient.ListCredentials(spec.ResourceGroupName, name)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(kubeconfig).NotTo(BeNil())

	g.Expect(aksClient.DeleteCluster(ctx, spec.ResourceGroupName, name)).Should(Succeed())
}
