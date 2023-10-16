package graph

import (
	"fmt"
	"io"
	"strings"
)

// Tree defines the Tree configuration
type Tree struct {
	Indent string
	IsLast bool
}

var _ Printer = &Tree{}

// Print writes the output to a Writer. The output of print is a tree, e.g. as in the bash `tree` command
//
//nolint:gocyclo // This is a simple for loop with if-statements on how to populate fields.
func (p *Tree) Print(w io.Writer, r Resource, fields []string) error {
	_, err := io.WriteString(w, p.Indent)
	if err != nil {
		return err
	}

	if p.IsLast {
		_, err := io.WriteString(w, "└─ ")
		if err != nil {
			return err
		}
		p.Indent += "  "
	} else {
		_, err := io.WriteString(w, "├─ ")
		if err != nil {
			return err
		}
		p.Indent += "│ "
	}

	var output = make([]string, len(fields))
	for i, field := range fields {
		if field == "name" {
			output[i] = fmt.Sprintf("Name: %s", r.GetName())
		}
		if field == "kind" {
			output[i] = fmt.Sprintf("Kind: %s", r.GetKind())
		}
		if field == "namespace" {
			output[i] = fmt.Sprintf("Namespace: %s", r.GetNamespace())
		}
		if field == "apiversion" {
			output[i] = fmt.Sprintf("ApiVersion: %s", r.GetAPIVersion())
		}
		if field == "synced" {
			output[i] = fmt.Sprintf("Synced: %s", r.GetConditionStatus("Synced"))
		}
		if field == "ready" {
			output[i] = fmt.Sprintf("Ready: %s", r.GetConditionStatus("Ready"))
		}
		if field == "message" {
			output[i] = fmt.Sprintf("Message: %s", r.GetConditionMessage())
		}
		if field == "event" {
			output[i] = fmt.Sprintf("Event: %s", r.GetEvent())
		}
	}
	_, err = io.WriteString(w, strings.Join(output, ", ")+"\n")
	if err != nil {
		return err
	}

	for i, child := range r.Children {
		childPrinter := Tree{
			Indent: p.Indent,
			IsLast: i == len(r.Children)-1,
		}
		err := childPrinter.Print(w, *child, fields)
		if err != nil {
			return err
		}
	}
	return nil
}
