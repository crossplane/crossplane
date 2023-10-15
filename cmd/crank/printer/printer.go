// Package printer implements a multitude of printers to print out Resource structs. The Resource struct is implemented in the `k8s` package.
// Every printer should implement the Printer interface.
package printer

import (
	"io"

	"github.com/crossplane/crossplane/internal/k8s"
)

// Printer implements the interface which is used by all printers in this package.
type Printer interface {
	Print(io.Writer, k8s.Resource, []string) error
}
