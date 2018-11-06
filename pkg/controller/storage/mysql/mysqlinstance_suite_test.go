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

package mysql

import (
	"context"
	"flag"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/apis/storage"
	. "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
	"github.com/upbound/conductor/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	timeout = 5 * time.Second

	namespace = "default"
	name      = "test-mysqlinstance"
)

var (
	cfg             *rest.Config
	expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: namespace}}
)

func init() {
	flag.Parse()
}

func TestMain(m *testing.M) {
	storage.AddToScheme(scheme.Scheme)

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
		recorder:   mgr.GetRecorder(controllerName),
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

func testInstance() *MySQLInstance {
	return &MySQLInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// MockClient controller-runtime client
type MockClient struct {
	client.Client

	MockGet    func(...interface{}) error
	MockUpdate func(...interface{}) error
}

func (mc *MockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return mc.MockGet(ctx, key, obj)
}

func (mc *MockClient) Update(ctx context.Context, obj runtime.Object) error {
	return mc.MockUpdate(ctx, obj)
}

// MockRecorder Kubernetes events recorder
type MockRecorder struct {
	record.EventRecorder
}

// The resulting event will be created in the same namespace as the reference object.
func (mr *MockRecorder) Event(object runtime.Object, eventtype, reason, message string) {}

// Eventf is just like Event, but with Sprintf for the message field.
func (mr *MockRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
}

// PastEventf is just like Eventf, but with an option to specify the event's 'timestamp' field.
func (mr *MockRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
}

// AnnotatedEventf is just like eventf, but with annotations attached
func (mr *MockRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}

type MockResourceHandler struct {
	MockProvision     func(*corev1alpha1.ResourceClass, *MySQLInstance, client.Client) (corev1alpha1.Resource, error)
	MockFind          func(types.NamespacedName, client.Client) (corev1alpha1.Resource, error)
	MockSetBindStatus func(types.NamespacedName, client.Client, bool) error
}

func (mrh *MockResourceHandler) provision(class *corev1alpha1.ResourceClass, instance *MySQLInstance, c client.Client) (corev1alpha1.Resource, error) {
	return mrh.MockProvision(class, instance, c)
}

func (mrh *MockResourceHandler) find(n types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	return mrh.MockFind(n, c)
}

func (mrh *MockResourceHandler) setBindStatus(n types.NamespacedName, c client.Client, s bool) error {
	return mrh.MockSetBindStatus(n, c, s)
}
