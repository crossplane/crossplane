package testutils

import (
	"regexp"
	"strings"
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
