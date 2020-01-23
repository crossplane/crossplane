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

package engines

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

// A ResourceEngineRunner is responsible for creating resources for a template stack when a hook is executed.
//
// The inputs are:
// - A hook configuration
// - The object which triggered this hook
// - The source files to create resources from
//
// The outputs should be:
// - Created resources
//
// This interface is still fairly new; in the future we'll want to be able to represent the status of the resources
// which are being created. For example, if the resources are created asynchronously (say by running a Job), the
// output the first time the engine is run may be the status of the Job. The next time, if the job is finished, it may
// be some sort of reference to the objects which have been created.
type ResourceEngineRunner interface {
	CreateConfig(claim *unstructured.Unstructured, hc *v1alpha1.HookConfiguration) (*corev1.ConfigMap, error)

	RunEngine(
		ctx context.Context,
		client client.Client,
		claim *unstructured.Unstructured,
		config *corev1.ConfigMap,
		stackSource string,
		hc *v1alpha1.HookConfiguration,
	) (*unstructured.Unstructured, error)
}
