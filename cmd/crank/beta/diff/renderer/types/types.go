// Package types provides types used in the renderer in order to facilitate code reuse in test
package types

import (
	"fmt"

	"github.com/sergi/go-diff/diffmatchpatch"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceDiff represents the diff for a specific resource.
type ResourceDiff struct {
	Gvk          schema.GroupVersionKind
	ResourceName string
	DiffType     DiffType
	LineDiffs    []diffmatchpatch.Diff
	Current      *un.Unstructured // Optional, for reference
	Desired      *un.Unstructured // Optional, for reference
}

// DiffType represents the type of diff (added, removed, modified).
type DiffType string

const (
	// DiffTypeAdded an added section.
	DiffTypeAdded DiffType = "+"
	// DiffTypeRemoved a removed section.
	DiffTypeRemoved DiffType = "-"
	// DiffTypeModified a modified section.
	DiffTypeModified DiffType = "~"
	// DiffTypeEqual an unchanged section.
	DiffTypeEqual DiffType = "="
)

// Colors for terminal output.
const (
	// ColorRed an ANSI "begin red" character.
	ColorRed = "\x1b[31m"
	// ColorGreen an ANSI "begin green" character.
	ColorGreen = "\x1b[32m"
	// ColorReset an ANSI "reset color" character.
	ColorReset = "\x1b[0m"
)

// GetDiffKey returns a key that can be used to identify this object for use in a map.
func (d *ResourceDiff) GetDiffKey() string {
	return MakeDiffKey(d.Gvk.Group+"/"+d.Gvk.Version, d.Gvk.Kind, d.ResourceName)
}

// MakeDiffKey creates a unique key for a resource diff.
func MakeDiffKey(apiVersion, kind, name string) string {
	return fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
}
