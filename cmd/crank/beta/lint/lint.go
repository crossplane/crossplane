package lint

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return fmt.Sprintf("%s.%s", path, key)
}

func walkYAML(path string, node *yaml.Node, fn func(path string, key *yaml.Node, val *yaml.Node)) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, content := range node.Content {
			walkYAML(path, content, fn)
		}

	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			fn(path, keyNode, valueNode)

			newPath := joinPath(path, keyNode.Value)
			walkYAML(newPath, valueNode, fn)
		}

	case yaml.SequenceNode:
		for idx, item := range node.Content {
			newPath := fmt.Sprintf("%s[%d]", path, idx)
			walkYAML(newPath, item, fn)
		}
	case yaml.ScalarNode:
		return // No action needed for scalar nodes
	case yaml.AliasNode:
		return // No action needed for alias nodes
	}
}

func checkRuleExclusion(n *yaml.Node, id string) bool {
	if n.HeadComment == "# nolint "+id || n.HeadComment == "# nolint" {
		return true
	}
	return false
}

// Rule XRD001: Warn if any field has type boolean.
func checkBooleanFields(name string, obj *yaml.Node) []Issue {
	var issues []Issue

	checkBoolean := func(path string, key *yaml.Node, val *yaml.Node) {
		if checkRuleExclusion(key, "XRD001") {
			return
		}
		if key.Value == "type" && val.Value == "boolean" {
			issue := Issue{
				Name:      name,
				Line:      key.Line,
				Error:     false,
				RuleID:    "XRD001",
				Reference: "https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#primitive-types",
				Message:   fmt.Sprintf("Boolean field detected at path %s — consider using an enum instead for extensibility.", path),
			}
			issues = append(issues, issue)
		}
	}

	walkYAML("", obj, checkBoolean)
	return issues
}

// Rule XRD002: Check for required fields.
func checkRequiredFields(name string, obj *yaml.Node) []Issue {
	var issues []Issue

	checkRequired := func(path string, key *yaml.Node, val *yaml.Node) {
		if checkRuleExclusion(key, "XRD002") {
			return
		}
		if key.Value == "required" && val.Kind == yaml.SequenceNode {
			for _, item := range val.Content {
				issue := Issue{
					Name:    name,
					Line:    item.Line,
					RuleID:  "XRD002",
					Message: fmt.Sprintf("Required field '%s' at path %s.%s — consider making it optional with a default.", item.Value, path, key.Value),
				}
				issues = append(issues, issue)
			}
		}
	}

	walkYAML("", obj, checkRequired)
	return issues
}

// Rule XRD003: Check fields for description.
func checkMissingDescriptions(name string, obj *yaml.Node) []Issue {
	var issues []Issue

	checkDescription := func(path string, key *yaml.Node, val *yaml.Node) {
		if key.Value == "properties" && val.Kind == yaml.MappingNode {
			for i := 0; i < len(val.Content)-1; i += 2 {
				fieldKey := val.Content[i]
				fieldVal := val.Content[i+1]
				found := false
				if checkRuleExclusion(fieldKey, "XRD0003") {
					continue
				}
				for j := 0; j < len(fieldVal.Content)-1; j += 2 {
					if fieldVal.Content[j].Value == "description" {
						found = true
						break
					}
				}
				if found || strings.HasSuffix(path, ".schema.openAPIV3Schema") {
					continue
				}
				issue := Issue{
					Name:    name,
					Line:    fieldKey.Line,
					RuleID:  "XRD003",
					Message: fmt.Sprintf("Missing description for field '%s' at path %s.%s", fieldKey.Value, path, key.Value),
				}

				issues = append(issues, issue)
			}
		}
	}

	walkYAML("", obj, checkDescription)
	return issues
}
