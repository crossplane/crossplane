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

// Package fake provides fake Crossplane resources for use in tests.
//
//nolint:musttag // We only use JSON to round-trip convert these mocks.
package fake

import (
	"encoding/json"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"
)

// Conditioned is a mock that implements Conditioned interface.
type Conditioned struct{ Conditions []xpv1.Condition }

// SetConditions sets the Conditions.
func (m *Conditioned) SetConditions(c ...xpv1.Condition) { m.Conditions = c }

// GetCondition get the Condition with the given ConditionType.
func (m *Conditioned) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return xpv1.Condition{Type: ct, Status: corev1.ConditionUnknown}
}

// ClaimReferencer is a mock that implements ClaimReferencer interface.
type ClaimReferencer struct{ Ref *reference.Claim }

// SetClaimReference sets the ClaimReference.
func (m *ClaimReferencer) SetClaimReference(r *reference.Claim) { m.Ref = r }

// GetClaimReference gets the ClaimReference.
func (m *ClaimReferencer) GetClaimReference() *reference.Claim { return m.Ref }

// ManagedResourceReferencer is a mock that implements ManagedResourceReferencer interface.
type ManagedResourceReferencer struct{ Ref *corev1.ObjectReference }

// SetResourceReference sets the ResourceReference.
func (m *ManagedResourceReferencer) SetResourceReference(r *corev1.ObjectReference) { m.Ref = r }

// GetResourceReference gets the ResourceReference.
func (m *ManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference { return m.Ref }

// TypedProviderConfigReferencer is a mock that implements resource.TypedProviderConfigReferencer interface.
type TypedProviderConfigReferencer struct{ Ref *xpv1.ProviderConfigReference }

// SetProviderConfigReference sets the ProviderConfigReference.
func (m *TypedProviderConfigReferencer) SetProviderConfigReference(p *xpv1.ProviderConfigReference) {
	m.Ref = p
}

// GetProviderConfigReference gets the ProviderConfigReference.
func (m *TypedProviderConfigReferencer) GetProviderConfigReference() *xpv1.ProviderConfigReference {
	return m.Ref
}

// LegacyProviderConfigReferencer is a mock that implements resource.ProviderConfigReferencer interface.
type LegacyProviderConfigReferencer struct{ Ref *xpv1.Reference }

// SetProviderConfigReference sets the ProviderConfigReference.
func (m *LegacyProviderConfigReferencer) SetProviderConfigReference(p *xpv1.Reference) { m.Ref = p }

// GetProviderConfigReference gets the ProviderConfigReference.
func (m *LegacyProviderConfigReferencer) GetProviderConfigReference() *xpv1.Reference {
	return m.Ref
}

// RequiredProviderConfigReferencer is a mock that implements the
// RequiredProviderConfigReferencer interface.
type RequiredProviderConfigReferencer struct{ Ref xpv1.Reference }

// SetProviderConfigReference sets the ProviderConfigReference.
func (m *RequiredProviderConfigReferencer) SetProviderConfigReference(p xpv1.Reference) {
	m.Ref = p
}

// GetProviderConfigReference gets the ProviderConfigReference.
func (m *RequiredProviderConfigReferencer) GetProviderConfigReference() xpv1.Reference {
	return m.Ref
}

// RequiredTypedProviderConfigReferencer is a mock that implements the
// RequiredTypedProviderConfigReferencer interface.
type RequiredTypedProviderConfigReferencer struct{ Ref xpv1.ProviderConfigReference }

// SetProviderConfigReference sets the ProviderConfigReference.
func (m *RequiredTypedProviderConfigReferencer) SetProviderConfigReference(p xpv1.ProviderConfigReference) {
	m.Ref = p
}

// GetProviderConfigReference gets the ProviderConfigReference.
func (m *RequiredTypedProviderConfigReferencer) GetProviderConfigReference() xpv1.ProviderConfigReference {
	return m.Ref
}

// RequiredTypedResourceReferencer is a mock that implements the
// RequiredTypedResourceReferencer interface.
type RequiredTypedResourceReferencer struct{ Ref xpv1.TypedReference }

// SetResourceReference sets the ResourceReference.
func (m *RequiredTypedResourceReferencer) SetResourceReference(p xpv1.TypedReference) {
	m.Ref = p
}

// GetResourceReference gets the ResourceReference.
func (m *RequiredTypedResourceReferencer) GetResourceReference() xpv1.TypedReference {
	return m.Ref
}

// LocalConnectionSecretWriterTo is a mock that implements LocalConnectionSecretWriterTo interface.
type LocalConnectionSecretWriterTo struct {
	Ref *xpv1.LocalSecretReference
}

// SetWriteConnectionSecretToReference sets the WriteConnectionSecretToReference.
func (m *LocalConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	m.Ref = r
}

// GetWriteConnectionSecretToReference gets the WriteConnectionSecretToReference.
func (m *LocalConnectionSecretWriterTo) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return m.Ref
}

// ConnectionSecretWriterTo is a mock that implements ConnectionSecretWriterTo interface.
type ConnectionSecretWriterTo struct{ Ref *xpv1.SecretReference }

// SetWriteConnectionSecretToReference sets the WriteConnectionSecretToReference.
func (m *ConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	m.Ref = r
}

// GetWriteConnectionSecretToReference gets the WriteConnectionSecretToReference.
func (m *ConnectionSecretWriterTo) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return m.Ref
}

// Manageable implements the Manageable interface.
type Manageable struct{ Policy xpv1.ManagementPolicies }

// SetManagementPolicies sets the ManagementPolicies.
func (m *Manageable) SetManagementPolicies(p xpv1.ManagementPolicies) { m.Policy = p }

// GetManagementPolicies gets the ManagementPolicies.
func (m *Manageable) GetManagementPolicies() xpv1.ManagementPolicies { return m.Policy }

// Orphanable implements the Orphanable interface.
type Orphanable struct{ Policy xpv1.DeletionPolicy }

// SetDeletionPolicy sets the DeletionPolicy.
func (m *Orphanable) SetDeletionPolicy(p xpv1.DeletionPolicy) { m.Policy = p }

// GetDeletionPolicy gets the DeletionPolicy.
func (m *Orphanable) GetDeletionPolicy() xpv1.DeletionPolicy { return m.Policy }

// CompositionReferencer is a mock that implements CompositionReferencer interface.
type CompositionReferencer struct{ Ref *corev1.ObjectReference }

// SetCompositionReference sets the CompositionReference.
func (m *CompositionReferencer) SetCompositionReference(r *corev1.ObjectReference) { m.Ref = r }

// GetCompositionReference gets the CompositionReference.
func (m *CompositionReferencer) GetCompositionReference() *corev1.ObjectReference { return m.Ref }

// CompositionSelector is a mock that implements CompositionSelector interface.
type CompositionSelector struct{ Sel *metav1.LabelSelector }

// SetCompositionSelector sets the CompositionSelector.
func (m *CompositionSelector) SetCompositionSelector(s *metav1.LabelSelector) { m.Sel = s }

// GetCompositionSelector gets the CompositionSelector.
func (m *CompositionSelector) GetCompositionSelector() *metav1.LabelSelector { return m.Sel }

// CompositionRevisionReferencer is a mock that implements CompositionRevisionReferencer interface.
type CompositionRevisionReferencer struct{ Ref *corev1.LocalObjectReference }

// SetCompositionRevisionReference sets the CompositionRevisionReference.
func (m *CompositionRevisionReferencer) SetCompositionRevisionReference(r *corev1.LocalObjectReference) {
	m.Ref = r
}

// GetCompositionRevisionReference gets the CompositionRevisionReference.
func (m *CompositionRevisionReferencer) GetCompositionRevisionReference() *corev1.LocalObjectReference {
	return m.Ref
}

// CompositionRevisionSelector is a mock that implements CompositionRevisionSelector interface.
type CompositionRevisionSelector struct{ Sel *metav1.LabelSelector }

// SetCompositionRevisionSelector sets the CompositionRevisionSelector.
func (m *CompositionRevisionSelector) SetCompositionRevisionSelector(ls *metav1.LabelSelector) {
	m.Sel = ls
}

// GetCompositionRevisionSelector gets the CompositionRevisionSelector.
func (m *CompositionRevisionSelector) GetCompositionRevisionSelector() *metav1.LabelSelector {
	return m.Sel
}

// CompositionUpdater is a mock that implements CompositionUpdater interface.
type CompositionUpdater struct{ Policy *xpv1.UpdatePolicy }

// SetCompositionUpdatePolicy sets the CompositionUpdatePolicy.
func (m *CompositionUpdater) SetCompositionUpdatePolicy(p *xpv1.UpdatePolicy) {
	m.Policy = p
}

// GetCompositionUpdatePolicy gets the CompositionUpdatePolicy.
func (m *CompositionUpdater) GetCompositionUpdatePolicy() *xpv1.UpdatePolicy {
	return m.Policy
}

// CompositeResourceDeleter is a mock that implements CompositeResourceDeleter interface.
type CompositeResourceDeleter struct{ Policy *xpv1.CompositeDeletePolicy }

// SetCompositeDeletePolicy sets the CompositeDeletePolicy.
func (m *CompositeResourceDeleter) SetCompositeDeletePolicy(p *xpv1.CompositeDeletePolicy) {
	m.Policy = p
}

// GetCompositeDeletePolicy gets the CompositeDeletePolicy.
func (m *CompositeResourceDeleter) GetCompositeDeletePolicy() *xpv1.CompositeDeletePolicy {
	return m.Policy
}

// CompositeResourceReferencer is a mock that implements CompositeResourceReferencer interface.
type CompositeResourceReferencer struct{ Ref *reference.Composite }

// SetResourceReference sets the composite resource reference.
func (m *CompositeResourceReferencer) SetResourceReference(p *reference.Composite) { m.Ref = p }

// GetResourceReference gets the composite resource reference.
func (m *CompositeResourceReferencer) GetResourceReference() *reference.Composite { return m.Ref }

// ComposedResourcesReferencer is a mock that implements ComposedResourcesReferencer interface.
type ComposedResourcesReferencer struct{ Refs []corev1.ObjectReference }

// SetResourceReferences sets the composed references.
func (m *ComposedResourcesReferencer) SetResourceReferences(r []corev1.ObjectReference) { m.Refs = r }

// GetResourceReferences gets the composed references.
func (m *ComposedResourcesReferencer) GetResourceReferences() []corev1.ObjectReference { return m.Refs }

// An EnvironmentConfigReferencer is a mock that implements the
// EnvironmentConfigReferencer interface.
type EnvironmentConfigReferencer struct{ Refs []corev1.ObjectReference }

// SetEnvironmentConfigReferences sets the EnvironmentConfig references.
func (m *EnvironmentConfigReferencer) SetEnvironmentConfigReferences(refs []corev1.ObjectReference) {
	m.Refs = refs
}

// GetEnvironmentConfigReferences gets the EnvironmentConfig references.
func (m *EnvironmentConfigReferencer) GetEnvironmentConfigReferences() []corev1.ObjectReference {
	return m.Refs
}

// ConnectionDetailsLastPublishedTimer is a mock that implements the
// ConnectionDetailsLastPublishedTimer interface.
type ConnectionDetailsLastPublishedTimer struct {
	// NOTE: runtime.DefaultUnstructuredConverter.ToUnstructured
	// cannot currently handle if `Time` is nil here.
	// The `omitempty` json tag is a workaround that
	// prevents a panic.
	Time *metav1.Time `json:"lastPublishedTime,omitempty"`
}

// SetConnectionDetailsLastPublishedTime sets the published time.
func (c *ConnectionDetailsLastPublishedTimer) SetConnectionDetailsLastPublishedTime(t *metav1.Time) {
	c.Time = t
}

// GetConnectionDetailsLastPublishedTime gets the published time.
func (c *ConnectionDetailsLastPublishedTimer) GetConnectionDetailsLastPublishedTime() *metav1.Time {
	return c.Time
}

// UserCounter is a mock that satisfies UserCounter
// interface.
type UserCounter struct{ Users int64 }

// SetUsers sets the count of users.
func (m *UserCounter) SetUsers(i int64) {
	m.Users = i
}

// GetUsers gets the count of users.
func (m *UserCounter) GetUsers() int64 {
	return m.Users
}

// Object is a mock that implements Object interface.
type Object struct {
	metav1.ObjectMeta
	runtime.Object
}

// GetObjectKind returns schema.ObjectKind.
func (o *Object) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (o *Object) DeepCopyObject() runtime.Object {
	out := &Object{}

	j, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// Managed is a mock that implements Managed interface.
type Managed struct {
	metav1.ObjectMeta
	Manageable
	xpv1.ConditionedStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Managed) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *Managed) DeepCopyObject() runtime.Object {
	out := &Managed{}

	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// ModernManaged is a mock that implements ModernManaged interface.
type ModernManaged struct {
	metav1.ObjectMeta
	TypedProviderConfigReferencer
	LocalConnectionSecretWriterTo
	Manageable
	xpv1.ConditionedStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *ModernManaged) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *ModernManaged) DeepCopyObject() runtime.Object {
	out := &ModernManaged{}

	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// LegacyManaged is a mock that implements LegacyManaged interface.
type LegacyManaged struct {
	metav1.ObjectMeta
	LegacyProviderConfigReferencer
	ConnectionSecretWriterTo
	Manageable
	Orphanable
	xpv1.ConditionedStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *LegacyManaged) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *LegacyManaged) DeepCopyObject() runtime.Object {
	out := &LegacyManaged{}

	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// Composite is a mock that implements Composite interface.
type Composite struct {
	metav1.ObjectMeta
	CompositionSelector
	CompositionReferencer
	CompositionRevisionReferencer
	CompositionRevisionSelector
	CompositionUpdater
	ComposedResourcesReferencer
	EnvironmentConfigReferencer
	ClaimReferencer
	ConnectionSecretWriterTo

	xpv1.ResourceStatus
	ConnectionDetailsLastPublishedTimer
}

// GetObjectKind returns schema.ObjectKind.
func (m *Composite) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *Composite) DeepCopyObject() runtime.Object {
	out := &Composite{}

	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// Composed is a mock that implements Composed interface.
type Composed struct {
	metav1.ObjectMeta
	ConnectionSecretWriterTo
	xpv1.ResourceStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Composed) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *Composed) DeepCopyObject() runtime.Object {
	out := &Composed{}

	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// CompositeClaim is a mock that implements the CompositeClaim interface.
type CompositeClaim struct {
	metav1.ObjectMeta
	CompositionSelector
	CompositionReferencer
	CompositionRevisionReferencer
	CompositionRevisionSelector
	CompositeResourceDeleter
	CompositionUpdater
	CompositeResourceReferencer
	LocalConnectionSecretWriterTo

	xpv1.ResourceStatus
	ConnectionDetailsLastPublishedTimer
}

// GetObjectKind returns schema.ObjectKind.
func (m *CompositeClaim) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *CompositeClaim) DeepCopyObject() runtime.Object {
	out := &CompositeClaim{}

	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// Manager is a mock object that satisfies manager.Manager interface.
type Manager struct {
	manager.Manager

	Cache      cache.Cache
	Client     client.Client
	Scheme     *runtime.Scheme
	Config     *rest.Config
	RESTMapper meta.RESTMapper
	Logger     logr.Logger
}

// Elected returns a closed channel.
func (m *Manager) Elected() <-chan struct{} {
	e := make(chan struct{})
	close(e)

	return e
}

// GetCache returns the cache.
func (m *Manager) GetCache() cache.Cache { return m.Cache }

// GetClient returns the client.
func (m *Manager) GetClient() client.Client { return m.Client }

// GetScheme returns the scheme.
func (m *Manager) GetScheme() *runtime.Scheme { return m.Scheme }

// GetConfig returns the config.
func (m *Manager) GetConfig() *rest.Config { return m.Config }

// GetRESTMapper returns the REST mapper.
func (m *Manager) GetRESTMapper() meta.RESTMapper { return m.RESTMapper }

// GetLogger returns the logger.
func (m *Manager) GetLogger() logr.Logger { return m.Logger }

// GV returns a mock schema.GroupVersion.
var GV = schema.GroupVersion{Group: "g", Version: "v"} //nolint:gochecknoglobals // We treat this as a constant.

// GVK returns the mock GVK of the given object.
func GVK(o runtime.Object) schema.GroupVersionKind {
	return GV.WithKind(reflect.TypeOf(o).Elem().Name())
}

// SchemeWith returns a scheme with list of `runtime.Object`s registered.
func SchemeWith(o ...runtime.Object) *runtime.Scheme {
	s := runtime.NewScheme()
	s.AddKnownTypes(GV, o...)

	return s
}

// MockConnectionSecretOwner is a mock object that satisfies ConnectionSecretOwner
// interface.
type MockConnectionSecretOwner struct {
	runtime.Object
	metav1.ObjectMeta

	WriterTo *xpv1.SecretReference
}

// GetWriteConnectionSecretToReference returns the connection secret reference.
func (m *MockConnectionSecretOwner) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	return m.WriterTo
}

// SetWriteConnectionSecretToReference sets the connection secret reference.
func (m *MockConnectionSecretOwner) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	m.WriterTo = r
}

// GetObjectKind returns schema.ObjectKind.
func (m *MockConnectionSecretOwner) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *MockConnectionSecretOwner) DeepCopyObject() runtime.Object {
	out := &MockConnectionSecretOwner{}

	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// MockLocalConnectionSecretOwner is a mock object that satisfies LocalConnectionSecretOwner
// interface.
type MockLocalConnectionSecretOwner struct {
	runtime.Object
	metav1.ObjectMeta

	Ref *xpv1.LocalSecretReference
}

// GetWriteConnectionSecretToReference returns the connection secret reference.
func (m *MockLocalConnectionSecretOwner) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return m.Ref
}

// SetWriteConnectionSecretToReference sets the connection secret reference.
func (m *MockLocalConnectionSecretOwner) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	m.Ref = r
}

// GetObjectKind returns schema.ObjectKind.
func (m *MockLocalConnectionSecretOwner) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *MockLocalConnectionSecretOwner) DeepCopyObject() runtime.Object {
	out := &MockLocalConnectionSecretOwner{}

	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// ProviderConfig is a mock implementation of the ProviderConfig interface.
type ProviderConfig struct {
	metav1.ObjectMeta

	UserCounter
	xpv1.ConditionedStatus
}

// GetObjectKind returns schema.ObjectKind.
func (p *ProviderConfig) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (p *ProviderConfig) DeepCopyObject() runtime.Object {
	out := &ProviderConfig{}

	j, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// ProviderConfigUsage is a mock implementation of the ProviderConfigUsage
// interface.
type ProviderConfigUsage struct {
	metav1.ObjectMeta

	RequiredTypedProviderConfigReferencer
	RequiredTypedResourceReferencer
}

// GetObjectKind returns schema.ObjectKind.
func (p *ProviderConfigUsage) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (p *ProviderConfigUsage) DeepCopyObject() runtime.Object {
	out := &ProviderConfigUsage{}

	j, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}

// LegacyProviderConfigUsage is a mock implementation of the LegacyProviderConfigUsage
// interface.
type LegacyProviderConfigUsage struct {
	metav1.ObjectMeta

	RequiredProviderConfigReferencer
	RequiredTypedResourceReferencer
}

// GetObjectKind returns schema.ObjectKind.
func (p *LegacyProviderConfigUsage) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (p *LegacyProviderConfigUsage) DeepCopyObject() runtime.Object {
	out := &LegacyProviderConfigUsage{}

	j, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out)

	return out
}
