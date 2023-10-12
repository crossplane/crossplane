package printer

import (
	"fmt"
	"strings"

	"github.com/crossplane/crossplane/internal/k8s"
)

func PrintResourceTree(r k8s.Resource, fields []string, indent string, isLast bool) {
	fmt.Print(indent)

	if isLast {
		fmt.Print("└─ ")
		indent += "  "
	} else {
		fmt.Print("├─ ")
		indent += "│ "
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
			output[i] = fmt.Sprintf("ApiVersion: %s", r.GetApiVersion())
		}
		if field == "synced" {
			output[i] = fmt.Sprintf("Synced: %s", r.GetConditionStatus("Synced"))
		}
		if field == "ready" {
			output[i] = fmt.Sprintf("Ready: %s", r.GetConditionStatus("Ready"))
		}
		if field == "message" {
			output[i] = fmt.Sprintf("ApiVersion: %s", r.GetConditionMessage())
		}
		if field == "event" {
			output[i] = fmt.Sprintf("ApiVersion: %s", r.GetEvent())
		}
	}
	fmt.Println(strings.Join(output, ", "))

	for i, child := range r.Children {
		isLastChild := i == len(r.Children)-1
		PrintResourceTree(child, fields, indent, isLastChild)
	}
}
