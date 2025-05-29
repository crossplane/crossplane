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

// Package usage finds usages.
package usage

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	legacy "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/apis/protection/v1beta1"
	"github.com/crossplane/crossplane/internal/protection"
)

// indexKey is a controller-runtime cache index key. It's used to index usages
// by the 'of' resource - the resource being used. This allows us to quickly
// determine whether a usage of a resource exists.
const indexKey = "inuse.apiversion.kind.name"

// indexVal returns a controller-runtime cache index value. It's used to index
// usages by the 'of' resource - the resource being used. This allows us to
// quickly determine whether a usage of a resource exists. The supplied
// apiVersion, kind, and name should represent the resource being used.
func indexVal(apiVersion, kind, name, namespace string) string {
	// There are two sources for "apiVersion" input, one is from the
	// unstructured objects fetched from k8s and the other is from the Usage
	// spec. The one from the objects from k8s is already validated by the k8s
	// API server, so we don't need to validate it again. The one from the Usage
	// spec is validated by the Usage controller, so we don't need to validate
	// it as well. So we can safely ignore the error here. Another reason to
	// ignore the error is that the IndexerFunc using this value to index the
	// objects does not return an error, so we cannot bubble up the error here.
	gr, _ := schema.ParseGroupVersion(apiVersion)
	return fmt.Sprintf("%s.%s.%s.%s", gr.Group, kind, name, namespace)
}

// A Finder finds all usages of a resource. It supports all known types of usage
// that satisfy the protection.Usage interface.
type Finder struct {
	client client.Reader
}

// NewFinder returns a new usage finder. The supplied Reader must be indexed by
// the supplied FieldIndexer. NewFinder adds indexes to the FieldIndexer; this
// means it can only be called once per FieldIndexer.
func NewFinder(r client.Reader, fi client.FieldIndexer) (*Finder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := fi.IndexField(ctx, &v1beta1.Usage{}, indexKey, func(obj client.Object) []string {
		u := obj.(*v1beta1.Usage) //nolint:forcetypeassert // Will always be a Usage.
		if u.Spec.Of.ResourceRef == nil || u.Spec.Of.ResourceRef.Name == "" {
			return []string{}
		}
		return []string{indexVal(u.Spec.Of.APIVersion, u.Spec.Of.Kind, u.Spec.Of.ResourceRef.Name, ptr.Deref(u.Spec.Of.ResourceRef.Namespace, u.GetNamespace()))}
	}); err != nil {
		return nil, err
	}

	if err := fi.IndexField(ctx, &v1beta1.ClusterUsage{}, indexKey, func(obj client.Object) []string {
		u := obj.(*v1beta1.ClusterUsage) //nolint:forcetypeassert // Will always be a ClusterUsage.
		if u.Spec.Of.ResourceRef == nil || u.Spec.Of.ResourceRef.Name == "" {
			return []string{}
		}
		return []string{indexVal(u.Spec.Of.APIVersion, u.Spec.Of.Kind, u.Spec.Of.ResourceRef.Name, "")}
	}); err != nil {
		return nil, err
	}

	//nolint:staticcheck // Usage is deprecated but we still need to support it.
	if err := fi.IndexField(context.Background(), &legacy.Usage{}, indexKey, func(obj client.Object) []string {
		u := obj.(*legacy.Usage) //nolint:forcetypeassert,staticcheck // This'll always be a Usage. Which, as above, is deprecated.
		if u.Spec.Of.ResourceRef == nil || u.Spec.Of.ResourceRef.Name == "" {
			return []string{}
		}
		return []string{indexVal(u.Spec.Of.APIVersion, u.Spec.Of.Kind, u.Spec.Of.ResourceRef.Name, "")}
	}); err != nil {
		return nil, err
	}

	return &Finder{client: r}, nil
}

// An Object to find usages of.
type Object interface {
	metav1.Object
	GetAPIVersion() string
	GetKind() string
}

// FindUsageOf the supplied object.
func (f *Finder) FindUsageOf(ctx context.Context, o Object) ([]protection.Usage, error) {
	usages := make([]protection.Usage, 0)

	ul := &v1beta1.UsageList{}
	if err := f.client.List(ctx, ul, client.MatchingFields{indexKey: indexVal(o.GetAPIVersion(), o.GetKind(), o.GetName(), o.GetNamespace())}); err != nil {
		return nil, errors.Wrapf(err, "cannot list %s", v1beta1.UsageGroupVersionKind)
	}
	for _, u := range ul.Items {
		usages = append(usages, &u)
	}

	cul := &v1beta1.ClusterUsageList{}
	if err := f.client.List(ctx, cul, client.MatchingFields{indexKey: indexVal(o.GetAPIVersion(), o.GetKind(), o.GetName(), "")}); err != nil {
		return nil, errors.Wrapf(err, "cannot list %s", v1beta1.ClusterUsageGroupVersionKind)
	}
	for _, u := range cul.Items {
		usages = append(usages, &u)
	}

	lul := &legacy.UsageList{} //nolint:staticcheck // It's deprecated but we still need to support it.
	if err := f.client.List(ctx, lul, client.MatchingFields{indexKey: indexVal(o.GetAPIVersion(), o.GetKind(), o.GetName(), "")}); err != nil {
		return nil, errors.Wrapf(err, "cannot list %s", legacy.UsageGroupVersionKind)
	}
	for _, u := range lul.Items {
		usages = append(usages, &u)
	}

	return usages, nil
}
