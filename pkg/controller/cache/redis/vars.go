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

package redis

import (
	"context"

	azurecachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/cache/v1alpha1"
	gcpcachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/cache/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
)

var (
	ctx = context.Background()

	// map of supported resource handlers
	handlers = map[string]corecontroller.ResourceHandler{
		azurecachev1alpha1.RedisKindAPIVersion:                  &RedisHandler{},
		gcpcachev1alpha1.CloudMemorystoreInstanceKindAPIVersion: &CloudMemorystoreInstanceHandler{},
	}
)
