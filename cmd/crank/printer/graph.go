package printer

import (
	"fmt"
	"io"
	"strings"

	"github.com/emicklei/dot"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane/internal/k8s"
)

// GraphPrinter defines the GraphPrinter configuration
type GraphPrinter struct {
}

var _ Printer = &GraphPrinter{}

// Print gets all the nodes and then return the graph as a dot format string to the Writer.
func (p *GraphPrinter) Print(w io.Writer, resource k8s.Resource, fields []string) error {

	g := dot.NewGraph(dot.Undirected)
	p.buildGraph(g, resource, fields)

	dotString := g.String()
	if dotString == "" {
		return errors.New("graph is empty")
	}

	_, err := w.Write([]byte(g.String()))
	if err != nil {
		return err
	}
	return nil
}

// Iteratre over resources and set ID and label(content) of each node
func (p *GraphPrinter) buildGraph(g *dot.Graph, r k8s.Resource, fields []string) {
	node := g.Node(resourceID(r))
	node.Label(resourceLabel(r, fields))
	node.Attr("penwidth", "2")

	for _, child := range r.Children {
		p.buildGraph(g, *child, fields)
		g.Edge(node, g.Node(resourceID(*child)))
	}
}

// Set individual resourceID for node
func resourceID(r k8s.Resource) string {
	name := r.GetName()
	if len(name) > 24 {
		name = name[:12] + "..." + name[len(name)-12:]
	}
	kind := r.GetKind()
	return fmt.Sprintf("%s-%s", kind, name)
}

// This functions sets the label (the actual content) of the nodes in a graph.
// Fields are defined by the fields string.
func resourceLabel(r k8s.Resource, fields []string) string {

	var label = make([]string, len(fields))
	for i, field := range fields {
		if field == "name" {
			label[i] = field + ": " + r.GetName()
		}
		if field == "kind" {
			label[i] = field + ": " + r.GetKind()
		}
		if field == "namespace" {
			label[i] = field + ": " + r.GetNamespace()
		}
		if field == "apiversion" {
			label[i] = field + ": " + r.GetAPIVersion()
		}
		if field == "synced" {
			label[i] = field + ": " + r.GetConditionStatus("Synced")
		}
		if field == "ready" {
			label[i] = field + ": " + r.GetConditionStatus("Ready")
		}
		if field == "message" {
			label[i] = field + ": " + r.GetConditionMessage()
		}
		if field == "event" {
			label[i] = field + ": " + r.GetEvent()
		}
	}

	return strings.Join(label, "\n")
}
