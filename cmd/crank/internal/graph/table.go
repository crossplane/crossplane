package graph

import (
	"bytes"
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
	var buf bytes.Buffer

	// Create a new table and set header, but write to the buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader(fields)

	// add all children to the table
	if err := cliTableAddResource(table, fields, r, ""); err != nil {
		return err
	}

	table.Render()

	// Write the table content from the buffer to the provided io.Writer
	_, err := io.Copy(w, &buf)
	if err != nil {
		return err
	}

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
			tableRow[i] = r.GetName()
		}
		if field == "kind" {
			tableRow[i] = r.GetKind()
		}
		if field == "namespace" {
			tableRow[i] = r.GetNamespace()
		}
		if field == "apiversion" {
			tableRow[i] = r.GetAPIVersion()
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
		err := cliTableAddResource(table, fields, *child, r.GetKind())
		if err != nil {
			return err
		}
	}
	return nil
}
