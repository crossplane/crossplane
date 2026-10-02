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

// Package watchcondition evaluates WatchOperation CEL watch conditions.
package watchcondition

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
)

const (
	varObject  = "object"
	varDeleted = "deleted"
)

// A Program is a compiled set of WatchOperation watch conditions.
type Program struct {
	hasOnChange bool
	onChange  cel.Program
	when      []cel.Program
}

// Compile builds a Program from a WatchSpec.
func Compile(watch v1alpha1.WatchSpec) (*Program, error) {
	env, err := cel.NewEnv(
		cel.Variable(varObject, cel.DynType),
		cel.Variable(varDeleted, cel.BoolType),
	)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create CEL environment")
	}

	p := &Program{}

	if watch.OnChange != nil {
		prg, err := compileExpression(env, watch.OnChange.Expression)
		if err != nil {
			return nil, errors.Wrap(err, "cannot compile onChange expression")
		}
		p.hasOnChange = true
		p.onChange = prg
	}

	for _, c := range watch.When {
		prg, err := compileBoolExpression(env, c.Expression)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot compile when expression %q", c.Name)
		}
		p.when = append(p.when, prg)
	}

	return p, nil
}

// HasOnChange returns true when the program tracks onChange fingerprints.
func (p *Program) HasOnChange() bool {
	return p.hasOnChange
}

// ShouldReport reports whether an Operation should be created for the watched
// object. previousFingerprint is the last recorded onChange fingerprint for
// this resource, or empty if none exists. When no onChange expression is
// configured, fingerprint is empty and any resourceVersion change may trigger
// an Operation subject to when conditions.
func (p *Program) ShouldReport(obj *unstructured.Unstructured, deleted bool, previousFingerprint string) (report bool, fingerprint string, err error) {
	if p == nil {
		return true, "", nil
	}

	vars := activation(obj, deleted)

	for _, prg := range p.when {
		ok, err := evalBool(prg, vars)
		if err != nil {
			return false, "", err
		}
		if !ok {
			return false, "", nil
		}
	}

	if !p.hasOnChange {
		return true, "", nil
	}

	fp, err := evalFingerprint(p.onChange, vars)
	if err != nil {
		return false, "", err
	}

	if fp == previousFingerprint {
		return false, fp, nil
	}

	return true, fp, nil
}

func compileExpression(env *cel.Env, expression string) (cel.Program, error) {
	ast, iss := env.Compile(expression)
	if err := iss.Err(); err != nil {
		return nil, err
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, err
	}

	return prg, nil
}

func compileBoolExpression(env *cel.Env, expression string) (cel.Program, error) {
	ast, iss := env.Compile(expression)
	if err := iss.Err(); err != nil {
		return nil, err
	}

	if !ast.OutputType().IsExactType(types.BoolType) {
		return nil, fmt.Errorf("expression must evaluate to bool, got %v", ast.OutputType())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, err
	}

	return prg, nil
}

func activation(obj *unstructured.Unstructured, deleted bool) map[string]any {
	object := obj.Object
	if object == nil {
		object = map[string]any{}
	}

	return map[string]any{
		varObject:  object,
		varDeleted: deleted,
	}
}

func evalBool(prg cel.Program, vars map[string]any) (bool, error) {
	out, _, err := prg.Eval(vars)
	if err != nil {
		return false, errors.Wrap(err, "cannot evaluate CEL expression")
	}

	b, err := out.ConvertToNative(reflect.TypeFor[bool]())
	if err != nil {
		return false, errors.Wrap(err, "CEL expression must evaluate to bool")
	}

	return b.(bool), nil
}

func evalFingerprint(prg cel.Program, vars map[string]any) (string, error) {
	out, _, err := prg.Eval(vars)
	if err != nil {
		return "", errors.Wrap(err, "cannot evaluate onChange expression")
	}

	native, err := out.ConvertToNative(reflect.TypeFor[any]())
	if err != nil {
		return "", errors.Wrap(err, "cannot convert onChange expression result")
	}

	b, err := json.Marshal(native)
	if err != nil {
		return "", errors.Wrap(err, "cannot fingerprint onChange expression result")
	}

	return string(b), nil
}
