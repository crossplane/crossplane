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

// Package gcp contains Kubernetes API for GCP cloud provider.
package apis

import (
	"testing"

	cachev1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/cache/v1alpha1"
	computev1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	databasev1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/database/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/storage/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAddToScheme(t *testing.T) {
	s := runtime.NewScheme()
	if err := AddToScheme(s); err != nil {
		t.Errorf("AddToScheme() error = %v", err)
	}
	gvs := []schema.GroupVersion{
		gcpv1alpha1.SchemeGroupVersion,
		cachev1alpha1.SchemeGroupVersion,
		computev1alpha1.SchemeGroupVersion,
		databasev1alpha1.SchemeGroupVersion,
		storagev1alpha1.SchemeGroupVersion,
	}
	for _, gv := range gvs {
		if !s.IsVersionRegistered(gv) {
			t.Errorf("AddToScheme() %v should be registered", gv)
		}
	}
}
