package graph

import (
	"io"

	"github.com/olekukonko/tablewriter"
)

// Table defines the Table configuration
type Table struct {
}

var _ Printer = &Table{}

// Print writes a CLI table of the passed Resource to the Writer. The fields variable determines the header and values of the table.
func (p *Table) Print(w io.Writer, r Resource, fields []string) error {
	// Create a buffer to capture the table output
	table := tablewriter.NewWriter(w)
	table.SetHeader(fields)

	// add all children to the table
	if err := cliTableAddResource(table, fields, r, ""); err != nil {
		return err
	}

	table.Render()

	return nil
}

// CliTableAddResource adds rows to the passed table in the order and as specified in the fields variable
//
//nolint:gocyclo // This is a simple for loop with if-statements on how to populate fields.
func cliTableAddResource(table *tablewriter.Table, fields []string, r Resource, parentKind string) error {
	var tableRow = make([]string, len(fields))

	// Using this for loop and if statement approach ensures keeping the same output order as the fields argument defined
	for i, field := range fields {
		if field == "parent" {
			var parentPrefix string
			if parentKind != "" {
				parentPrefix = parentKind
			}
			tableRow[i] = parentPrefix
		}
		if field == "name" {
			tableRow[i] = r.manifest.GetName()
		}
		if field == "kind" {
			tableRow[i] = r.manifest.GetKind()
		}
		if field == "namespace" {
			tableRow[i] = r.manifest.GetNamespace()
		}
		if field == "apiversion" {
			tableRow[i] = r.manifest.GetAPIVersion()
		}
		if field == "synced" {
			tableRow[i] = r.GetConditionStatus("Synced")
		}
		if field == "ready" {
			tableRow[i] = r.GetConditionStatus("Ready")
		}
		if field == "message" {
			tableRow[i] = r.GetConditionMessage()
		}
		if field == "event" {
			tableRow[i] = r.GetEvent()
		}
	}

	// Add the row to the table.
	table.Append(tableRow)

	// Recursively print children with the updated parent information.
	for _, child := range r.Children {
		err := cliTableAddResource(table, fields, *child, r.manifest.GetKind())
		if err != nil {
			return err
		}
	}
	return nil
}
