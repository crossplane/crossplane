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
	Resources string `arg:"" help:"Resources - currently limited to XRDs - can be provided as a file, a directory, or indicated by '-' for standard input."`

	// Flags.
	Output        string `default:"stdout" help:"Output format. Valid values are 'stdout' or 'json'. Default is 'stdout'." short:"o"`
	SkipReference bool   `default:"false"  help:"Skip printing the reference to the rule that was violated."`
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

// Issue represents a linting issue found in a resource.
type Issue struct {
	RuleID    string `json:"id"`
	Name      string `json:"name"`
	Line      int    `json:"line"`
	Error     bool   `json:"error"`
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

type output struct {
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
		resourceAsBytes, err := yaml.Marshal(resource)
		if err != nil {
			return errors.Wrapf(err, "cannot convert to yaml %q", resource.Value)
		}

		var xrd unstructured.Unstructured
		if err := sigyaml.Unmarshal(resourceAsBytes, &xrd); err != nil {
			return errors.Wrapf(err, "cannot unmarshal into Unstructured %q", resource.Value)
		}

		name := xrd.GetName()

		if xrd.GetKind() != v1.CompositeResourceDefinitionKind || !strings.HasPrefix(xrd.GetAPIVersion(), v1.Group) {
			issue := Issue{
				Name:    name,
				Line:    0,
				Error:   true,
				RuleID:  "XRD000",
				Message: fmt.Sprintf("Resource %s is not a CompositeResourceDefinition", xrd.GetName()),
			}
			allIssues = append(allIssues, issue)
			continue
		}

		allIssues = append(allIssues, checkBooleanFields(name, resource)...)
		allIssues = append(allIssues, checkRequiredFields(name, resource)...)
		allIssues = append(allIssues, checkMissingDescriptions(name, resource)...)
	}

	output := output{}
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

	if !output.Summary.Valid {
		switch c.Output {
		case "json":
			err := printJSON(&output, k)
			if err != nil {
				return errors.Wrap(err, "cannot print summary")
			}
		case "stdout":
			err = printStdout(&output, k, c.SkipReference)
			if err != nil {
				return errors.Wrap(err, "cannot print summary")
			}
		default:
			return errors.New("invalid output format specified")
		}
	}

	if output.Summary.Errors > 0 {
		os.Exit(1)
	} else if output.Summary.Warnings > 0 {
		os.Exit(2)
	}

	os.Exit(0)

	return nil
}

func printJSON(o *output, k *kong.Context) error {
	e := json.NewEncoder(k.Stdout)
	err := e.Encode(o)
	return err
}

func printStdout(o *output, k *kong.Context, skipReference bool) error {
	for _, issue := range *o.Issues {
		reference := ""
		if issue.Reference != "" && !skipReference {
			reference = fmt.Sprintf("More information: (%s)", issue.Reference)
		}
		_, err := fmt.Fprintf(k.Stdout, "%s:%d [%s] %s %s\n", issue.Name, issue.Line, issue.RuleID, issue.Message, reference)
		if err != nil {
			return errors.Wrap(err, "cannot print summary")
		}
	}
	_, err := fmt.Fprintf(k.Stdout, "Found %d issues: %d errors, %d warnings\n", o.Summary.Total, o.Summary.Errors, o.Summary.Warnings)
	if err != nil {
		return errors.Wrap(err, "cannot print summary")
	}
	return nil
}
