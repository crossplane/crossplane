/*
Copyright 2024 The Crossplane Authors.

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

// Package lint implements lint command for Composite Resource Definitions (XRDs).
package lint

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Cmd arguments and flags for render subcommand.
type Cmd struct {
	// Arguments.
	Resources string `arg:"" help:"Resources source which can be a file, directory, or '-' for standard input."`

	// Flags.
	Output             string `short:"o" help:"Output format. Valid values are 'stdout' or 'json'. Default is 'stdout'." default:"stdout"`
	SkipSuccessResults bool   `help:"Skip printing success results."`
}

// Help prints out the help for the validate command.
func (c *Cmd) Help() string {
	return `
This command checks the provided Crossplane CompositeResourceDefinition (XRD) via stdin, files or directories of such files for 
API Design Best Practices. It scans each resource and identifies issues such as missing descriptions, required fields and more.

The command will return a non-zero exit code if any errors are found. The exit codes are as follows:
  - 0: No issues found
  - 1: Errors found
  - 2: Warnings found

Examples:

  # Lint a single XRD file 
  crossplane beta lint xrd.yaml

  # Lint all XRD files in the directory "xrds"
  crossplane beta lint ./xrds/

  # Lint all XRD files in the directory "xrds" and skip success logs
  crossplane beta validate ./xrds/ --skip-success-results

  # Lint all XRD files in the directory "xrds" and output results in JSON format
  crossplane beta lint ./xrds/ --output json
`
}

type Issue struct {
	RuleID  string `json:"id"`
	Name    string `json:"name"`
	Line    int    `json:"line"`
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

type Output struct {
	Summary struct {
		Valid    bool `json:"valid"`
		Total    int  `json:"total"`
		Errors   int  `json:"errors"`
		Warnings int  `json:"warnings"`
	} `json:"summary"`
	Issues *[]Issue `json:"issues"`
}

// Run lint.
func (c *Cmd) Run(k *kong.Context, _ logging.Logger) error {

	// Load all resources
	resourceLoader, err := NewLoader(c.Resources)
	if err != nil {
		return errors.Wrapf(err, "cannot load resources from %q", c.Resources)
	}

	resources, err := resourceLoader.Load()
	if err != nil {
		return errors.Wrapf(err, "cannot load resources from %q", c.Resources)
	}

	var allIssues []Issue

	for _, resource := range resources {

		var name string
		resourceAsBytes, err := yaml.Marshal(resource)
		if err != nil {
			fmt.Printf("Failed to convert resource to bytes: %v\n", err)
		}

		var k8s unstructured.Unstructured
		if err := sigyaml.Unmarshal(resourceAsBytes, &k8s); err != nil {
			fmt.Printf("Failed to decode object into XRD: %v\n", err)
			continue
		}

		name = k8s.GetName()

		if k8s.GetKind() != v1.CompositeResourceDefinitionKind || !strings.HasPrefix(k8s.GetAPIVersion(), v1.Group) {
			issue := Issue{
				Name:    name,
				Line:    0,
				Error:   true,
				RuleID:  "XRD000",
				Message: fmt.Sprintf("Resource %s is not a CompositeResourceDefinition", k8s.GetName()),
			}
			allIssues = append(allIssues, issue)
		}

		allIssues = append(allIssues, checkBooleanFields(name, resource)...)
		allIssues = append(allIssues, checkRequiredFields(name, resource)...)
		allIssues = append(allIssues, checkMissingDescriptions(name, resource)...)
	}

	output := Output{}
	output.Summary.Valid = len(allIssues) == 0
	output.Summary.Total = len(allIssues)
	output.Issues = &allIssues

	for _, issue := range allIssues {
		if issue.Error {
			output.Summary.Errors++
		} else {
			output.Summary.Warnings++
		}
	}

	if !c.SkipSuccessResults || !output.Summary.Valid {
		switch c.Output {
		case "json":
			printJson(&output)
		case "stdout":
			printStdout(&output)
		default:
			return errors.New("invalid output format specified")
		}
	}

	if output.Summary.Errors > 0 {
		os.Exit(1)
	} else if output.Summary.Warnings > 0 {
		os.Exit(2)
	} else {
		os.Exit(0)
	}

	return nil
}

func printStdout(output *Output) {
	for _, issue := range *output.Issues {
		fmt.Printf("%s:%d [%s] %s\n", issue.Name, issue.Line, issue.RuleID, issue.Message)
	}

	fmt.Printf("Found %d issues: %d errors, %d warnings\n", output.Summary.Total, output.Summary.Errors, output.Summary.Warnings)
}

func printJson(output *Output) {
	data, err := json.Marshal(output)
	if err != nil {
		fmt.Printf("Failed to convert output to JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
