/*
Copyright 2018 The Crossplane Authors.

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

// Package azure contains Kubernetes API groups for Azure cloud provider.
package azure

import (
	"testing"

	cache "github.com/crossplaneio/crossplane/pkg/apis/azure/cache/v1alpha1"
	compute "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	database "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	storage "github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAddToScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Errorf("AddToScheme() error = %v", err)
	}
	gvs := []schema.GroupVersion{
		v1alpha1.SchemeGroupVersion,
		cache.SchemeGroupVersion,
		compute.SchemeGroupVersion,
		database.SchemeGroupVersion,
		storage.SchemeGroupVersion,
	}
	for _, gv := range gvs {
		if !s.IsVersionRegistered(gv) {
			t.Errorf("AddToScheme() %v should be registered", gv)
		}
	}
}
