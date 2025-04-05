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

// Package lint implements offline checks of Composite Resource Definitions (XRDs).
package lint

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"gopkg.in/yaml.v3"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

type Issue struct {
	Name    string
	Line    int
	Error   bool
	RuleID  string
	Message string
}

type Rule interface {
	Check(name string, xrd *v1.CompositeResourceDefinition) []Issue
}

// Cmd arguments and flags for render subcommand.
type Cmd struct {
	// Arguments.
	Resources string `arg:"" help:"Resources source which can be a file, directory, or '-' for standard input."`
}

// Help prints out the help for the validate command.
func (c *Cmd) Help() string {
	return `
TODO
`
}

func DecodeNodeTo(obj *yaml.Node, out interface{}) error {
	b, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, out)
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

		var xrd v1.CompositeResourceDefinition
		if err := sigyaml.Unmarshal(resourceAsBytes, &xrd); err != nil {
			fmt.Printf("Failed to decode object into XRD: %v\n", err)
			continue
		}

		name = xrd.GetName()

		allIssues = append(allIssues, checkBooleanFieldsYaml(name, resource)...)
	}

	for _, issue := range allIssues {
		fmt.Printf("%s:%d [%s] %s\n", issue.Name, issue.Line, issue.RuleID, issue.Message)
	}

	fmt.Printf("Linting complete!\n")

	for _, issue := range allIssues {
		if issue.Error {
			os.Exit(1)
		}
	}

	if len(allIssues) > 0 {
		os.Exit(2)
	}

	return nil
}

// func lintXRD(xrd *v1.CompositeResourceDefinition) {
// 	fmt.Printf("Linting XRD: %s\n", xrd.Name)

// 	if xrd.Spec.Group == "" {
// 		fmt.Println("spec.group is missing")
// 	}

// 	if xrd.Spec.Names.Kind == "" {
// 		fmt.Println("spec.names.kind is missing")
// 	}

// 	if len(xrd.Spec.Versions) == 0 {
// 		fmt.Println("spec.versions is missing or empty")
// 	}

// 	if xrd.Spec.ClaimNames == nil {
// 		fmt.Println("spec.claimNames is not defined – consider adding it for portability")
// 	}

// 	for _, v := range xrd.Spec.Versions {
// 		if !v.Served {
// 			fmt.Printf("version %q is not served\n", v.Name)
// 		}
// 		if !v.Referenceable {
// 			fmt.Printf("version %q is not referenceable\n", v.Name)
// 		}
// 	}

// 	if len(xrd.Spec.ConnectionSecretKeys) == 0 {
// 		fmt.Println("connectionSecretKeys is missing or empty – consumers may expect connection details")
// 	}

// 	fmt.Println("Linting complete")
// }

// Rule XRD001: Warn if any field has type boolean
func checkBooleanFieldsYaml(name string, xrd *yaml.Node) []Issue {
	var issues []Issue
	var walk func(path string, node *yaml.Node)

	walk = func(path string, node *yaml.Node) {
		switch node.Kind {
		case yaml.DocumentNode:
			for _, content := range node.Content {
				walk(path, content)
			}

		case yaml.MappingNode:
			for i := 0; i < len(node.Content); i += 2 {
				keyNode := node.Content[i]
				valueNode := node.Content[i+1]

				if keyNode.Value == "type" && valueNode.Value == "boolean" {
					issue := Issue{
						Name:    name,
						Line:    keyNode.Line,
						Error:   false,
						RuleID:  "XRD0001",
						Message: fmt.Sprintf("Boolean field detected at path %s — consider using an enum instead for extensibility.", path),
					}
					issues = append(issues, issue)
				}

				newPath := fmt.Sprintf("%s.%s", path, keyNode.Value)

				walk(newPath, valueNode)
			}

		case yaml.SequenceNode:
			for idx, item := range node.Content {
				newPath := fmt.Sprintf("%s[%d]", path, idx)
				walk(newPath, item)
			}
		}
	}

	walk("", xrd)

	return issues
}
