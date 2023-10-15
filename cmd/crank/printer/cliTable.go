package printer

import (
	"bytes"
	"fmt"
	"io"

	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/olekukonko/tablewriter"
)

type CliPrinter struct {
}

var _ Printer = &CliPrinter{}

// Prints out a CLI table of the passed Resource. The fields variable determines the header and values of the table.
func (p *CliPrinter) Print(w io.Writer, r k8s.Resource, fields []string) error {
	// Create a buffer to capture the table output
	var buf bytes.Buffer

	// Create a new table and set header, but write to the buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader(fields)

	// add all children to the table
	if err := p.CliTableAddResource(table, fields, r, ""); err != nil {
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

// This functions adds rows to the passed table in the order and as specified in the fields variable
func (p *CliPrinter) CliTableAddResource(table *tablewriter.Table, fields []string, r k8s.Resource, parentKind string) error {
	var tableRow = make([]string, len(fields))

	// Using this for loop and if statement approach ensures keeping the same output order as the fields argument defined
	for i, field := range fields {
		if field == "parent" {
			var parentPrefix string
			if parentKind != "" {
				parentPrefix = fmt.Sprintf("%s", parentKind)
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
			tableRow[i] = r.GetApiVersion()
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
		err := p.CliTableAddResource(table, fields, *child, r.GetKind())
		if err != nil {
			return err
		}
	}
	return nil
}
