// Package graph implements a multitude of printers to print out Resource structs. The Resource struct is implemented in the `k8s` package.
// Every printer should implement the Printer interface.
package graph

import (
	"io"

	"github.com/pkg/errors"
)

// PrinterType represents the type of printer.
type PrinterType string

// Implemented PrinterTypes
const (
	TypeTree  PrinterType = "tree"
	TypeTable PrinterType = "table"
	TypeGraph PrinterType = "graph"
)

// Printer implements the interface which is used by all printers in this package.
type Printer interface {
	Print(io.Writer, Resource, []string) error
}

// NewPrinter creates a new printer based on the specified type.
func NewPrinter(typeStr string) (Printer, error) {
	var p Printer

	switch PrinterType(typeStr) {
	case TypeTree:
		p = &Tree{
			Indent: "",
			IsLast: true,
		}
	case TypeTable:
		p = &Table{}
	case TypeGraph:
		p = &DotGraph{}
	default:
		return nil, errors.Errorf("unknown printer output type: %s", typeStr)
	}

	return p, nil
}
