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

	"github.com/crossplane/crossplane/test/e2e/funcs"
)

func TestCrossplane(t *testing.T) {
	t.Parallel()

	install := features.Table{
		{
			Name:       "CoreDeploymentBecomesAvailable",
			Assessment: funcs.DeploymentBecomesAvailableIn(namespace, "crossplane", 1*time.Minute),
		},
		{
			Name:       "RBACManagerDeploymentBecomesAvailable",
			Assessment: funcs.DeploymentBecomesAvailableIn(namespace, "crossplane-rbac-manager", 1*time.Minute),
		},
		{
			Name:       "CoreCRDsBecomeEstablished",
			Assessment: funcs.CrossplaneCRDsBecomeEstablishedIn(1 * time.Minute),
		},
	}

	environment.Test(t, install.Build("InstallCrossplane").Feature())
}
