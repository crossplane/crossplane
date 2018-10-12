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
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	. "github.com/onsi/gomega"
	awsapis "github.com/upbound/conductor/pkg/apis/aws"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
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

	namespace      = "default"
	secretName     = "test-secret"
	secretDataKey  = "credentials"
	providerName   = "test-provider"
	providerRegion = "us-east-1"
)

var (
	awsCredsFile    = flag.String("aws-creds", "", "run integration tests that require .aws/credentials")
	ctx             = context.TODO()
	cfg             *rest.Config
	expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: providerName, Namespace: namespace}}
)

func init() {
	flag.Parse()
}

func TestMain(m *testing.M) {
	awsapis.AddToScheme(scheme.Scheme)

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
func StartTestManager(mgr manager.Manager, g *GomegaWithT) chan struct{} {
	stop := make(chan struct{})
	go func() {
		g.Expect(mgr.Start(stop)).NotTo(HaveOccurred())
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
		Validator:  &ConfigurationValidator{},
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

func (tm *TestManager) createProvider(p *awsv1alpha1.Provider) error {
	return tm.reconciler.Create(ctx, p)
}

func (tm *TestManager) getProvider() (*awsv1alpha1.Provider, error) {
	p := &awsv1alpha1.Provider{}
	return p, tm.reconciler.Get(ctx, expectedRequest.NamespacedName, p)
}

func (tm *TestManager) deleteProvider(p *awsv1alpha1.Provider) error {
	return tm.reconciler.Delete(ctx, p)
}

func testSecretData(profile, id, secret string) []byte {
	return []byte(fmt.Sprintf("[%s]\naws_access_key_id = %s\naws_secret_access_key = %s", strings.ToLower(profile), id, secret))
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

func testProvider(s *corev1.Secret) *awsv1alpha1.Provider {
	return &awsv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: s.Namespace,
		},
		Spec: awsv1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: s.Name},
				Key:                  secretDataKey,
			},
			Region: providerRegion,
		},
	}
}

// MockValidator - validates credentials
type MockValidator struct{}

// Validate - never fails
func (mv *MockValidator) Validate(config *aws.Config) error {
	return nil
}
