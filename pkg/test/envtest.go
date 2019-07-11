/*
Copyright 2019 The Crossplane Authors.

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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	defaultNamespace            = "default"
	testAssetUseExistingCluster = "USE_EXISTING_CLUSTER"
)

var (
	_, b, _, _ = runtime.Caller(0)
	crds       = filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(b))), "cluster", "charts", "crossplane", "crds")
)

// Env - wrapper for controller-runtime envtest with additional functionality
type Env struct {
	envtest.Environment

	namespace string
	cfg       *rest.Config
}

// NewEnv - create new test environment instance
func NewEnv(namespace string, builder apiruntime.SchemeBuilder, crds ...string) *Env {
	if err := builder.AddToScheme(scheme.Scheme); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	t := envtest.Environment{
		UseExistingCluster: UseExistingCluster(),
	}
	if !t.UseExistingCluster {
		if crds, err := CheckCRDFiles(crds); err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			t.CRDDirectoryPaths = crds
		}
	}
	if len(namespace) == 0 {
		namespace = "default"
	}
	return &Env{
		Environment: t,
		namespace:   namespace,
	}
}

// Start - starts and bootstraps test environment ang returns Kubernetes config instance
func (te *Env) Start() *rest.Config {
	cfg, err := te.Environment.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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
			fmt.Println(err)
			os.Exit(1)
		}
	}

	return cfg
}

// StartClient - starts test environment and returns instance of go-client
func (te *Env) StartClient() client.Client {
	cfg := te.Start()

	clnt, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return clnt
}

// Stop - stops test environment performing additional cleanup (if needed)
func (te *Env) Stop() {
	defer func() {
		if err := te.Environment.Stop(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()
	if te.namespace != defaultNamespace {
		k := kubernetes.NewForConfigOrDie(te.cfg)
		dp := metav1.DeletePropagationForeground
		err := k.CoreV1().Namespaces().Delete(te.namespace, &metav1.DeleteOptions{PropagationPolicy: &dp})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

// StopAndExit - stops and exists, typically used as a last call in TestMain
func (te *Env) StopAndExit(code int) {
	te.Stop()
	os.Exit(code)
}

// UseExistingCluster - checks if USE_EXISTING_CLUSTER environment variable is set
func UseExistingCluster() bool {
	env, err := strconv.ParseBool(os.Getenv(testAssetUseExistingCluster))
	if err != nil {
		return false
	}
	return env
}

// CheckCRDFiles - validates that all crds files are found.
func CheckCRDFiles(crds []string) ([]string, error) {
	var rs []string // nolint:prealloc

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
