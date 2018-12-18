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

package aks

import (
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2018-03-31/containerservice"
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/Azure/go-autorest/autorest/to"
	azurecomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	. "github.com/onsi/gomega"
)

const (
	TestAssetAzureCredsFile = "AZURE_CREDS_FILE"
)

func TestNewAKSClient(t *testing.T) {
	g := NewGomegaWithT(t)

	file := os.Getenv(TestAssetAzureCredsFile)
	if file == "" {
		t.Skipf("test asset %s environment variable is not provided", TestAssetAzureCredsFile)
	}

	data, err := ioutil.ReadFile(file)
	g.Expect(err).NotTo(HaveOccurred())

	config, err := azure.NewClientCredentialsConfig(data)
	g.Expect(err).NotTo(HaveOccurred())

	client, err := NewAKSClient(config)
	g.Expect(err).NotTo(HaveOccurred())

	spec := azurecomputev1alpha1.AKSClusterSpec{
		ResourceGroupName: "group-westus-1",
		Location:          "Central US",
		NodeCount:         to.IntPtr(1),
		DNSNamePrefix:     "crossplane",
		Version:           "1.11.4",
		DisableRBAC:       false,
		NodeVMSize:        "Standard_B2s",
	}

	suffx, err := util.GenerateHex(3)
	g.Expect(err).NotTo(HaveOccurred())
	name := "test-" + suffx

	cluster, err := client.Create(name, spec)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(to.String(cluster.Name)).To(Equal(name))
	g.Expect(to.String(cluster.KubernetesVersion)).To(Equal(spec.Version))

	_, err = waitFor(client, spec.ResourceGroupName, name, azurecomputev1alpha1.ClusterStateSucceeded, time.Second*15, time.Minute*10)
	g.Expect(err).NotTo(HaveOccurred())

	kubeconfig, err := client.ListCredentials(spec.ResourceGroupName, name)
	g.Expect(err).NotTo(HaveOccurred())
	t.Logf(util.StringValue(kubeconfig.Name))
	t.Logf("%s", *kubeconfig.Value)

	g.Expect(client.Delete(spec.ResourceGroupName, name)).Should(Succeed())
	t.Logf("cluster name: %s\n", name)
}

func TestTrimName(t *testing.T) {
	g := NewGomegaWithT(t)

	cases := []struct {
		input    string
		expected string
	}{
		// name is OK, should not be modified
		{"foo", "foo"},
		// name too long, should be truncated down to max length
		{"aks-ca60851e-168b-4cee-b3e3-3cc4bb031103", "aks-ca60851e-168b-4cee-b3e3-3cc"},
		// truncated length would result in a trailing hyphen, it should also be removed
		{"aks-ca60851e-168b-4cee-b3e3-3c--", "aks-ca60851e-168b-4cee-b3e3-3c"},
	}

	for _, tt := range cases {
		actual := trimName(tt.input)
		g.Expect(actual).To(Equal(tt.expected))
	}
}

// waitFor cluster to be in a provided state
func waitFor(c Client, group, name, status string, interval, duration time.Duration) (containerservice.ManagedCluster, error) {
	var cluster containerservice.ManagedCluster
	return cluster, wait.PollImmediate(interval, duration, func() (bool, error) {
		c, err := c.Get(group, name)
		if err != nil {
			return false, err
		}

		clusterStatus := util.StringValue(c.ProvisioningState)
		if clusterStatus == status {
			cluster = c
			return true, nil
		}
		log.Printf("status: %s, waiting ...", clusterStatus)
		return false, nil
	})
}
