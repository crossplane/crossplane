/*
Copyright 2018 The Crossplane Authors.

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

package test

import (
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Bucket builder for testing purposes
type Bucket struct {
	*v1alpha1.Bucket
}

// NewBucket builder object
func NewBucket(ns, name string) *Bucket {
	return (&Bucket{Bucket: &v1alpha1.Bucket{}}).WithObjectMetaNames(ns, name)
}

// WithObjectMeta sets objects metadata value
func (b *Bucket) WithObjectMeta(meta metav1.ObjectMeta) *Bucket {
	b.Bucket.ObjectMeta = meta
	return b
}

// WithObjectMetaNames sets object namespace and name values
func (b *Bucket) WithObjectMetaNames(ns, name string) *Bucket {
	b.Bucket.Namespace = ns
	b.Bucket.Name = name
	return b
}

// WithObjectMetaUID sets object's metadata UID value
func (b *Bucket) WithObjectMetaUID(uid string) *Bucket {
	b.Bucket.UID = types.UID(uid)
	return b
}

// WithSpecName sets object's spec name value
func (b *Bucket) WithSpecName(name string) *Bucket {
	b.Bucket.Spec.Name = name
	return b
}

// NewBucketClaimReference new claim reference
func NewBucketClaimReference(ns, name, uid string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.BucketKind,
		Namespace:  ns,
		Name:       name,
		UID:        types.UID(uid),
	}
}

// NewBucketClassReference new class reference
func NewBucketClassReference(ns, name string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: corev1alpha1.APIVersion,
		Kind:       corev1alpha1.ResourceClassKind,
		Namespace:  ns,
		Name:       name,
	}
}
