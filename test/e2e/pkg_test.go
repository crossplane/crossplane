/*
Copyright 2022 The Crossplane Authors.

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

package e2e

import (
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/features"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

func TestConfiguration(t *testing.T) {
	t.Parallel()

	// Test that we can install a Configuration from a private repository using
	// a package pull secret.
	manifests := "test/e2e/manifests/pkg/configuration/private"
	private := features.Table{
		{
			Name: "ConfigurationIsCreated",
			Assessment: funcs.AllOf(
				funcs.CreateResources(manifests, "*.yaml"),
				funcs.ResourcesCreatedWithin(1*time.Minute, manifests, "*.yaml"),
			),
		},
		{
			Name:       "ConfigurationIsInstalled",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "configuration.yaml", v1.Healthy(), v1.Active()),
		},
		{
			Name: "ConfigurationIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "*.yaml"),
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "*.yaml"),
			),
		},
	}

	setup := funcs.ReadyToTestWithin(1*time.Minute, namespace)
	environment.Test(t,
		private.Build("PrivateRegistry").
			WithLabel("area", "pkg").
			WithLabel("size", "small").
			Setup(setup).Feature(),
	)
}
