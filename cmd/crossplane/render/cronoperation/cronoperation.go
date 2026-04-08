/*
Copyright 2025 The Crossplane Authors.

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

// Package cronoperation implements the
// 'crossplane internal render cronoperation' subcommand.
package cronoperation

import (
	"encoding/json"
	"io"
	"os"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/render/operation"
)

// Command produces the Operation a CronOperation would create.
type Command struct{}

// Run reads a CronOperationInput from stdin and writes an OperationOutput to
// stdout.
func (c *Command) Run(_ logging.Logger) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return errors.Wrap(err, "cannot read input")
	}

	in := &operation.CronOperationInput{}
	if err := json.Unmarshal(data, in); err != nil {
		return errors.Wrap(err, "cannot unmarshal render input")
	}

	op := operation.NewFromCronOperation(in)

	out := &operation.TemplateOutput{
		APIVersion: operation.APIVersion,
		Kind:       "OperationOutput",
		Operation:  *op,
	}

	outData, err := json.Marshal(out)
	if err != nil {
		return errors.Wrap(err, "cannot marshal render output")
	}

	_, err = os.Stdout.Write(outData)
	return errors.Wrap(err, "cannot write render output")
}
