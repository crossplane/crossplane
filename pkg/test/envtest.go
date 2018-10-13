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

package test

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	DEFAULT_NAMESPACE               = "default"
	TEST_ASSET_USE_EXISTING_CLUSTER = "USE_EXISTING_CLUSTER"
)

var (
	_, b, _, _ = runtime.Caller(0)
	crds       = filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(b))), "cluster", "charts", "conductor", "crds")
)

// TestEnv - wrapper for controller-runtime envtest with additional functionality
type TestEnv struct {
	envtest.Environment

	namespace string
	cfg       *rest.Config
}

// NewTestEnv - create new test environment instance
func NewTestEnv(namespace string, crds ...string) *TestEnv {
	t := envtest.Environment{
		UseExistingCluster: UseExistingCluster(),
	}
	if !t.UseExistingCluster {
		if crds, err := CheckCRDFiles(crds); err != nil {
			log.Panic(err)
		} else {
			t.CRDDirectoryPaths = crds
		}
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
	if te.namespace != DEFAULT_NAMESPACE {
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
func CheckCRDFiles(crds []string) ([]string, error) {
	var rs []string

	for _, path := range crds {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return nil, err
		}
		files, err := Expand(path)
		if err != nil {
			return nil, err
		}
		rs = append(rs, files...)
	}
	return rs, nil
}

// Expand recursively and returns list of all sub-directories (if any)
func Expand(path string) ([]string, error) {
	var rs []string

	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, err
	}
	if fi.Mode().IsDir() {
		files, err := filepath.Glob(filepath.Join(path, "*"))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			exp, err := Expand(f)
			if err != nil {
				return nil, err
			}
			rs = append(rs, exp...)
		}
		rs = append(rs, path)
	}
	return rs, nil
}

// CRDs path to project crds location
func CRDs() string {
	return crds
}
