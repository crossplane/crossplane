// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package printer

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/pkg/errors"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

const (
	errCannotMarshalJSON = "cannot marshal resource graph as JSON"
)

// JSONPrinter is a printer that prints the resource graph as JSON.
type JSONPrinter struct {
}

var _ Printer = &JSONPrinter{}

// Print implements the Printer interface.
func (p *JSONPrinter) Print(w io.Writer, root *resource.Resource) error {
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return errors.Wrap(err, errCannotMarshalJSON)
	}
	_, err = fmt.Fprintln(w, string(out))
	return err
}
