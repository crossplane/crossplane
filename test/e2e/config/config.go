/*
Copyright 2023 The Crossplane Authors.
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

// Package config contains the e2e test configuration.
package config

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

// LabelTestSuite is used to define the suite each test should be part of.
const LabelTestSuite = "test-suite"

// TestSuiteDefault is the default suite's key and value for LabelTestSuite.
const TestSuiteDefault = "base"

const testSuiteFlag = "test-suite"

// E2EConfig is these e2e test configuration.
type E2EConfig struct {
	createKindCluster     *bool
	destroyKindCluster    *bool
	preinstallCrossplane  *bool
	loadImagesKindCluster *bool
	kindClusterName       *string

	selectedTestSuite *selectedTestSuite

	specificTestSelected *bool
	suites               map[string]testSuite
}

type selectedTestSuite struct {
	name string
	set  bool
}

func (s *selectedTestSuite) String() string {
	if !s.set {
		return TestSuiteDefault
	}
	return s.name
}

func (s *selectedTestSuite) Set(v string) error {
	fmt.Printf("Setting test suite to %s\n", v)
	s.name = v
	s.set = true
	return nil
}

// testSuite is a test suite, allows to specify a set of options to be used
// for a suite, by default all options will include the base suite
// "SuiteDefault".
type testSuite struct {
	excludeBaseSuite     bool
	helmInstallOpts      []helm.Option
	additionalSetupFuncs []conditionalSetupFunc
	labelsToSelect       features.Labels
}

type conditionalSetupFunc struct {
	condition func() bool
	f         []env.Func
}

// NewFromFlags creates a new e2e test configuration, setting up the flags, but
// not parsing them yet, which is left to the caller to do.
func NewFromFlags() E2EConfig {
	c := E2EConfig{
		suites: map[string]testSuite{},
	}
	c.kindClusterName = flag.String("kind-cluster-name", "", "name of the kind cluster to use")
	c.createKindCluster = flag.Bool("create-kind-cluster", true, "create a kind cluster (and deploy Crossplane) before running tests, if the cluster does not already exist with the same name")
	c.destroyKindCluster = flag.Bool("destroy-kind-cluster", true, "destroy the kind cluster when tests complete")
	c.preinstallCrossplane = flag.Bool("preinstall-crossplane", true, "install Crossplane before running tests")
	c.loadImagesKindCluster = flag.Bool("load-images-kind-cluster", true, "load Crossplane images into the kind cluster before running tests")
	c.selectedTestSuite = &selectedTestSuite{}
	flag.Var(c.selectedTestSuite, testSuiteFlag, "test suite defining environment setup and tests to run")
	// Need to override the default usage message to allow setting the available
	// suites at runtime.
	flag.Usage = func() {
		if f := flag.Lookup(testSuiteFlag); f != nil {
			f.Usage = fmt.Sprintf("%s. Available options: %+v", f.Usage, c.getAvailableSuitesOptions())
		}
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	return c
}

func (e *E2EConfig) getAvailableSuitesOptions() (opts []string) {
	for s := range e.suites {
		opts = append(opts, s)
	}
	sort.Strings(opts)
	return
}

// GetKindClusterName returns the name of the kind cluster, returns empty string
// if it's not a kind cluster.
func (e *E2EConfig) GetKindClusterName() string {
	if !e.IsKindCluster() {
		return ""
	}
	if *e.kindClusterName == "" {
		name := envconf.RandomName("crossplane-e2e", 32)
		e.kindClusterName = &name
	}
	return *e.kindClusterName
}

// IsKindCluster returns true if the test is running against a kind cluster.
func (e *E2EConfig) IsKindCluster() bool {
	return *e.createKindCluster || *e.kindClusterName != ""
}

// ShouldLoadImages returns true if the test should load images into the kind
// cluster.
func (e *E2EConfig) ShouldLoadImages() bool {
	return *e.loadImagesKindCluster && e.IsKindCluster()
}

// GetSuiteInstallOpts returns the helm install options for the specified
// suite, appending additional specified ones
func (e *E2EConfig) GetSuiteInstallOpts(suite string, extra ...helm.Option) []helm.Option {
	p, ok := e.suites[suite]
	if !ok {
		panic(fmt.Sprintf("The selected suite %q does not exist", suite))
	}
	opts := p.helmInstallOpts
	if !p.excludeBaseSuite {
		opts = append(e.suites[TestSuiteDefault].helmInstallOpts, opts...)
	}
	return append(opts, extra...)
}

// GetSelectedSuiteInstallOpts returns the helm install options for the
// selected suite, appending additional specified ones.
func (e *E2EConfig) GetSelectedSuiteInstallOpts(extra ...helm.Option) []helm.Option {
	return e.GetSuiteInstallOpts(e.selectedTestSuite.String(), extra...)
}

// AddTestSuite adds a new test suite, panics if already defined.
func (e *E2EConfig) AddTestSuite(name string, opts ...TestSuiteOpt) {
	if _, ok := e.suites[name]; ok {
		panic(fmt.Sprintf("suite already defined: %s", name))
	}

	o := testSuite{}
	for _, opt := range opts {
		opt(&o)
	}
	e.suites[name] = o
}

// AddDefaultTestSuite adds the default suite, panics if already defined.
func (e *E2EConfig) AddDefaultTestSuite(opts ...TestSuiteOpt) {
	e.AddTestSuite(TestSuiteDefault, append([]TestSuiteOpt{WithoutBaseDefaultTestSuite()}, opts...)...)
}

// TestSuiteOpt is an option to midify a testSuite.
type TestSuiteOpt func(*testSuite)

// WithoutBaseDefaultTestSuite sets the provided testSuite to not include the base
// one.
func WithoutBaseDefaultTestSuite() TestSuiteOpt {
	return func(suite *testSuite) {
		suite.excludeBaseSuite = true
	}
}

// WithLabelsToSelect sets the provided testSuite to include the provided
// labels, if not already specified by the user
func WithLabelsToSelect(labels features.Labels) TestSuiteOpt {
	return func(suite *testSuite) {
		suite.labelsToSelect = labels
	}
}

// WithHelmInstallOpts sets the provided testSuite to include the provided
// helm install options.
func WithHelmInstallOpts(opts ...helm.Option) TestSuiteOpt {
	return func(suite *testSuite) {
		suite.helmInstallOpts = append(suite.helmInstallOpts, opts...)
	}
}

// WithConditionalEnvSetupFuncs sets the provided testSuite to include the
// provided env setup funcs, if the condition is true, when evaluated.
func WithConditionalEnvSetupFuncs(condition func() bool, funcs ...env.Func) TestSuiteOpt {
	return func(suite *testSuite) {
		suite.additionalSetupFuncs = append(suite.additionalSetupFuncs, conditionalSetupFunc{condition, funcs})
	}
}

// HelmOptions valid for installing and upgrading the Crossplane Helm chart.
// Used to install Crossplane before any test starts, but some tests also use
// these options - for example to reinstall Crossplane with a feature flag
// enabled.
func (e *E2EConfig) HelmOptions(extra ...helm.Option) []helm.Option {
	return append(e.GetSelectedSuiteInstallOpts(), extra...)
}

// HelmOptionsForSuite returns the Helm options for the specified suite,
// appending additional specified ones.
func (e *E2EConfig) HelmOptionsForSuite(suite string, extra ...helm.Option) []helm.Option {
	return append(e.GetSuiteInstallOpts(suite), extra...)
}

// ShouldInstallCrossplane returns true if the test should install Crossplane
// before starting.
func (e *E2EConfig) ShouldInstallCrossplane() bool {
	return *e.preinstallCrossplane
}

// ShouldDestroyKindCluster returns true if the test should destroy the kind
// cluster after finishing.
func (e *E2EConfig) ShouldDestroyKindCluster() bool {
	return *e.destroyKindCluster && e.IsKindCluster()
}

// GetSelectedSuiteLabels returns the labels to select for the selected suite.
func (e *E2EConfig) getSelectedSuiteLabels() features.Labels {
	if !e.selectedTestSuite.set {
		return nil
	}
	return e.suites[e.selectedTestSuite.String()].labelsToSelect
}

// GetSelectedSuiteAdditionalEnvSetup returns the additional env setup funcs
// for the selected suite, to be run before installing Crossplane, if required.
func (e *E2EConfig) GetSelectedSuiteAdditionalEnvSetup() (out []env.Func) {
	selectedTestSuite := e.selectedTestSuite.String()
	for _, s := range e.suites[selectedTestSuite].additionalSetupFuncs {
		if s.condition() {
			out = append(out, s.f...)
		}
	}
	if selectedTestSuite == TestSuiteDefault {
		for name, suite := range e.suites {
			if name == TestSuiteDefault {
				continue
			}
			for _, setupFunc := range suite.additionalSetupFuncs {
				if setupFunc.condition() {
					out = append(out, setupFunc.f...)
				}
			}
		}
	}
	return out
}

// EnrichLabels returns the provided labels enriched with the selected suite
// labels, preserving user-specified ones in case of key conflicts.
func (e *E2EConfig) EnrichLabels(labels features.Labels) features.Labels {
	if e.isSelectingTests() {
		return labels
	}
	if labels == nil {
		labels = make(features.Labels)
	}
	for k, v := range e.getSelectedSuiteLabels() {
		if _, ok := labels[k]; ok {
			continue
		}
		labels[k] = v
	}
	return labels
}

func (e *E2EConfig) isSelectingTests() bool {
	if e.specificTestSelected == nil {
		f := flag.Lookup("test.run")
		e.specificTestSelected = pointer.Bool(f != nil && f.Value.String() != "")
	}
	return *e.specificTestSelected
}
