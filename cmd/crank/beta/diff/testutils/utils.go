package testutils

import (
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/go-logr/logr/testr"
	"github.com/go-logr/stdr"
	stdlog "log"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"testing"
)

// Colors for terminal output
const (
	ColorRed   = "\x1b[31m"
	ColorGreen = "\x1b[32m"
	ColorReset = "\x1b[0m"
)

// green takes a multiline string, splits it by line, and adds green coloring to each line.
// It returns a single string with all lines joined back together.
func Green(input string) string {
	lines := strings.Split(input, "\n")
	var coloredLines []string

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

// red takes a multiline string, splits it by line, and adds red coloring to each line.
// It returns a single string with all lines joined back together.
func Red(input string) string {
	lines := strings.Split(input, "\n")
	var coloredLines []string

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

func CompareIgnoringAnsi(expected, actual string) bool {
	// Strip ANSI codes from both strings
	ansiPattern := regexp.MustCompile("\x1b\\[[0-9;]*m")
	expectedStripped := ansiPattern.ReplaceAllString(expected, "")
	actualStripped := ansiPattern.ReplaceAllString(actual, "")

	// Compare the stripped strings
	return expectedStripped == actualStripped
}

func SetupKubeTestLogger(t *testing.T) {
	// Create a logr.Logger that writes to testing.T.Log
	testLogger := stdr.NewWithOptions(stdlog.New(testWriter{t}, "", 0), stdr.Options{LogCaller: stdr.All})

	// Set the logger for controller-runtime
	log.SetLogger(testLogger)
}

// testWriter adapts testing.T.Log to io.Writer
type testWriter struct {
	t *testing.T
}

func (tw testWriter) Write(p []byte) (int, error) {
	tw.t.Log(string(p))
	return len(p), nil
}

func TestLogger(t *testing.T) logging.Logger {
	return logging.NewLogrLogger(testr.New(t))
}
