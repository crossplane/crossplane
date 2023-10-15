package printer

import (
	"io"

	"github.com/crossplane/crossplane/internal/k8s"
)

type Printer interface {
	Print(io.Writer, k8s.Resource, []string) error
}
