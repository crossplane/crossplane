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

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane/v2/cmd/crank/common/resource"
)

const (
	errCannotMarshalYAML = "cannot marshal resource graph as YAML"
)

// YAMLPrinter is a printer that prints the resource graph as YAML.
type YAMLPrinter struct{}

var _ Printer = &YAMLPrinter{}

// Print implements the Printer interface.
func (p *YAMLPrinter) Print(w io.Writer, root *resource.Resource) error {
	out, err := yaml.Marshal(root)
	if err != nil {
		return errors.Wrap(err, errCannotMarshalYAML)
	}

	_, err = fmt.Fprintln(w, string(out))

	return err
}

// PrintList implements the Printer interface.
func (p *YAMLPrinter) PrintList(w io.Writer, roots *resource.ResourceList) error {
	if roots == nil {
		roots = &resource.ResourceList{}
	}
	if roots.Items == nil {
		roots.Items = []*resource.Resource{}
	}

	out, err := yaml.Marshal(roots)
	if err != nil {
		return errors.Wrap(err, errCannotMarshalYAML)
	}
	_, err = fmt.Fprintln(w, string(out))
	return err
}
