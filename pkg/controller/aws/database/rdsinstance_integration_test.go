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
	"io/ioutil"
	"testing"

	. "github.com/onsi/gomega"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// TestReconcileWithCreds - run reconciliation loop with actual aws credentials (if provided, otherwise - skipped)
func TestReconcileWithCreds(t *testing.T) {
	g := NewGomegaWithT(t)

	if *awsCredsFile == "" {
		t.Skip()
	}
	data, err := ioutil.ReadFile(*awsCredsFile)
	g.Expect(err).NotTo(HaveOccurred())

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr.manager, g))

	// Create Provider secret
	s, err := mgr.createSecret(TSecret(data))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create Provider
	p, err := mgr.createProvider(TProvider(s))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Create RDS Instance
	i, err := mgr.createInstance(TInstance(p))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteInstance(i)

	// Initial Loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))
	ri, err := mgr.getInstance()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ri).NotTo(BeNil())

	// Assert CRD
	status := ri.Status
	g.Expect(status.InstanceName).NotTo(BeEmpty())
	g.Expect(status.Conditions).NotTo(BeEmpty())
	condition := status.GetCondition(databasev1alpha1.Creating)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))

	// Assert using rds client
	rds, err := RDSService(p, mgr.reconciler.kubeclient)
	g.Expect(err).NotTo(HaveOccurred())
	db, err := rds.GetInstance(status.InstanceName)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(db).NotTo(BeNil())

	// Delete Instance
	mgr.deleteInstance(i)
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Cleanup
	db, err = rds.DeleteInstance(ri.Status.InstanceName)
	g.Expect(err).NotTo(HaveOccurred())
}
