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

	gcrname "github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource/xpkg"
)

const (
	errWriteHeader    = "cannot write header"
	errWriteRow       = "cannot write row"
	errFlushTabWriter = "cannot flush tab writer"
)

// DefaultPrinter defines the DefaultPrinter configuration
type DefaultPrinter struct {
	wide bool
}

var _ Printer = &DefaultPrinter{}

type defaultPrinterRow struct {
	name   string
	synced string
	ready  string
	status string
}

func (r *defaultPrinterRow) String() string {
	return strings.Join([]string{
		r.name,
		r.synced,
		r.ready,
		r.status,
	}, "\t")
}

type defaultPkgPrinterRow struct {
	wide bool
	// wide only fields
	// NOTE(phisco): just package is a reserved word
	packageImg string

	name      string
	version   string
	installed string
	healthy   string
	state     string
	status    string
}

func (r *defaultPkgPrinterRow) String() string {
	cols := []string{
		r.name,
	}
	if r.wide {
		cols = append(cols, r.packageImg)
	}
	cols = append(cols,
		r.version,
		r.installed,
		r.healthy,
		r.state,
		r.status,
	)
	return strings.Join(cols, "\t") + "\t"
}

func getHeaders(gk schema.GroupKind, wide bool) (headers fmt.Stringer, isPackageOrPackageRevision bool) {
	if xpkg.IsPackageType(gk) || xpkg.IsPackageRevisionType(gk) {
		return &defaultPkgPrinterRow{
			wide: wide,

			name:       "NAME",
			packageImg: "PACKAGE",
			version:    "VERSION",
			installed:  "INSTALLED",
			healthy:    "HEALTHY",
			state:      "STATE",
			status:     "STATUS",
		}, true
	}
	return &defaultPrinterRow{
		name:   "NAME",
		synced: "SYNCED",
		ready:  "READY",
		status: "STATUS",
	}, false

}

// Print implements the Printer interface by prints the resource tree in a
// human-readable format.
func (p *DefaultPrinter) Print(w io.Writer, root *resource.Resource) error {
	tw := printers.GetNewTabWriter(w)

	headers, isPackageOrRevision := getHeaders(root.Unstructured.GroupVersionKind().GroupKind(), p.wide)

	if _, err := fmt.Fprintln(tw, headers.String()); err != nil {
		return errors.Wrap(err, errWriteHeader)
	}

	type queueItem struct {
		resource *resource.Resource
		depth    int
		isLast   bool
		prefix   string
	}

	// Initialize LIFO queue with root element to traverse the tree depth-first,
	// enqueuing children in reverse order so that they are dequeued in the
	// right order w.r.t. the way they are defined by the resources.
	queue := []*queueItem{{resource: root}}

	for len(queue) > 0 {
		var item *queueItem
		l := len(queue)
		item, queue = queue[l-1], queue[:l-1] // Pop the last element

		// Build the name of the current node, prepending the required prefix to
		// show the tree structure
		name := strings.Builder{}
		childPrefix := item.prefix // Inherited prefix for all the children of the current node
		switch {
		case item.depth == 0:
			// We don't need a prefix for the root, nor a custom
			// prefix for its children
		case item.isLast:
			name.WriteString(item.prefix + "└─ ")
			childPrefix += "   "
		default:
			name.WriteString(item.prefix + "├─ ")
			childPrefix += "│  "
		}

		name.WriteString(fmt.Sprintf("%s/%s", item.resource.Unstructured.GetKind(), item.resource.Unstructured.GetName()))

		// Append the namespace if it's not empty
		if item.resource.Unstructured.GetNamespace() != "" {
			name.WriteString(fmt.Sprintf(" (%s)", item.resource.Unstructured.GetNamespace()))
		}

		var row fmt.Stringer
		if isPackageOrRevision {
			row = getPkgResourceStatus(item.resource, name.String(), p.wide)
		} else {
			row = getResourceStatus(item.resource, name.String(), p.wide)
		}

		if _, err := fmt.Fprintln(tw, row.String()); err != nil {
			return errors.Wrap(err, errWriteRow)
		}

		// Enqueue the children of the current node in reverse order to ensure
		// that they are dequeued from the LIFO queue in the same order w.r.t.
		// the way they are defined by the resources.
		for idx := len(item.resource.Children) - 1; idx >= 0; idx-- {
			isLast := idx == len(item.resource.Children)-1
			queue = append(queue, &queueItem{resource: item.resource.Children[idx], depth: item.depth + 1, isLast: isLast, prefix: childPrefix})
		}
	}

	if err := tw.Flush(); err != nil {
		return errors.Wrap(err, errFlushTabWriter)
	}

	return nil
}

// getResourceStatus returns a string that represents an entire row of status
// information for the resource.
func getResourceStatus(r *resource.Resource, name string, wide bool) fmt.Stringer { //nolint:gocyclo // NOTE(phisco): just a few switches, not much to do here
	readyCond := r.GetCondition(xpv1.TypeReady)
	syncedCond := r.GetCondition(xpv1.TypeSynced)
	var status, m string
	switch {
	case r.Error != nil:
		// if there is an error we want to show it
		status = "Error"
		m = r.Error.Error()
	case readyCond.Status == corev1.ConditionTrue && syncedCond.Status == corev1.ConditionTrue:
		// if both are true we want to show the ready reason only
		status = string(readyCond.Reason)

	// The following cases are for when one of the conditions is not true (false or unknown),
	// prioritizing synced over readiness in case of issues.
	case syncedCond.Status != corev1.ConditionTrue &&
		(syncedCond.Reason != "" || syncedCond.Message != ""):
		status = string(syncedCond.Reason)
		m = syncedCond.Message
	case readyCond.Status != corev1.ConditionTrue &&
		(readyCond.Reason != "" || readyCond.Message != ""):
		status = string(readyCond.Reason)
		m = readyCond.Message

	default:
		// both are unknown or unset, let's try showing the ready reason, probably empty
		status = string(readyCond.Reason)
		m = readyCond.Message
	}

	// Crop the message to the last 64 characters if it's too long and we are
	// not in wide mode
	if !wide && len(m) > 64 {
		m = "..." + m[len(m)-64:]
	}

	// Append the message to the status if it's not empty
	if m != "" {
		status = fmt.Sprintf("%s: %s", status, m)
	}

	return &defaultPrinterRow{
		name:   name,
		ready:  mapEmptyStatusToDash(readyCond.Status),
		synced: mapEmptyStatusToDash(syncedCond.Status),
		status: status,
	}
}

func getPkgResourceStatus(r *resource.Resource, name string, wide bool) fmt.Stringer { //nolint:gocyclo // TODO: just a few switches, not much to do here
	var err error
	var packageImg, state, status, m string

	healthyCond := r.GetCondition(pkgv1.TypeHealthy)
	installedCond := r.GetCondition(pkgv1.TypeInstalled)

	gk := r.Unstructured.GroupVersionKind().GroupKind()
	switch {
	case r.Error != nil:
		// If there is an error we want to show it, regardless of what type this
		// resource is and what conditions it has.
		status = "Error"
		m = r.Error.Error()
	case xpkg.IsPackageType(gk):
		switch {
		case healthyCond.Status == corev1.ConditionTrue && installedCond.Status == corev1.ConditionTrue:
			// If both are true we want to show the healthy reason only
			status = string(healthyCond.Reason)

		// The following cases are for when one of the conditions is not true (false or unknown),
		// prioritizing installed over healthy in case of issues.
		case installedCond.Status != corev1.ConditionTrue &&
			(installedCond.Reason != "" || installedCond.Message != ""):
			status = string(installedCond.Reason)
			m = installedCond.Message
		case healthyCond.Status != corev1.ConditionTrue &&
			(healthyCond.Reason != "" || healthyCond.Message != ""):
			status = string(healthyCond.Reason)
			m = healthyCond.Message
		default:
			// both are unknown or unset, let's try showing the installed reason
			status = string(installedCond.Reason)
			m = installedCond.Message
		}

		if packageImg, err = fieldpath.Pave(r.Unstructured.Object).GetString("spec.package"); err != nil {
			state = err.Error()
		}
	case xpkg.IsPackageRevisionType(gk):
		// package revisions only have the healthy condition, so use that
		status = string(healthyCond.Reason)
		m = healthyCond.Message

		// Get the state (active vs. inactive) of this package revision.
		var err error
		state, err = fieldpath.Pave(r.Unstructured.Object).GetString("spec.desiredState")
		if err != nil {
			state = err.Error()
		}
		// Get the image used.
		if packageImg, err = fieldpath.Pave(r.Unstructured.Object).GetString("spec.image"); err != nil {
			state = err.Error()
		}
	case xpkg.IsPackageRuntimeConfigType(gk):
		// nothing to do here
	default:
		status = "Unknown package type"
	}

	// Crop the message to the last 64 characters if it's too long and we are
	// not in wide mode
	if !wide && len(m) > 64 {
		m = "..." + m[len(m)-64:]
	}

	// Append the message to the status if it's not empty
	if m != "" {
		status = fmt.Sprintf("%s: %s", status, m)
	}

	// Parse the image reference extracting the tag, we'll leave it empty if we
	// couldn't parse it and leave the whole thing as package instead. We pass
	// an empty default registry here so the displayed package image will be
	// unmodified from what we found in the spec, similar to how kubectl output
	// behaves.
	var packageImgTag string
	if tag, err := gcrname.NewTag(packageImg, gcrname.WithDefaultRegistry("")); err == nil {
		packageImgTag = tag.TagStr()
		packageImg = tag.RepositoryStr()
		if tag.RegistryStr() != "" {
			packageImg = fmt.Sprintf("%s/%s", tag.RegistryStr(), packageImg)
		}
	}

	return &defaultPkgPrinterRow{
		wide: wide,

		name:       name,
		packageImg: packageImg,
		version:    packageImgTag,
		installed:  mapEmptyStatusToDash(installedCond.Status),
		healthy:    mapEmptyStatusToDash(healthyCond.Status),
		state:      mapEmptyStatusToDash(corev1.ConditionStatus(state)),
		status:     status,
	}
}

func mapEmptyStatusToDash(s corev1.ConditionStatus) string {
	if s == "" {
		return "-"
	}
	return string(s)
}
