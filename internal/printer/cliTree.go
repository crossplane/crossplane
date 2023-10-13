package printer

import (
	"fmt"
	"io"
	"strings"

	"github.com/crossplane/crossplane/internal/k8s"
)

type TreePrinter struct {
	Indent string
	IsLast bool
}

var _ Printer = &TreePrinter{}

func (p *TreePrinter) Print(w io.Writer, r k8s.Resource, fields []string) error {
	io.WriteString(w, p.Indent)

	if p.IsLast {
		io.WriteString(w, "└─ ")
		p.Indent += "  "
	} else {
		io.WriteString(w, "├─ ")
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
			output[i] = fmt.Sprintf("ApiVersion: %s", r.GetApiVersion())
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
	io.WriteString(w, strings.Join(output, ", ")+"\n")

	for i, child := range r.Children {
		childPrinter := TreePrinter{
			Indent: p.Indent,
			IsLast: i == len(r.Children)-1,
		}
		childPrinter.Print(w, *child, fields)
	}
	return nil
}
