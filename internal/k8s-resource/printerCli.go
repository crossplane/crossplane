package k8s_resource

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

// Takes a filled Resource which should be printed as input. The fields input defines the fields which are printed out and are set as header.
// The available fields for the fields variable are defined in the cmd/root.go file
func PrintResourceTable(rootResource Resource, fields []string) error {
	// Create a new table and set header
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(fields)

	// add all children to the table
	if err := printResourceAndChildren(table, fields, rootResource, ""); err != nil {
		return fmt.Errorf("Error getting resource field %w\n", err)
	}
	table.Render()

	return nil
}

// This functions adds rows to the passed table in the order and as specified in the fields variable
func printResourceAndChildren(table *tablewriter.Table, fields []string, r Resource, parentKind string) error {
	var tableRow = make([]string, len(fields))

	// Using this for loop and if statement approach ensures keeping the same output order as the fields argument was passed
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
	for _, child := range r.children {
		printResourceAndChildren(table, fields, child, r.GetKind())
	}
	return nil
}
