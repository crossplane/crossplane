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
	"context"
	"flag"
	"testing"
	"time"

	"github.com/upbound/conductor/pkg/test"

	. "github.com/onsi/gomega"
	"github.com/upbound/conductor/pkg/apis/aws"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
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
	timeout        = 5 * time.Second
	namespace      = "default"
	instanceName   = "test-db-instance"
	secretName     = "test-secret"
	secretDataKey  = "credentials"
	providerName   = "test-provider"
	providerRegion = "us-east-1"
	masterUserName = "testuser"
	engine         = "mysql"
	class          = "db.t2.small"
	size           = int64(10)
)

var (
	// used for integration tests with real aws credentials
	awsCredsFile = flag.String("aws-creds", "", "run integration tests that require .aws/credentials")
	ctx             = context.TODO()
	cfg             *rest.Config
	expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: instanceName, Namespace: namespace}}
)

func init() {
	flag.Parse()
}

func TestMain(m *testing.M) {
	aws.AddToScheme(scheme.Scheme)

	t := test.NewTestEnv(namespace, test.CRDs())
	cfg = t.Start()
	t.StopAndExit(m.Run())
}

type TestManager struct {
	manager     manager.Manager
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
		manager:     mgr,
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

func (tm *TestManager) createProvider(p *awsv1alpha1.Provider) (*awsv1alpha1.Provider, error) {
	return p, tm.reconciler.Create(ctx, p)
}

func (tm *TestManager) deleteProvider(p *awsv1alpha1.Provider) error {
	return tm.reconciler.Delete(ctx, p)
}

func (tm *TestManager) createInstance(i *databasev1alpha1.RDSInstance) (*databasev1alpha1.RDSInstance, error) {
	return i, tm.reconciler.Create(context.TODO(), i)
}

func (tm *TestManager) getInstance() (*databasev1alpha1.RDSInstance, error) {
	i := &databasev1alpha1.RDSInstance{}
	return i, tm.reconciler.Get(ctx, expectedRequest.NamespacedName, i)
}

func (tm *TestManager) deleteInstance(i *databasev1alpha1.RDSInstance) error {
	return tm.reconciler.Delete(context.TODO(), i)
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

func testInstance(p *awsv1alpha1.Provider) *databasev1alpha1.RDSInstance {
	return &databasev1alpha1.RDSInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: namespace,
		},
		Spec: databasev1alpha1.RDSInstanceSpec{
			MasterUsername: masterUserName,
			Engine:         engine,
			Class:          class,
			Size:           size,
			ProviderRef: corev1.LocalObjectReference{
				Name: p.Name,
			},
			ConnectionSecretRef: corev1.LocalObjectReference{
				Name: p.Name,
			},
		},
	}
}
