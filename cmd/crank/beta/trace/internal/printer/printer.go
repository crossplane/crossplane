// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package printer contains the definition of the Printer interface and the
// implementation of all the available printers implementing it.
package printer

import (
	"io"

	"github.com/pkg/errors"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

const (
	errFmtUnknownPrinterType = "unknown printer output type: %s"
)

// Type represents the type of printer.
type Type string

// Implemented PrinterTypes
const (
	TypeDefault Type = "default"
	TypeWide    Type = "wide"
	TypeJSON    Type = "json"
	TypeDot     Type = "dot"
)

// Printer implements the interface which is used by all printers in this package.
type Printer interface {
	Print(io.Writer, *resource.Resource) error
}

// New creates a new printer based on the specified type.
func New(typeStr string) (Printer, error) {
	var p Printer

	switch Type(typeStr) {
	case TypeDefault:
		p = &DefaultPrinter{}
	case TypeWide:
		p = &DefaultPrinter{
			wide: true,
		}
	case TypeJSON:
		p = &JSONPrinter{}
	case TypeDot:
		p = &DotPrinter{}
	default:
		return nil, errors.Errorf(errFmtUnknownPrinterType, typeStr)
	}

	return p, nil
}
