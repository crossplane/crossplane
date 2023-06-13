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
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	"github.com/crossplane/crossplane/test/e2e/funcs"
)

const namespace = "crossplane-system"

// The test environment, shared by all E2E test functions.
var environment env.Environment

func TestMain(m *testing.M) {
	clusterName := envconf.RandomName("crossplane-e2e", 32)
	environment, _ = env.NewFromFlags()
	environment.Setup(funcs.SetupControlPlane(clusterName, namespace))
	environment.Finish(funcs.DestroyControlPlane(clusterName))
	os.Exit(environment.Run(m))
}
