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

	"github.com/crossplane/crossplane/test/e2e/funcs"
)

func TestComposition(t *testing.T) {
	t.Parallel()

	// TODO(negz): We're using DeploymentAvailable as a proxy to ensure the
	// Composition validation webhook is ready. We should probably check whether
	// the service has replicas instead.
	setup := funcs.AllOf(
		funcs.DeploymentBecomesAvailableIn(namespace, "crossplane", 1*time.Minute),
		funcs.CrossplaneCRDsBecomeEstablishedIn(1*time.Minute),
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
				funcs.ResourcesCreatedIn(manifests, "prerequisites/*.yaml", 30*time.Second),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.XRDsBecomeEstablishedIn(manifests, "prerequisites/definition.yaml", 1*time.Minute),
		},
		{
			Name: "ClaimIsCreated",
			Assessment: funcs.AllOf(
				funcs.CreateResources(manifests, "claim.yaml"),
				funcs.ResourcesCreatedIn(manifests, "claim.yaml", 30*time.Second),
			),
		},
		{
			Name:       "ClaimBecomesAvailable",
			Assessment: funcs.ResourcesBecomeIn(manifests, "claim.yaml", 2*time.Minute, xpv1.Available()),
		},
		{
			Name: "ClaimIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedIn(manifests, "claim.yaml", 2*time.Minute),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedIn(manifests, "prerequisites/*.yaml", 3*time.Minute),
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
				funcs.ResourcesCreatedIn(manifests, "prerequisites/*.yaml", 30*time.Second),
			),
		},
		{
			Name:       "XRDBecomesEstablished",
			Assessment: funcs.XRDsBecomeEstablishedIn(manifests, "prerequisites/definition.yaml", 1*time.Minute),
		},
		{
			Name: "ClaimIsCreated",
			Assessment: funcs.AllOf(
				funcs.CreateResources(manifests, "claim.yaml"),
				funcs.ResourcesCreatedIn(manifests, "claim.yaml", 30*time.Second),
			),
		},
		{
			Name:       "ClaimBecomesAvailable",
			Assessment: funcs.ResourcesBecomeIn(manifests, "claim.yaml", 2*time.Minute, xpv1.Available()),
		},
		{
			Name:       "ClaimHasPatchedField",
			Assessment: funcs.ResourcesHaveIn(manifests, "claim.yaml", 2*time.Minute, "status.coolerField", "I'M COOL!"),
		},
		{
			Name: "ClaimIsDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "claim.yaml"),
				funcs.ResourcesDeletedIn(manifests, "claim.yaml", 2*time.Minute),
			),
		},
		{
			Name: "PrerequisitesAreDeleted",
			Assessment: funcs.AllOf(
				funcs.DeleteResources(manifests, "prerequisites/*.yaml"),
				funcs.ResourcesDeletedIn(manifests, "prerequisites/*.yaml", 3*time.Minute),
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
