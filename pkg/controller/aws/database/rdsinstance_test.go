/*
Copyright 2018 The Conductor Authors.

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

package database

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	"github.com/upbound/conductor/pkg/clients/aws/rds"
	"github.com/upbound/conductor/pkg/controller/core/provider"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// TestReconcile - Missing Provider
func TestReconcileMissingProvider(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr.manager, g))

	// Provider (define, but not create)
	p := testProvider(testSecret([]byte("testdata")))

	// Create RDS Instance
	i, err := mgr.createInstance(testInstance(p))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteInstance(i)

	// Initial Loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
	ri, err := mgr.getInstance()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ri).NotTo(BeNil())
	g.Expect(ri.Status.Conditions).NotTo(BeEmpty())
	c := ri.Status.GetCondition(databasev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorFetchingAwsProvider))

	// Assert Requeue
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
}

// TestReconcile - Invalid Provider
func TestReconcileInvalidProvider(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr.manager, g))

	// Create Provider secret
	s, err := mgr.createSecret(testSecret([]byte("testdata")))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create Provider with invalid state
	p := testProvider(s)
	provider.SetInvalid(&p.Status, "test-reason", "")
	p, err = mgr.createProvider(p)
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Create RDS Instance
	i, err := mgr.createInstance(testInstance(p))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteInstance(i)

	// Initial Loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
	ri, err := mgr.getInstance()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ri).NotTo(BeNil())
	g.Expect(ri.Status.Conditions).NotTo(BeEmpty())
	c := ri.Status.GetCondition(databasev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorProviderStatusInvalid))

	// Assert Requeue
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
	g.Expect(mgr.deleteSecret(s)).NotTo(HaveOccurred())
}

func TestReconcileRDSClientError(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr.manager, g))

	// Create Provider secret
	s, err := mgr.createSecret(testSecret([]byte("testdata")))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create Provider
	p, err := mgr.createProvider(testProvider(s))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Create RDS Instance
	i, err := mgr.createInstance(testInstance(p))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteInstance(i)

	// RDS Client, returns error on client creation
	RDSService = func(p *awsv1alpha1.Provider, k kubernetes.Interface) (rds.Service, error) {
		return nil, fmt.Errorf("test-error")
	}

	// Initial Loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
	ri, err := mgr.getInstance()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ri).NotTo(BeNil())
	g.Expect(ri.Status.Conditions).NotTo(BeEmpty())
	c := ri.Status.GetCondition(databasev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorRDSClient))

	// Assert Requeue
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
}

func TestReconcileGetInstanceError(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr.manager, g))

	// Create Provider secret
	s, err := mgr.createSecret(testSecret([]byte("testdata")))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create Provider
	p, err := mgr.createProvider(testProvider(s))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Create RDS Instance
	i, err := mgr.createInstance(testInstance(p))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteInstance(i)

	// Mock RDS Client
	RDSService = func(p *awsv1alpha1.Provider, k kubernetes.Interface) (rds.Service, error) {
		m := &rds.MockClient{}
		// return error on get instance
		m.MockGetInstance = func(name string) (*rds.Instance, error) {
			return nil, fmt.Errorf("test-get-instance-error")
		}
		return m, nil
	}

	// Initial Loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
	ri, err := mgr.getInstance()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ri).NotTo(BeNil())
	g.Expect(ri.Status.Conditions).NotTo(BeEmpty())
	c := ri.Status.GetCondition(databasev1alpha1.Failed)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.Reason).To(Equal(errorGetDbInstance))

	// Assert Requeue
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
}

func TestReconcile(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr.manager, g))

	// Create Provider secret
	s, err := mgr.createSecret(testSecret([]byte("testdata")))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create Provider
	p, err := mgr.createProvider(testProvider(s))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Create instance
	i, err := mgr.createInstance(testInstance(p))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteInstance(i)

	// Mock RDS Client
	var createdPassword string
	mi := &rds.Instance{
		ARN:    "test-arn",
		Status: rds.DBInstanceStatusCreating.String(), // to avoid requeue
	}

	m := &rds.MockClient{}
	m.MockGetInstance = func(name string) (*rds.Instance, error) {
		if len(createdPassword) > 0 {
			return mi, nil
		}
		return nil, nil
	}
	m.MockCreateInstance = func(name, password string, spec *databasev1alpha1.RDSInstanceSpec) (*rds.Instance, error) {
		createdPassword = password
		mi.Name = name
		return mi, nil
	}
	RDSService = func(p *awsv1alpha1.Provider, k kubernetes.Interface) (rds.Service, error) {
		return m, nil
	}

	// Initial Loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
	ri, err := mgr.getInstance()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ri).NotTo(BeNil())
	g.Expect(ri.Status.Conditions).NotTo(BeEmpty())
	// assert creating condition
	c := ri.Status.GetCondition(databasev1alpha1.Creating)
	g.Expect(c).NotTo(BeNil())
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	// assert connection secret
	cs, err := mgr.getSecret(ri.Spec.ConnectionSecretRef.Name)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cs.Data[connectionSecretUsernameKey]).To(Equal([]byte(i.Spec.MasterUsername)))
	g.Expect(cs.Data[connectionSecretPasswordKey]).To(Equal([]byte(createdPassword)))
	g.Expect(cs.Data[connectionSecretEndpointKey]).To(BeNil())

	// Set endpoint and update status to running
	mi.Endpoint = "Test Endpoint"
	mi.Status = rds.DBInstanceStatusAvailable.String()
	m.MockGetInstance = func(name string) (*rds.Instance, error) {
		return mi, nil
	}

	// wait for running state
	c = ri.Status.GetCondition(databasev1alpha1.Running)
	for c == nil || c.Status != corev1.ConditionTrue {
		g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
		ri, err = mgr.getInstance()
		g.Expect(err).NotTo(HaveOccurred())
		c = ri.Status.GetCondition(databasev1alpha1.Running)
		t.Logf("conditions: %v", ri.Status.Conditions)
	}

	// wait for endpoint value in secret
	for string(cs.Data[connectionSecretEndpointKey]) != mi.Endpoint {
		g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
		cs, err = mgr.getSecret(ri.Spec.ConnectionSecretRef.Name)
		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("waiting: %s", cs.Data[connectionSecretEndpointKey])
	}

}

func TestApplyConnectionSecret(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr.manager, g))

	// Create new secret
	s := testSecret([]byte("testdata"))
	defer mgr.deleteSecret(s)
	cs, err := mgr.reconciler.ApplyConnectionSecret(s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cs.Data).To(Equal(s.Data))

	// Create another secret with the same name but different data
	ns := testSecret([]byte("testdata-new"))
	cs, err = mgr.reconciler.ApplyConnectionSecret(ns)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cs.Data).To(Equal(ns.Data))
}
