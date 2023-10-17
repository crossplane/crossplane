package e2e

import (
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane/test/e2e/config"
)

const (
	// SuiteRealtimeCompositions is the value for the config.LabelTestSuite
	// label to be assigned to tests that should be part of the Usage test
	// suite.
	SuiteRealtimeCompositions = "realtime-compositions"
)

func init() {
	environment.AddTestSuite(SuiteRealtimeCompositions,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-realtime-compositions}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteRealtimeCompositions, config.TestSuiteDefault},
		}),
	)
}
