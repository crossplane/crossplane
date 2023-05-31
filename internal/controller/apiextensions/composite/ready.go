/*
Copyright 2022 The Crossplane Authors.

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

package composite

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Error strings
const (
	errInvalidCheck = "invalid"
	errPaveObject   = "cannot lookup field paths in supplied object"

	errFmtRequiresFieldPath       = "type %q requires a field path"
	errFmtRequiresMatchString     = "type %q requires a match string"
	errFmtRequiresMatchConditions = "type %q requires a valid match condition"
	errFmtRequiresMatchInteger    = "type %q requires a match integer"
	errFmtUnknownCheck            = "unknown type %q"
	errFmtRunCheck                = "cannot run readiness check at index %d"
)

// ReadinessCheckType is used for readiness check types.
type ReadinessCheckType string

// The possible values for readiness check type.
const (
	ReadinessCheckTypeNonEmpty     ReadinessCheckType = "NonEmpty"
	ReadinessCheckTypeMatchString  ReadinessCheckType = "MatchString"
	ReadinessCheckTypeMatchInteger ReadinessCheckType = "MatchInteger"
	// discussion regarding MatchBool vs MatchTrue/MatchFalse:
	// https://github.com/crossplane/crossplane/pull/4399#discussion_r1277225375
	ReadinessCheckTypeMatchTrue      ReadinessCheckType = "MatchTrue"
	ReadinessCheckTypeMatchFalse     ReadinessCheckType = "MatchFalse"
	ReadinessCheckTypeMatchCondition ReadinessCheckType = "MatchCondition"
	ReadinessCheckTypeNone           ReadinessCheckType = "None"
)

// ReadinessCheck is used to indicate how to tell whether a resource is ready
// for consumption
type ReadinessCheck struct {
	// Type indicates the type of probe you'd like to use.
	Type ReadinessCheckType

	// FieldPath shows the path of the field whose value will be used.
	FieldPath *string

	// MatchString is the value you'd like to match if you're using "MatchString" type.
	MatchString *string

	// MatchInt is the value you'd like to match if you're using "MatchInt" type.
	MatchInteger *int64

	// MatchCondition is the condition you'd like to match if you're using "MatchCondition" type.
	MatchCondition *MatchConditionReadinessCheck
}

// MatchConditionReadinessCheck is used to indicate how to tell whether a resource is ready
// for consumption
type MatchConditionReadinessCheck struct {
	// Type indicates the type of condition you'd like to use.
	Type xpv1.ConditionType

	// Status is the status of the condition you'd like to match.
	Status corev1.ConditionStatus
}

// ReadinessCheckFromV1 derives a ReadinessCheck from the supplied v1.ReadinessCheck.
func ReadinessCheckFromV1(in *v1.ReadinessCheck) ReadinessCheck {
	if in == nil {
		return ReadinessCheck{}
	}

	out := ReadinessCheck{
		Type: ReadinessCheckType(in.Type),
	}
	if in.FieldPath != "" {
		out.FieldPath = pointer.String(in.FieldPath)
	}

	// NOTE(negz): ComposedTemplate doesn't use pointer values for optional
	// strings, so today the empty string and 0 are equivalent to "unset".
	if in.MatchString != "" {
		out.MatchString = pointer.String(in.MatchString)
	}
	if in.MatchInteger != 0 {
		out.MatchInteger = pointer.Int64(in.MatchInteger)
	}
	if in.MatchCondition != nil {
		out.MatchCondition = &MatchConditionReadinessCheck{
			Type:   in.MatchCondition.Type,
			Status: in.MatchCondition.Status,
		}
	}
	return out
}

// ReadinessChecksFromComposedTemplate derives readiness checks from the supplied
// composed template.
func ReadinessChecksFromComposedTemplate(t *v1.ComposedTemplate) []ReadinessCheck {
	if t == nil {
		return nil
	}
	out := make([]ReadinessCheck, len(t.ReadinessChecks))
	for i := range t.ReadinessChecks {
		out[i] = ReadinessCheckFromV1(&t.ReadinessChecks[i])
	}
	return out
}

// TODO(negz): Ideally we'd validate P&T readiness checks (which are specified
// in the Composition) using a webhook. We still need to validate the output of
// a Composition Function Pipeline, though.

// Validate returns an error if the readiness check is invalid.
func (c ReadinessCheck) Validate() error {
	switch c.Type {
	case ReadinessCheckTypeNone:
		// This type has no dependencies.
		return nil
	case ReadinessCheckTypeNonEmpty, ReadinessCheckTypeMatchTrue, ReadinessCheckTypeMatchFalse:
		// This type only needs a field path.
	case ReadinessCheckTypeMatchString:
		if c.MatchString == nil {
			return errors.Errorf(errFmtRequiresMatchString, c.Type)
		}
	case ReadinessCheckTypeMatchInteger:
		if c.MatchInteger == nil {
			return errors.Errorf(errFmtRequiresMatchInteger, c.Type)
		}
	case ReadinessCheckTypeMatchCondition:
		if c.MatchCondition == nil {
			return errors.Errorf(errFmtRequiresMatchConditions, c.Type)
		}
		return nil
	default:
		return errors.Errorf(errFmtUnknownCheck, c.Type)
	}

	if c.FieldPath == nil {
		return errors.Errorf(errFmtRequiresFieldPath, c.Type)
	}

	return nil
}

// IsReady runs the readiness check against the supplied object.
//
//nolint:gocyclo // just a switch
func (c ReadinessCheck) IsReady(p *fieldpath.Paved, o ConditionedObject) (bool, error) {
	if err := c.Validate(); err != nil {
		return false, errors.Wrap(err, errInvalidCheck)
	}
	switch c.Type {
	case ReadinessCheckTypeNone:
		return true, nil
	case ReadinessCheckTypeNonEmpty:
		if _, err := p.GetValue(*c.FieldPath); err != nil {
			return false, resource.Ignore(fieldpath.IsNotFound, err)
		}
		return true, nil
	case ReadinessCheckTypeMatchString:
		val, err := p.GetString(*c.FieldPath)
		if err != nil {
			return false, resource.Ignore(fieldpath.IsNotFound, err)
		}
		return val == *c.MatchString, nil
	case ReadinessCheckTypeMatchInteger:
		val, err := p.GetInteger(*c.FieldPath)
		if err != nil {
			return false, resource.Ignore(fieldpath.IsNotFound, err)
		}
		return val == *c.MatchInteger, nil
	case ReadinessCheckTypeMatchCondition:
		val := o.GetCondition(c.MatchCondition.Type)
		return val.Status == c.MatchCondition.Status, nil
	case ReadinessCheckTypeMatchFalse:
		val, err := p.GetBool(*c.FieldPath)
		if err != nil {
			return false, resource.Ignore(fieldpath.IsNotFound, err)
		}
		return val == false, nil //nolint:gosimple // returning '!val' here as suggested hurts readability
	case ReadinessCheckTypeMatchTrue:
		val, err := p.GetBool(*c.FieldPath)
		if err != nil {
			return false, resource.Ignore(fieldpath.IsNotFound, err)
		}
		return val == true, nil //nolint:gosimple // returning 'val' here as suggested hurts readability
	}

	return false, nil
}

// A ReadinessChecker checks whether a composed resource is ready or not.
type ReadinessChecker interface {
	IsReady(ctx context.Context, o ConditionedObject, rc ...ReadinessCheck) (ready bool, err error)
}

// A ReadinessCheckerFn checks whether a composed resource is ready or not.
type ReadinessCheckerFn func(ctx context.Context, o ConditionedObject, rc ...ReadinessCheck) (ready bool, err error)

// IsReady reports whether a composed resource is ready or not.
func (fn ReadinessCheckerFn) IsReady(ctx context.Context, o ConditionedObject, rc ...ReadinessCheck) (ready bool, err error) {
	return fn(ctx, o, rc...)
}

// A ConditionedObject is a runtime object with conditions.
type ConditionedObject interface {
	resource.Object
	resource.Conditioned
}

// IsReady returns whether the composed resource is ready.
func IsReady(_ context.Context, o ConditionedObject, rc ...ReadinessCheck) (bool, error) {
	// kept as a safety net, but defaulting should ensure this is never hit
	if len(rc) == 0 {
		return resource.IsConditionTrue(o.GetCondition(xpv1.TypeReady)), nil
	}
	paved, err := fieldpath.PaveObject(o)
	if err != nil {
		return false, errors.Wrap(err, errPaveObject)
	}

	for i := range rc {
		ready, err := rc[i].IsReady(paved, o)
		if err != nil {
			return false, errors.Wrapf(err, errFmtRunCheck, i)
		}
		if !ready {
			return false, nil
		}
	}
	return true, nil
}
