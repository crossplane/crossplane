// Package graph implements a multitude of printers to print out Resource structs. The Resource struct is implemented in the `k8s` package.
// Every printer should implement the Printer interface.
package graph

import (
	"io"
)

// Printer implements the interface which is used by all printers in this package.
type Printer interface {
	Print(io.Writer, Resource, []string) error
}
