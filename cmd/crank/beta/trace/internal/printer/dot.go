package printer

import (
	"fmt"
	"io"
	"strings"

	"github.com/emicklei/dot"
	"github.com/pkg/errors"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource/xpkg"
)

// DotPrinter defines the DotPrinter configuration
type DotPrinter struct {
}

var _ Printer = &DotPrinter{}

type dotLabel struct {
	namespace  string
	apiVersion string
	name       string
	ready      string
	synced     string
	error      string
}

func (r *dotLabel) String() string {
	out := []string{
		"Name: " + r.name,
		"ApiVersion: " + r.apiVersion,
	}
	if r.namespace != "" {
		out = append(out,
			"Namespace: "+r.namespace)
	}

	out = append(out,
		"Ready: "+r.ready,
		"Synced: "+r.synced,
	)
	if r.error != "" {
		out = append(out,
			"Error: "+r.error,
		)
	}
	return strings.Join(out, "\n") + "\n"
}

type dotPackageLabel struct {
	apiVersion string
	name       string
	pkg        string
	installed  string
	healthy    string
	state      string
	error      string
}

func (r *dotPackageLabel) String() string {
	out := []string{
		"Name: " + r.name,
		"ApiVersion: " + r.apiVersion,
		"Package: " + r.pkg,
	}
	if r.installed != "" {
		out = append(out,
			"Installed: "+r.installed)
	}
	out = append(out,
		"Healthy: "+r.healthy,
	)
	if r.state != "" {
		out = append(out,
			"State: "+r.state,
		)
	}

	if r.error != "" {
		out = append(out,
			"Error: "+r.error,
		)
	}
	return strings.Join(out, "\n") + "\n"
}

// Print gets all the nodes and then return the graph as a dot format string to the Writer.
func (p *DotPrinter) Print(w io.Writer, root *resource.Resource) error {
	g := dot.NewGraph(dot.Undirected)

	type queueItem struct {
		resource *resource.Resource
		parent   *dot.Node
	}

	queue := []*queueItem{{root, nil}}
	var id int

	for len(queue) > 0 {
		// Dequeue the first element from the start
		item := queue[0]
		queue = queue[1:]

		node := g.Node(fmt.Sprintf("%d", id))
		id++

		if item.parent != nil {
			g.Edge(*item.parent, node)
		}
		var label fmt.Stringer
		gk := item.resource.Unstructured.GroupVersionKind().GroupKind()
		switch {
		case xpkg.IsPackageType(gk):
			pkg, err := fieldpath.Pave(item.resource.Unstructured.Object).GetString("spec.package")
			l := &dotPackageLabel{
				apiVersion: item.resource.Unstructured.GroupVersionKind().GroupVersion().String(),
				name:       item.resource.Unstructured.GetName(),
				pkg:        pkg,
				installed:  string(item.resource.GetCondition(v1.TypeInstalled).Status),
				healthy:    string(item.resource.GetCondition(v1.TypeHealthy).Status),
			}
			if err != nil {
				l.error = err.Error()
			}
			label = l
		case xpkg.IsPackageRevisionType(gk):
			pkg, err := fieldpath.Pave(item.resource.Unstructured.Object).GetString("spec.image")
			l := &dotPackageLabel{
				apiVersion: item.resource.Unstructured.GroupVersionKind().GroupVersion().String(),
				name:       item.resource.Unstructured.GetName(),
				pkg:        pkg,
				healthy:    string(item.resource.GetCondition(v1.TypeHealthy).Status),
				state:      string(item.resource.GetCondition(v1.TypeHealthy).Reason),
			}
			if err != nil {
				l.error = err.Error()
			}
			label = l
		default:
			label = &dotLabel{
				namespace:  item.resource.Unstructured.GetNamespace(),
				apiVersion: item.resource.Unstructured.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				name:       fmt.Sprintf("%s/%s", item.resource.Unstructured.GetKind(), item.resource.Unstructured.GetName()),
				ready:      string(item.resource.GetCondition(xpv1.TypeReady).Status),
				synced:     string(item.resource.GetCondition(xpv1.TypeSynced).Status),
			}
		}
		node.Label(label.String())
		node.Attr("penwidth", "2")

		// Push the children to the stack, increasing the depth
		for _, child := range item.resource.Children {
			queue = append(queue, &queueItem{child, &node})
		}
	}
	dotString := g.String()
	if dotString == "" {
		return errors.New("graph is empty")
	}

	g.Write(w)

	return nil
}
