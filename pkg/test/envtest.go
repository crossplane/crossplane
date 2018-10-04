package test

import (
	"log"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	TEST_ASSET_USE_EXISTING_CLUSTER = "USE_EXISTING_CLUSTER"
)

// TestEnv - wrapper for controller-runtime envtest with additional functionality
type TestEnv struct {
	envtest.Environment

	namespace string
	cfg       *rest.Config
}

// NewTestEnv - create new test environment instance
func NewTestEnv(crds []string, namespace string) *TestEnv {
	t := envtest.Environment{
		UseExistingCluster: UseExistingCluster(),
	}
	if !t.UseExistingCluster {
		if err := CheckCRDFiles(crds); err != nil {
			log.Panic(err)
		}
		t.CRDDirectoryPaths = crds
	}
	if len(namespace) == 0 {
		namespace = "default"
	}
	return &TestEnv{
		Environment: t,
		namespace:   namespace,
	}
}

// Start - starts and bootstraps test environment ang returns Kubernetes config instance
func (te *TestEnv) Start() *rest.Config {
	cfg, err := te.Environment.Start()
	if err != nil {
		log.Fatal(err)
	}
	te.cfg = cfg

	// Crate testing namespace for this package if it is not "default"
	if te.namespace != "default" {
		k := kubernetes.NewForConfigOrDie(cfg)
		_, err := k.CoreV1().Namespaces().Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: te.namespace,
			},
		})
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Fatal(err)
		}
	}
	return cfg
}

// Stop - stops test environment performing additional cleanup (if needed)
func (te *TestEnv) Stop() {
	defer te.Environment.Stop()
	if te.namespace != "default" {
		k := kubernetes.NewForConfigOrDie(te.cfg)
		dp := metav1.DeletePropagationForeground
		err := k.CoreV1().Namespaces().Delete(te.namespace, &metav1.DeleteOptions{PropagationPolicy: &dp})
		if err != nil {
			log.Panic(err)
		}
	}
}

// StopAndExit - stops and exists, typically used as a last call in TestMain
func (te *TestEnv) StopAndExit(code int) {
	te.Stop()
	os.Exit(code)
}

// UseExistingCluster - checks if USE_EXISTING_CLUSTER environment variable is set
func UseExistingCluster() bool {
	env, err := strconv.ParseBool(os.Getenv(TEST_ASSET_USE_EXISTING_CLUSTER))
	if err != nil {
		return false
	}
	return env
}

// CheckCRDFiles - validates that all crds files are found.
func CheckCRDFiles(crds []string) error {
	for _, path := range crds {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
