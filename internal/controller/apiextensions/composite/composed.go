/*
Copyright 2020 The Crossplane Authors.

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
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings
const (
	errApply          = "cannot apply composed resource"
	errFetchSecret    = "cannot fetch connection secret"
	errReadiness      = "cannot check whether composed resource is ready"
	errUnmarshal      = "cannot unmarshal base template"
	errFmtPatch       = "cannot apply the patch at index %d"
	errGetSecret      = "cannot get connection secret of composed resource"
	errNamePrefix     = "name prefix is not found in labels"
	errName           = "cannot use dry-run create to name composed resource"
	errConnDetailKey  = "connection detail of type %s key is not set"
	errConnDetailVal  = "connection detail of type %s value is not set"
	errConnDetailPath = "connection detail of type %s fromFieldPath is not set"
)

// Observation is the result of composed reconciliation.
type Observation struct {
	Ref               corev1.ObjectReference
	ConnectionDetails managed.ConnectionDetails
	Ready             bool
}

// A RenderFn renders the supplied composed resource.
type RenderFn func(cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error

// Render calls RenderFn.
func (c RenderFn) Render(cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
	return c(cp, cd, t)
}

// An APIDryRunRenderer renders composed resources. It may perform a dry-run
// create against an API server in order to name and validate the rendered
// resource.
type APIDryRunRenderer struct {
	client client.Client
}

// NewAPIDryRunRenderer returns a Renderer of composed resources that may
// perform a dry-run create against an API server in order to name and validate
// it.
func NewAPIDryRunRenderer(c client.Client) *APIDryRunRenderer {
	return &APIDryRunRenderer{client: c}
}

// Render the supplied composed resource using the supplied composite resource
// and template. The rendered resource may be submitted to an API server via a
// dry run create in order to name and validate it.
func (r *APIDryRunRenderer) Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
	// Any existing name will be overwritten when we unmarshal the template. We
	// store it here so that we can reset it after unmarshalling.
	name := cd.GetName()
	namespace := cd.GetNamespace()
	if err := json.Unmarshal(t.Base.Raw, cd); err != nil {
		return errors.Wrap(err, errUnmarshal)
	}
	if cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] == "" {
		return errors.New(errNamePrefix)
	}
	// This label will be used if composed resource is yet another composite.
	meta.AddLabels(cd, map[string]string{
		xcrd.LabelKeyNamePrefixForComposed: cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed],
		xcrd.LabelKeyClaimName:             cp.GetLabels()[xcrd.LabelKeyClaimName],
		xcrd.LabelKeyClaimNamespace:        cp.GetLabels()[xcrd.LabelKeyClaimNamespace],
	})
	// Unmarshalling the template will overwrite any existing fields, so we must
	// restore the existing name, if any. We also set generate name in case we
	// haven't yet named this composed resource.
	cd.SetGenerateName(cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] + "-")
	cd.SetName(name)
	cd.SetNamespace(namespace)
	for i, p := range t.Patches {
		if err := p.Apply(cp, cd); err != nil {
			return errors.Wrapf(err, errFmtPatch, i)
		}
	}

	// We do this last to ensure that a Composition cannot influence owner (and
	// especially controller) references.
	or := meta.AsController(meta.TypedReferenceTo(cp, cp.GetObjectKind().GroupVersionKind()))
	cd.SetOwnerReferences([]metav1.OwnerReference{or})

	// We don't want to dry-run create a resource that can't be named by the API
	// server due to a missing generate name. We also don't want to create one
	// that is already named, because doing so will result in an error. The API
	// server seems to respond with a 500 ServerTimeout error for all dry-run
	// failures, so we can't just perform a dry-run and ignore 409 Conflicts for
	// resources that are already named.
	if cd.GetName() != "" || cd.GetGenerateName() == "" {
		return nil
	}

	// The API server returns an available name derived from generateName when
	// we perform a dry-run create. This name is likely (but not guaranteed) to
	// be available when we create the composed resource. If the API server
	// generates a name that is unavailable it will return a 500 ServerTimeout
	// error.
	return errors.Wrap(r.client.Create(ctx, cd, client.DryRunAll), errName)
}

// RenderComposite renders the supplied composite resource using the supplied composed
// resource and template.
func RenderComposite(_ context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
	onlyPatches := []v1.PatchType{v1.PatchTypeToCompositeFieldPath}
	for i, p := range t.Patches {
		if err := p.Apply(cp, cd, onlyPatches...); err != nil {
			return errors.Wrapf(err, errFmtPatch, i)
		}
	}

	return nil
}

// An APIConnectionDetailsFetcher may use the API server to read connection
// details from a Secret.
type APIConnectionDetailsFetcher struct {
	client client.Client
}

// NewAPIConnectionDetailsFetcher returns a ConnectionDetailsFetcher that may
// use the API server to read connection details from a Secret.
func NewAPIConnectionDetailsFetcher(c client.Client) *APIConnectionDetailsFetcher {
	return &APIConnectionDetailsFetcher{client: c}
}

// FetchConnectionDetails of the supplied composed resource, if any.
func (cdf *APIConnectionDetailsFetcher) FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) { // nolint:gocyclo
	data := map[string][]byte{}
	if sref := cd.GetWriteConnectionSecretToReference(); sref != nil {
		// It's possible that the composed resource does want to write a
		// connection secret but has not yet. We presume this isn't an issue and
		// that we'll propagate any connection details during a future
		// iteration.
		s := &corev1.Secret{}
		nn := types.NamespacedName{Namespace: sref.Namespace, Name: sref.Name}
		if err := cdf.client.Get(ctx, nn, s); client.IgnoreNotFound(err) != nil {
			return nil, errors.Wrap(err, errGetSecret)
		}
		data = s.Data
	}

	conn := managed.ConnectionDetails{}

	for _, d := range t.ConnectionDetails {
		switch d.Type {
		case v1.ConnectionDetailValue:
			// Name, Value must be set if value type
			switch {
			case d.Name == nil:
				return nil, fmt.Errorf(errConnDetailKey, d.Type)
			case d.Value == nil:
				return nil, fmt.Errorf(errConnDetailVal, d.Type)
			default:
				conn[*d.Name] = []byte(*d.Value)
			}
		case v1.ConnectionDetailFromConnectionSecretKey:
			if d.FromConnectionSecretKey == nil {
				return nil, fmt.Errorf(errConnDetailKey, d.Type)
			}
			if data[*d.FromConnectionSecretKey] == nil {
				// We don't consider this an error because it's possible the
				// key will still be written at some point in the future.
				continue
			}
			key := *d.FromConnectionSecretKey
			if d.Name != nil {
				key = *d.Name
			}
			if key != "" {
				conn[key] = data[*d.FromConnectionSecretKey]
			}
		case v1.ConnectionDetailFromFieldPath:
			switch {
			case d.Name == nil:
				return nil, fmt.Errorf(errConnDetailKey, d.Type)
			case d.FromFieldPath == nil:
				return nil, fmt.Errorf(errConnDetailPath, d.Type)
			default:
				_ = extractFieldPathValue(cd, d, conn)
			}
		}
	}

	if len(conn) == 0 {
		return nil, nil
	}

	return conn, nil
}

func extractFieldPathValue(from runtime.Object, detail v1.ConnectionDetail, conn managed.ConnectionDetails) error {
	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return err
	}

	str, err := fieldpath.Pave(fromMap).GetString(*detail.FromFieldPath)
	if err == nil {
		conn[*detail.Name] = []byte(str)
		return nil
	}

	in, err := fieldpath.Pave(fromMap).GetValue(*detail.FromFieldPath)
	if err != nil {
		return err
	}

	buffer, err := json.Marshal(in)
	if err != nil {
		return err
	}
	conn[*detail.Name] = buffer
	return nil
}

// IsReady returns whether the composed resource is ready.
func IsReady(_ context.Context, cd resource.Composed, t v1.ComposedTemplate) (bool, error) { // nolint:gocyclo
	// NOTE(muvaf): The cyclomatic complexity of this function comes from the
	// mandatory repetitiveness of the switch clause, which is not really complex
	// in reality. Though beware of adding additional complexity besides that.

	if len(t.ReadinessChecks) == 0 {
		return resource.IsConditionTrue(cd.GetCondition(xpv1.TypeReady)), nil
	}
	// TODO(muvaf): We can probably get rid of resource.Composed interface and fake.Composed
	// structs and use *composed.Unstructured everywhere including tests.
	u, ok := cd.(*composed.Unstructured)
	if !ok {
		return false, errors.New("composed resource has to be Unstructured type")
	}
	paved := fieldpath.Pave(u.UnstructuredContent())

	for i, check := range t.ReadinessChecks {
		var ready bool
		switch check.Type {
		case v1.ReadinessCheckNone:
			return true, nil
		case v1.ReadinessCheckNonEmpty:
			_, err := paved.GetValue(check.FieldPath)
			if resource.Ignore(fieldpath.IsNotFound, err) != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err)
		case v1.ReadinessCheckMatchString:
			val, err := paved.GetString(check.FieldPath)
			if resource.Ignore(fieldpath.IsNotFound, err) != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err) && val == check.MatchString
		case v1.ReadinessCheckMatchInteger:
			val, err := paved.GetInteger(check.FieldPath)
			if err != nil {
				return false, err
			}
			ready = !fieldpath.IsNotFound(err) && val == check.MatchInteger
		default:
			return false, errors.New(fmt.Sprintf("readiness check at index %d: an unknown type is chosen", i))
		}
		if !ready {
			return false, nil
		}
	}
	return true, nil
}
