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
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/upbound/conductor/pkg/apis/aws"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	"github.com/upbound/conductor/pkg/clients/aws/rds"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	timeout           = 5 * time.Minute
	instanceName      = "foo"
	instanceNamespace = "default"
	dbMasterUserName  = "testuser"
	dbEngine          = "mysql"
	dbClass           = "db.t2.small"
	dbSize            = int64(10)
)

var (
	ctx             = context.TODO()
	awsCredsFile    = flag.String("aws-creds", "", "run integration tests that require .aws/credentials")
	cfg             *rest.Config
	expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: instanceName, Namespace: instanceNamespace}}
)

func init() {
	flag.Parse()
}

type TestManager struct {
	manager     manager.Manager
	requests    chan reconcile.Request
	reconciler  Reconciler
	recFunction reconcile.Reconciler
}

func NewTestManager() (*TestManager, error) {
	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		return nil, err
	}

	r := Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(recorderName),
	}

	recFn, requests := SetupTestReconcile(&r)
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
	return tm.reconciler.kubeclient.CoreV1().Secrets(instanceNamespace).Get(name, metav1.GetOptions{})
}

func (tm *TestManager) deleteSecret(s *corev1.Secret) error {
	return tm.reconciler.kubeclient.CoreV1().Secrets(s.Namespace).Delete(s.Name, &metav1.DeleteOptions{})
}

func (tm *TestManager) createProvider(p *awsv1alpha1.Provider) (*awsv1alpha1.Provider, error) {
	return p, tm.reconciler.Client.Create(ctx, p)
}

func (tm *TestManager) deleteProvider(p *awsv1alpha1.Provider) error {
	return tm.reconciler.Client.Delete(ctx, p)
}

func (tm *TestManager) createInstance(i *databasev1alpha1.RDSInstance) (*databasev1alpha1.RDSInstance, error) {
	return i, tm.reconciler.Client.Create(context.TODO(), i)
}

func (tm *TestManager) getInstance() (*databasev1alpha1.RDSInstance, error) {
	i := &databasev1alpha1.RDSInstance{}
	return i, tm.reconciler.Client.Get(ctx, expectedRequest.NamespacedName, i)
}

func (tm *TestManager) deleteInstance(i *databasev1alpha1.RDSInstance) error {
	return tm.reconciler.Client.Delete(context.TODO(), i)
}

func TestMain(m *testing.M) {

	t := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "cluster", "charts", "conductor", "crds", "aws", "database", "v1alpha1"),
			filepath.Join("..", "..", "..", "..", "cluster", "charts", "conductor", "crds", "aws", "v1alpha1"),
		},
	}
	aws.AddToScheme(scheme.Scheme)

	var err error
	if cfg, err = t.Start(); err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	t.Stop()
	os.Exit(code)
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

func TSecret(data []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-bar",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": data,
		},
	}
}

func TProvider(s *corev1.Secret) *awsv1alpha1.Provider {
	return &awsv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: s.Namespace,
		},
		Spec: awsv1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: s.Name},
				Key:                  "credentials",
			},
			Region: "us-east-1",
		},
	}
}

func TInstance(p *awsv1alpha1.Provider) *databasev1alpha1.RDSInstance {
	return &databasev1alpha1.RDSInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RDSInstance",
			APIVersion: "database.aws.conductor.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: instanceNamespace,
		},
		Spec: databasev1alpha1.RDSInstanceSpec{
			MasterUsername: dbMasterUserName,
			Engine:         dbEngine,
			Class:          dbClass,
			Size:           dbSize,
			ProviderRef: corev1.LocalObjectReference{
				Name: p.Name,
			},
			ConnectionSecretRef: corev1.LocalObjectReference{
				Name: p.Name,
			},
		},
	}
}

type MockRDS struct {
	MockGetInstance    func(string) (*rds.Instance, error)
	MockCreateInstance func(name, password string, spec *databasev1alpha1.RDSInstanceSpec) (*rds.Instance, error)
	MockDeleteInstance func(name string) (*rds.Instance, error)
}

// GetInstance finds RDS Instance by name
func (m *MockRDS) GetInstance(name string) (*rds.Instance, error) {
	return m.MockGetInstance(name)
}

// CreateInstance creates RDS Instance with provided Specification
func (m *MockRDS) CreateInstance(name, password string, spec *databasev1alpha1.RDSInstanceSpec) (*rds.Instance, error) {
	return m.MockCreateInstance(name, password, spec)
}

// DeleteInstance deletes RDS Instance
func (m *MockRDS) DeleteInstance(name string) (*rds.Instance, error) {
	return m.MockDeleteInstance(name)
}
