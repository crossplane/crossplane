/*
Copyright 2019 The Crossplane Authors.

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

package claim

import (
	"context"
	"fmt"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	errUnsupportedClaimSpec = "composite resource claim spec was not an object"
	errUnsupportedDstObject = "destination object was not valid object"
	errUnsupportedSrcObject = "source object was not valid object"

	errMergeClaimSpec   = "unable to merge claim spec"
	errMergeClaimStatus = "unable to merge claim status"
)

// ConfigureComposite configures the supplied composite resource. The composite resource name
// is derived from the supplied claim, as {name}-{random-string}. The claim's
// external name annotation, if any, is propagated to the composite resource.
func ConfigureComposite(_ context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	// It's possible we're being asked to configure a statically provisioned
	// composite resource in which case we should respect its existing name and
	// external name.
	en := meta.GetExternalName(cp)
	if !meta.WasCreated(cp) {
		cp.SetGenerateName(fmt.Sprintf("%s-", cm.GetName()))
	}

	meta.AddAnnotations(cp, cm.GetAnnotations())
	meta.AddLabels(cp, cm.GetLabels())
	meta.AddLabels(cp, map[string]string{
		xcrd.LabelKeyClaimName:      cm.GetName(),
		xcrd.LabelKeyClaimNamespace: cm.GetNamespace(),
	})

	// If our composite resource already exists we want to restore its original
	// external name (even if that external name was empty) in order to ensure
	// we don't try to rename anything after the fact.
	if meta.WasCreated(cp) {
		// Fix(2353): do not introduce a superfluous extern-name
		// (empty external-names are treated as invalid)
		if en != "" {
			meta.SetExternalName(cp, en)
		}
	}

	ucm, ok := cm.(*claim.Unstructured)
	if !ok {
		return nil
	}
	ucp, ok := cp.(*composite.Unstructured)
	if !ok {
		return nil
	}

	icmSpec := ucm.Object["spec"]
	spec, ok := icmSpec.(map[string]interface{})
	if !ok {
		return errors.New(errUnsupportedClaimSpec)
	}

	// Delete base claim fields when configuring composite spec
	baseClaimSpec := xcrd.CompositeResourceClaimSpecProps()

	// keep any necessary fields between claim and composite
	for _, field := range xcrd.KeepClaimSpecProps {
		delete(baseClaimSpec, field)
	}
	claimSpecFilter := xcrd.GetPropFields(baseClaimSpec)
	ucp.Object["spec"] = filter(spec, claimSpecFilter...)
	return nil
}

func filter(in map[string]interface{}, keys ...string) map[string]interface{} {
	filter := map[string]bool{}
	for _, k := range keys {
		filter[k] = true
	}

	out := map[string]interface{}{}
	for k, v := range in {
		if filter[k] {
			continue
		}

		out[k] = v
	}
	return out
}

// APIClaimConfigurator configures the supplied claims with fields
// from the composite. This includes late-initializing spec values
// and updating status fields in claim.
type APIClaimConfigurator struct {
	client client.Client
}

// NewAPIClaimConfigurator returns a APIClaimConfigurator.
func NewAPIClaimConfigurator(client client.Client) *APIClaimConfigurator {
	return &APIClaimConfigurator{client: client}
}

// Configure the supplied claims with fields from the composite.
// This includes late-initializing spec values and updating status fields in claim.
func (c *APIClaimConfigurator) Configure(ctx context.Context, cr resource.CompositeClaim, cp resource.Composite) error {
	ucr, ok := cr.(*claim.Unstructured)
	if !ok {
		return nil
	}
	ucp, ok := cp.(*composite.Unstructured)
	if !ok {
		return nil
	}

	if err := merge(ucr.Object["status"], ucp.Object["status"],
		// Status fields from composite overwrite non-empty fields in claim
		withMergeOptions(mergo.WithOverride),
		withSrcFilter(xcrd.GetPropFields(xcrd.CompositeResourceStatusProps())...)); err != nil {
		return errors.Wrap(err, errMergeClaimStatus)
	}

	if err := c.client.Status().Update(ctx, cr); err != nil {
		return errors.Wrap(err, errUpdateClaimStatus)
	}

	if err := merge(ucr.Object["spec"], ucp.Object["spec"],
		withSrcFilter(xcrd.GetPropFields(xcrd.CompositeResourceSpecProps())...)); err != nil {
		return errors.Wrap(err, errMergeClaimSpec)
	}

	return errors.Wrap(c.client.Update(ctx, cr), errUpdateClaim)
}

type mergeConfig struct {
	mergeOptions []func(*mergo.Config)
	srcfilter    []string
}

// withMergeOptions allows custom mergo.Config options
func withMergeOptions(opts ...func(*mergo.Config)) func(*mergeConfig) {
	return func(config *mergeConfig) {
		config.mergeOptions = opts
	}
}

// withSrcFilter filters supplied keys from src map before merging
func withSrcFilter(keys ...string) func(*mergeConfig) {
	return func(config *mergeConfig) {
		config.srcfilter = keys
	}
}

// merge a src map into dst map
func merge(dst, src interface{}, opts ...func(*mergeConfig)) error {
	if dst == nil || src == nil {
		// Nothing available to merge if dst or src are nil.
		// This can occur early on in reconciliation when the
		// status subresource has not been set yet.
		return nil
	}

	config := &mergeConfig{}

	for _, opt := range opts {
		opt(config)
	}

	dstMap, ok := dst.(map[string]interface{})
	if !ok {
		return errors.New(errUnsupportedDstObject)
	}

	srcMap, ok := src.(map[string]interface{})
	if !ok {
		return errors.New(errUnsupportedSrcObject)
	}

	return mergo.Merge(&dstMap, filter(srcMap, config.srcfilter...), config.mergeOptions...)
}
