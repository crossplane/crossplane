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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// TestComposition tests Crossplane's Composition functionality.
func TestComposition(t *testing.T) {
	t.Parallel()

	// TODO(negz): We're using DeploymentAvailable as a proxy to ensure the
	// Composition validation webhook is ready. We should probably check whether
	// the service has replicas instead.
	setup := funcs.AllOf(
		funcs.DeploymentBecomesAvailableWithin(1*time.Minute, namespace, "crossplane"),
		funcs.ResourcesHaveConditionWithin(1*time.Minute, crdsDir, "*.yaml", funcs.CRDInitialNamesAccepted()),
	)

	// Test that a claim using a very minimal Composition (with no patches,
	// transforms, or functions) will become available when its composed
	// resources do.
	manifests := "test/e2e/manifests/composition/minimal"
	minimal := features.Table{
		{
			Name: "PrerequisitesAreCreated",
			Assessment: funcs.AllOf(
				funcs.CreateResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", apiextensionsv1.WatchingComposite()),
		},
		{
			Name: "ClaimIsCreated",
			Assessment: funcs.AllOf(
				funcs.CreateResources(manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			),
		},
		{
			Name:       "ClaimBecomesAvailable",
			Assessment: funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "claim.yaml", xpv1.Available()),
		},
		{
			Name: "ClaimIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "prerequisites/*.yaml"),
			),
		},
	}

	// Test that a claim using patch-and-transform Composition will become
	// available when its composed resources do, and have a field derived from
	// the patch.
	manifests = "test/e2e/manifests/composition/patch-and-transform"
	pandt := features.Table{
		{
			Name: "PrerequisitesAreCreated",
			Assessment: funcs.AllOf(
				funcs.CreateResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "prerequisites/*.yaml"),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "prerequisites/definition.yaml", apiextensionsv1.WatchingComposite()),
		},
		{},
		{
			Name: "ClaimIsCreated",
			Assessment: funcs.AllOf(
				funcs.CreateResources(manifests, "claim.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "claim.yaml"),
			),
		},
		{
			Name:       "ClaimBecomesAvailable",
			Assessment: funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "claim.yaml", xpv1.Available()),
		},
		{
			Name:       "ClaimHasPatchedField",
			Assessment: funcs.ResourcesHaveFieldValueWithin(2*time.Minute, manifests, "claim.yaml", "status.coolerField", "I'M COOL!"),
		},
		{
			Name: "ClaimIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "claim.yaml"),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "prerequisites/*.yaml"),
			),
		},
	}

	// TODO(negz): Use TestInParallel to test features in parallel. This will
	// require them to avoid sharing state - e.g. to ensure a claim always
	// selects the correct Composition when there are many.
	environment.Test(t,
		minimal.Build("Minimal").Setup(setup).Feature(),
		pandt.Build("PatchAndTransform").Setup(setup).Feature(),
	)
}
