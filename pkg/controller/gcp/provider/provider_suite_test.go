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
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/upbound/conductor/pkg/apis/gcp"
	gcpv1alpha1 "github.com/upbound/conductor/pkg/apis/gcp/v1alpha1"
	"github.com/upbound/conductor/pkg/test"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	timeout = 5 * time.Second

	namespace       = "default"
	secretName      = "test-secret"
	secretDataKey   = "credentials"
	providerName    = "test-provider"
	providerProject = "test-project"
)

var (
	ctx             = context.TODO()
	cfg             *rest.Config
	expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: providerName, Namespace: namespace}}
)

func TestMain(m *testing.M) {
	gcp.AddToScheme(scheme.Scheme)

	t := test.NewTestEnv(namespace, test.CRDs())
	cfg = t.Start()
	t.StopAndExit(m.Run())

}

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func SetupTestReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(req)
		requests <- req
		return result, err
	})
	return fn, requests
}

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager, g *gomega.GomegaWithT) chan struct{} {
	stop := make(chan struct{})
	go func() {
		g.Expect(mgr.Start(stop)).NotTo(gomega.HaveOccurred())
	}()
	return stop
}

type TestManager struct {
	manager.Manager
	requests    chan reconcile.Request
	reconciler  *Reconciler
	recFunction reconcile.Reconciler
}

func NewTestManager() (*TestManager, error) {
	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		return nil, err
	}

	r := &Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(recorderName),
	}

	recFn, requests := SetupTestReconcile(r)
	if err = add(mgr, recFn); err != nil {
		return nil, err
	}

	return &TestManager{
		Manager:     mgr,
		reconciler:  r,
		recFunction: recFn,
		requests:    requests,
	}, nil
}

func (tm *TestManager) createSecret(s *corev1.Secret) (*corev1.Secret, error) {
	return tm.reconciler.kubeclient.CoreV1().Secrets(s.Namespace).Create(s)
}

func (tm *TestManager) getSecret(name string) (*corev1.Secret, error) {
	return tm.reconciler.kubeclient.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
}

func (tm *TestManager) deleteSecret(s *corev1.Secret) error {
	return tm.reconciler.kubeclient.CoreV1().Secrets(s.Namespace).Delete(s.Name, &metav1.DeleteOptions{})
}

func (tm *TestManager) createProvider(p *gcpv1alpha1.Provider) error {
	return tm.reconciler.Create(ctx, p)
}

func (tm *TestManager) getProvider() (*gcpv1alpha1.Provider, error) {
	p := &gcpv1alpha1.Provider{}
	return p, tm.reconciler.Get(ctx, expectedRequest.NamespacedName, p)
}

func (tm *TestManager) deleteProvider(p *gcpv1alpha1.Provider) error {
	return tm.reconciler.Delete(ctx, p)
}

func testSecret(data []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretDataKey: data,
		},
	}
}

func testProvider(s *corev1.Secret) *gcpv1alpha1.Provider {
	return &gcpv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: s.Namespace,
		},
		Spec: gcpv1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  secretDataKey,
			},
			ProjectID: providerProject,
		},
	}
}

// MockValidator - validates credentials
type MockValidator struct{}

// Validate - always valid
func (mv *MockValidator) Validate(k kubernetes.Interface, p *gcpv1alpha1.Provider) error {
	return nil
}
