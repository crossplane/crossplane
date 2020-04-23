/*
Copyright 2020 The Crossplane Authors.

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

package definition

import (
	"context"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Definer has properties that are used to create a CustomResourceDefinition.
type Definer interface {
	resource.Object

	GetDefinedGroupVersionKind() schema.GroupVersionKind
	GenerateCRD() (*v1beta1.CustomResourceDefinition, error)
	GetConnectionSecretKeys() []string
}

// Client does the necessary operations to manage the lifecycle of a CustomResourceDefinition.
type Client interface {
	Get(context.Context, Definer) (*v1beta1.CustomResourceDefinition, error)
	DeleteCustomResources(context.Context, Definer) (bool, error)
	Delete(context.Context, Definer) error
	Apply(context.Context, Definer) error
}
