// Package testutils is for test utilities.
package testutils

import (
	stdlog "log"
	"regexp"
	"strings"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/go-logr/stdr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	kt "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Colors for terminal output.
const (
	ColorRed   = "\x1b[31m"
	ColorGreen = "\x1b[32m"
	ColorReset = "\x1b[0m"
)

// Green takes a multiline string, splits it by line, and adds green coloring to each line.
// It returns a single string with all lines joined back together.
func Green(input string) string {
	lines := strings.Split(input, "\n")
	coloredLines := make([]string, 0, len(lines))

	for _, line := range lines {
		// Handle the case of the last empty line after a newline
		if line == "" && len(coloredLines) == len(lines)-1 {
			coloredLines = append(coloredLines, "")
			continue
		}
		coloredLines = append(coloredLines, ColorGreen+line+ColorReset)
	}

	return strings.Join(coloredLines, "\n")
}

// Red takes a multiline string, splits it by line, and adds red coloring to each line.
// It returns a single string with all lines joined back together.
func Red(input string) string {
	lines := strings.Split(input, "\n")
	coloredLines := make([]string, 0, len(lines))

	for _, line := range lines {
		// Handle the case of the last empty line after a newline
		if line == "" && len(coloredLines) == len(lines)-1 {
			coloredLines = append(coloredLines, "")
			continue
		}
		coloredLines = append(coloredLines, ColorRed+line+ColorReset)
	}

	return strings.Join(coloredLines, "\n")
}

// CompareIgnoringAnsi compares two strings after stripping ANSI special characters.
func CompareIgnoringAnsi(expected, actual string) bool {
	// Strip ANSI codes from both strings
	ansiPattern := regexp.MustCompile("\x1b\\[[0-9;]*m")
	expectedStripped := ansiPattern.ReplaceAllString(expected, "")
	actualStripped := ansiPattern.ReplaceAllString(actual, "")

	// Compare the stripped strings
	return expectedStripped == actualStripped
}

// SetupKubeTestLogger sets the global logger for use of the Kube environment to the T.Log of this test.
func SetupKubeTestLogger(t *testing.T) {
	t.Helper()

	// Create a logr.Logger that writes to testing.T.Log
	testLogger := stdr.NewWithOptions(stdlog.New(testWriter{t}, "", 0), stdr.Options{LogCaller: stdr.All})

	// Set the logger for controller-runtime
	log.SetLogger(testLogger)
}

// testWriter adapts testing.T.Log to io.Writer.
type testWriter struct {
	t *testing.T
}

// Write logs the provided argument as a string to the T.Log.
func (tw testWriter) Write(p []byte) (int, error) {
	tw.t.Log(string(p))
	return len(p), nil
}

// TestLogger coerces the T.Log into the shape of a Logr logger.
func TestLogger(t *testing.T, verbose bool) logging.Logger {
	verbosity := 0
	if verbose {
		verbosity = 1
	}
	return logging.NewLogrLogger(testr.NewWithOptions(t, testr.Options{Verbosity: verbosity}))
}

// CreateFakeDiscoveryClient is a helper function to create a fake discovery client for testing.
func CreateFakeDiscoveryClient(resources map[string][]metav1.APIResource) discovery.DiscoveryInterface {
	fakeDiscovery := &fakediscovery.FakeDiscovery{
		Fake: &kt.Fake{},
	}

	apiResourceLists := make([]*metav1.APIResourceList, 0, len(resources))

	for gv, apiResources := range resources {
		apiResourceLists = append(apiResourceLists, &metav1.APIResourceList{
			GroupVersion: gv,
			APIResources: apiResources,
		})
	}

	fakeDiscovery.Resources = apiResourceLists
	return fakeDiscovery
}
