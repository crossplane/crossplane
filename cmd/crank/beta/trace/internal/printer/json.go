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
type JSONPrinter struct{}

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
