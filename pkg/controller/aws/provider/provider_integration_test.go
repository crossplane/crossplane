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

package provider

import (
	"io/ioutil"
	"testing"

	. "github.com/onsi/gomega"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/controller/core/provider"
	corev1 "k8s.io/api/core/v1"
)

// TestReconcileValid - - run reconciliation loop with actual aws credentials (if provided, otherwise - skipped)
func TestReconcileWithCreds(t *testing.T) {
	g := NewGomegaWithT(t)

	// retrieve aws credentials
	if *awsCredsFile == "" {
		t.Skip()
	}
	data, err := ioutil.ReadFile(*awsCredsFile)
	g.Expect(err).NotTo(HaveOccurred())

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	s, err := mgr.createSecret(testSecret(data))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create provider
	p := testProvider(s)
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Reconcile loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	rp, err := mgr.getProvider()
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(rp.Status, corev1alpha1.Invalid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(rp.Status, corev1alpha1.Valid)
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
}
