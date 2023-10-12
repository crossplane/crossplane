package printer

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/emicklei/dot"
	"github.com/pkg/errors"
)

type GraphPrinter struct {
	writer io.Writer
}

// Initialize a new graph printer
func NewGraphPrinter() *GraphPrinter {
	return &GraphPrinter{writer: os.Stdout}
}

// Set a new graph. Gets all the nodes and then return the graph as a dot format string.
func (p *GraphPrinter) PrintDotGraph(resource k8s.Resource, fields []string) (string, error) {
	g := dot.NewGraph(dot.Undirected)
	p.buildGraph(g, resource, fields)

	dot_string := g.String()
	if dot_string == "" {
		return "", errors.New("Graph is empty.")
	}

	return dot_string, nil
}

// Iteratre over resources and set ID and label(content) of each node
func (p *GraphPrinter) buildGraph(g *dot.Graph, r k8s.Resource, fields []string) {
	node := g.Node(resourceId(r))
	node.Label(resourceLabel(r, fields))
	node.Attr("penwidth", "2")

	for _, child := range r.Children {
		p.buildGraph(g, child, fields)
		g.Edge(node, g.Node(resourceId(child)))
	}
}

// Set individual resourceID for node
func resourceId(r k8s.Resource) string {
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
			label[i] = field + ": " + r.GetApiVersion()
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
