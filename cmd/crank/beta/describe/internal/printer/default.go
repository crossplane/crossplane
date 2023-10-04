/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package printer

import (
	"fmt"
	"io"
	"strings"

	"k8s.io/cli-runtime/pkg/printers"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/cmd/crank/beta/describe/internal/resource"
)

const (
	errFmtCannotWriteHeader    = "cannot write header: %s"
	errFmtCannotWriteRow       = "cannot write row: %s"
	errFmtCannotFlushTabWriter = "cannot flush tab writer: %s"
)

// DefaultPrinter defines the DefaultPrinter configuration
type DefaultPrinter struct {
}

var _ Printer = &DefaultPrinter{}

type defaultPrinterRow struct {
	namespace   string
	apiVersion  string
	name        string
	ready       string
	synced      string
	latestEvent string
}

func (r *defaultPrinterRow) String() string {
	return strings.Join([]string{
		r.namespace,
		r.apiVersion,
		r.name,
		r.ready,
		r.synced,
		r.latestEvent,
	}, "\t") + "\t"
}

// Print implements the Printer interface by prints the resource tree in a
// human-readable format.
func (p *DefaultPrinter) Print(w io.Writer, root *resource.Resource) error {
	tw := printers.GetNewTabWriter(w)

	headers := defaultPrinterRow{
		namespace:   "NAMESPACE",
		apiVersion:  "APIVERSION",
		name:        "NAME",
		ready:       "READY",
		synced:      "SYNCED",
		latestEvent: "LATESTEVENT",
	}
	if _, err := fmt.Fprintln(tw, headers.String()); err != nil {
		return errors.Errorf(errFmtCannotWriteHeader, err)
	}

	type queueItem struct {
		resource *resource.Resource
		depth    int
		isLast   bool
		prefix   string
	}

	// Initialize LIFO queue with root element
	queue := []*queueItem{{resource: root}}

	for len(queue) > 0 {
		// Dequeue first element
		var item *queueItem
		item, queue = queue[0], queue[1:] //nolint:gosec // false positive, the queue length has been checked above and [1:] works even if the queue is a single element

		// Choose the right prefix
		name := strings.Builder{}

		childPrefix := item.prefix
		if item.depth > 0 {
			if item.isLast {
				name.WriteString(item.prefix + "└── ")
				childPrefix += "    "
			} else {
				name.WriteString(item.prefix + "├── ")
				childPrefix += "│   "
			}
		}

		name.WriteString(fmt.Sprintf("%s/%s", item.resource.Unstructured.GetKind(), item.resource.Unstructured.GetName()))

		row := defaultPrinterRow{
			namespace:  item.resource.Unstructured.GetNamespace(),
			apiVersion: item.resource.Unstructured.GetAPIVersion(),
			name:       name.String(),
			ready:      string(item.resource.GetCondition(xpv1.TypeReady).Status),
			synced:     string(item.resource.GetCondition(xpv1.TypeSynced).Status),
		}
		if e := item.resource.LatestEvent; e != nil {
			row.latestEvent = fmt.Sprintf("[%s] %s", e.Type, e.Message)
		}
		if _, err := fmt.Fprintln(tw, row.String()); err != nil {
			return errors.Errorf(errFmtCannotWriteRow, err)
		}

		// Enqueue the children of the current node
		for idx := len(item.resource.Children) - 1; idx >= 0; idx-- {
			isLast := idx == len(item.resource.Children)-1
			queue = append([]*queueItem{{resource: item.resource.Children[idx], depth: item.depth + 1, isLast: isLast, prefix: childPrefix}}, queue...)
		}
	}
	if err := tw.Flush(); err != nil {
		return errors.Errorf(errFmtCannotFlushTabWriter, err)
	}

	return nil
}
