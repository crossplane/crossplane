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

package v1alpha1

import (
	"encoding/json"
	"time"

	"cloud.google.com/go/storage"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// ProjectTeam is the project team associated with the entity, if any.
type ProjectTeam struct {
	ProjectNumber string
	Team          string
}

// NewProjectTeam creates new instance of ProjectTeam from the storage counterpart
func NewProjectTeam(pt *storage.ProjectTeam) *ProjectTeam {
	if pt == nil {
		return nil
	}
	return &ProjectTeam{
		ProjectNumber: pt.ProjectNumber,
		Team:          pt.Team,
	}
}

// CopyToProjectTeam create a copy in storage format
func CopyToProjectTeam(pt *ProjectTeam) *storage.ProjectTeam {
	if pt == nil {
		return nil
	}
	return &storage.ProjectTeam{
		ProjectNumber: pt.ProjectNumber,
		Team:          pt.Team,
	}
}

// ACLRule represents a grant for a role to an entity (user, group or team) for a
// Google Cloud Storage object or bucket.
type ACLRule struct {
	Entity      string
	EntityID    string
	Role        string
	Domain      string
	Email       string
	ProjectTeam *ProjectTeam
}

// NewACLRule creates new instance of ACLRule from the storage counterpart
func NewACLRule(r storage.ACLRule) ACLRule {
	return ACLRule{
		Entity:      string(r.Entity),
		EntityID:    r.EntityID,
		Role:        string(r.Role),
		Domain:      r.Domain,
		Email:       r.Email,
		ProjectTeam: NewProjectTeam(r.ProjectTeam),
	}
}

// CopyToACLRule create a copy in storage format
func CopyToACLRule(ar ACLRule) storage.ACLRule {
	return storage.ACLRule{
		Entity:      storage.ACLEntity(ar.Entity),
		EntityID:    ar.EntityID,
		Role:        storage.ACLRole(ar.Role),
		Domain:      ar.Domain,
		Email:       ar.Email,
		ProjectTeam: CopyToProjectTeam(ar.ProjectTeam),
	}
}

// NewACLRules creates a new instance of ACLRule list from the storage counterpart
func NewACLRules(r []storage.ACLRule) []ACLRule {
	var rules []ACLRule
	if l := len(r); l > 0 {
		rules = make([]ACLRule, l)
		for i, v := range r {
			rules[i] = NewACLRule(v)
		}
	}
	return rules
}

// CopyToACLRules create a copy in storage format
func CopyToACLRules(r []ACLRule) []storage.ACLRule {
	var rules []storage.ACLRule
	if l := len(r); l > 0 {
		rules = make([]storage.ACLRule, l)
		for i, v := range r {
			rules[i] = CopyToACLRule(v)
		}
	}
	return rules
}

// LifecycleAction is a lifecycle configuration action.
type LifecycleAction struct {
	// StorageClass is the storage class to set on matching objects if the Action
	// is "SetStorageClass".
	StorageClass string `json:"storageClass,omitempty"`

	// Type is the type of action to take on matching objects.
	//
	// Acceptable values are "Delete" to delete matching objects and
	// "SetStorageClass" to set the storage class defined in StorageClass on
	// matching objects.
	Type string `json:"type,omitempty"`
}

// NewLifecyleAction creates a new instance of LifecycleAction from the storage counterpart
func NewLifecyleAction(la storage.LifecycleAction) LifecycleAction {
	return LifecycleAction{
		Type:         la.Type,
		StorageClass: la.StorageClass,
	}
}

// CopyToLifecyleAction create a copy in storage format
func CopyToLifecyleAction(la LifecycleAction) storage.LifecycleAction {
	return storage.LifecycleAction{
		Type:         la.Type,
		StorageClass: la.StorageClass,
	}
}

// LifecycleCondition is a set of conditions used to match objects and take an
// action automatically.
//
// All configured conditions must be met for the associated action to be taken.
type LifecycleCondition struct {
	// AgeInDays is the age of the object in days.
	AgeInDays int64 `json:"ageInDays,omitempty"`

	// CreatedBefore is the time the object was created.
	//
	// This condition is satisfied when an object is created before midnight of
	// the specified date in UTC.
	CreatedBefore metav1.Time `json:"createdBefore,omitempty"`

	// Liveness specifies the object's liveness. Relevant only for versioned objects
	Liveness storage.Liveness `json:"liveness,omitempty"`

	// MatchesStorageClasses is the condition matching the object's storage
	// class.
	//
	// Values include "MULTI_REGIONAL", "REGIONAL", "NEARLINE", "COLDLINE",
	// "STANDARD", and "DURABLE_REDUCED_AVAILABILITY".
	MatchesStorageClasses []string `json:"matchesStorageClasses,omitempty"`

	// NumNewerVersions is the condition matching objects with a number of newer versions.
	//
	// If the value is N, this condition is satisfied when there are at least N
	// versions (including the live version) newer than this version of the
	// object.
	NumNewerVersions int64 `json:"numNewerVersions,omitempty"`
}

// NewLifecycleCondition creates a new instance of LifecycleCondition from the storage counterpart
func NewLifecycleCondition(lc storage.LifecycleCondition) LifecycleCondition {
	return LifecycleCondition{
		AgeInDays:             lc.AgeInDays,
		CreatedBefore:         metav1.Time{Time: lc.CreatedBefore},
		Liveness:              lc.Liveness,
		MatchesStorageClasses: lc.MatchesStorageClasses,
		NumNewerVersions:      lc.NumNewerVersions,
	}
}

// CopyToLifecycleCondition create a copy in storage format
func CopyToLifecycleCondition(lc LifecycleCondition) storage.LifecycleCondition {
	return storage.LifecycleCondition{
		AgeInDays:             lc.AgeInDays,
		CreatedBefore:         lc.CreatedBefore.Time,
		Liveness:              lc.Liveness,
		MatchesStorageClasses: lc.MatchesStorageClasses,
		NumNewerVersions:      lc.NumNewerVersions,
	}
}

// LifecycleRule is a lifecycle configuration rule.
//
// When all the configured conditions are met by an object in the bucket, the
// configured action will automatically be taken on that object.
type LifecycleRule struct {
	// Action is the action to take when all of the associated conditions are
	// met.
	Action LifecycleAction `json:"action,omitempty"`

	// Condition is the set of conditions that must be met for the associated
	// action to be taken.
	Condition LifecycleCondition `json:"condition,omitempty"`
}

// NewLifecycleRule creates a new instance of LifecycleRule from the storage counterpart
func NewLifecycleRule(lr storage.LifecycleRule) LifecycleRule {
	return LifecycleRule{
		Action:    NewLifecyleAction(lr.Action),
		Condition: NewLifecycleCondition(lr.Condition),
	}
}

// CopyToLifecyleRule create a copy in storage format
func CopyToLifecyleRule(lr LifecycleRule) storage.LifecycleRule {
	return storage.LifecycleRule{
		Action:    CopyToLifecyleAction(lr.Action),
		Condition: CopyToLifecycleCondition(lr.Condition),
	}
}

// Lifecycle is the lifecycle configuration for objects in the bucket.
type Lifecycle struct {
	Rules []LifecycleRule `json:"rules,omitempty"`
}

// NewLifecycle creates a new instance of Lifecycle from the storage counterpart
func NewLifecycle(lf storage.Lifecycle) *Lifecycle {
	lifecycle := &Lifecycle{}

	if l := len(lf.Rules); l > 0 {
		lifecycle.Rules = make([]LifecycleRule, l)
		for i, v := range lf.Rules {
			lifecycle.Rules[i] = NewLifecycleRule(v)
		}
	}

	return lifecycle
}

// CopyToLifecycle create a copy in storage format
func CopyToLifecycle(lf Lifecycle) storage.Lifecycle {
	lifecycle := storage.Lifecycle{}

	if l := len(lf.Rules); l > 0 {
		lifecycle.Rules = make([]storage.LifecycleRule, l)
		for i, v := range lf.Rules {
			lifecycle.Rules[i] = CopyToLifecyleRule(v)
		}
	}

	return lifecycle
}

// RetentionPolicy enforces a minimum retention time for all objects
// contained in the bucket.
//
// Any attempt to overwrite or delete objects younger than the retention
// period will result in an error. An unlocked retention policy can be
// modified or removed from the bucket via the Update method. A
// locked retention policy cannot be removed or shortened in duration
// for the lifetime of the bucket.
//
// This feature is in private alpha release. It is not currently available to
// most customers. It might be changed in backwards-incompatible ways and is not
// subject to any SLA or deprecation policy.
type RetentionPolicy struct {
	// RetentionPeriod specifies the duration value in seconds that objects
	// need to be retained. Retention duration must be greater than zero and
	// less than 100 years. Note that enforcement of retention periods less
	// than a day is not guaranteed. Such periods should only be used for
	// testing purposes.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=3155673600
	RetentionPeriodSeconds int `json:"retentionPeriodSeconds,omitempty"`
}

// NewRetentionPolicy creates a new instance of RetentionPolicy from the storage counterpart
func NewRetentionPolicy(rp *storage.RetentionPolicy) *RetentionPolicy {
	if rp == nil {
		return nil
	}
	return &RetentionPolicy{
		RetentionPeriodSeconds: int(rp.RetentionPeriod.Seconds()),
	}
}

// CopyToRetentionPolicy create a copy in storage format
func CopyToRetentionPolicy(rp *RetentionPolicy) *storage.RetentionPolicy {
	var d time.Duration

	if rp == nil {
		d = time.Duration(0)
	} else {
		d = time.Duration(rp.RetentionPeriodSeconds)
	}

	return &storage.RetentionPolicy{
		RetentionPeriod: d * time.Second,
	}
}

// RetentionPolicyStatus output component of storage.RetentionPolicy
type RetentionPolicyStatus struct {
	// EffectiveTime is the time from which the policy was enforced and
	// effective.
	EffectiveTime metav1.Time `json:"effectiveTime,omitempty"`

	// IsLocked describes whether the bucket is locked. Once locked, an object
	// retention policy cannot be modified.
	IsLocked bool `json:"isLocked,omitempty"`
}

// NewRetentionPolicyStatus creates a new instance of RetentionPolicy from the storage counterpart
func NewRetentionPolicyStatus(r *storage.RetentionPolicy) *RetentionPolicyStatus {
	if r == nil {
		return nil
	}
	return &RetentionPolicyStatus{
		EffectiveTime: metav1.Time{
			Time: r.EffectiveTime,
		},
		IsLocked: r.IsLocked,
	}
}

// BucketEncryption is a bucket's encryption configuration.
type BucketEncryption struct {
	// A Cloud KMS key name, in the form
	// projects/P/locations/L/keyRings/R/cryptoKeys/K, that will be used to encrypt
	// objects inserted into this bucket, if no encryption method is specified.
	// The key's location must be the same as the bucket's.
	DefaultKMSKeyName string `json:"defaultKmsKeyName,omitempty"`
}

// NewBucketEncryption creates a new instance of BucketEncryption from the storage counterpart
func NewBucketEncryption(e *storage.BucketEncryption) *BucketEncryption {
	if e == nil {
		return nil
	}
	return &BucketEncryption{
		DefaultKMSKeyName: e.DefaultKMSKeyName,
	}
}

// CopyToBucketEncryption create a copy in storage format
func CopyToBucketEncryption(e *BucketEncryption) *storage.BucketEncryption {
	if e == nil {
		return nil
	}
	return &storage.BucketEncryption{
		DefaultKMSKeyName: e.DefaultKMSKeyName,
	}
}

// BucketLogging holds the bucket's logging configuration, which defines the
// destination bucket and optional name prefix for the current bucket's
// logs.
type BucketLogging struct {
	// The destination bucket where the current bucket's logs
	// should be placed.
	LogBucket string `json:"logBucket,omitempty"`

	// A prefix for log object names.
	LogObjectPrefix string `json:"logObjectPrefix,omitempty"`
}

// NewBucketLogging creates a new instance of BucketLogging from the storage counterpart
func NewBucketLogging(l *storage.BucketLogging) *BucketLogging {
	if l == nil {
		return nil
	}
	return &BucketLogging{
		LogBucket:       l.LogBucket,
		LogObjectPrefix: l.LogObjectPrefix,
	}
}

// CopyToBucketLogging create a copy in storage format
func CopyToBucketLogging(l *BucketLogging) *storage.BucketLogging {
	if l == nil {
		return nil
	}
	return &storage.BucketLogging{
		LogBucket:       l.LogBucket,
		LogObjectPrefix: l.LogObjectPrefix,
	}
}

// CORS is the bucket's Cross-Origin Resource Sharing (CORS) configuration.
type CORS struct {
	// MaxAge is the value to return in the Access-Control-Max-Age
	// header used in preflight responses.
	MaxAge metav1.Duration `json:"maxAge,omitempty"`

	// Methods is the list of HTTP methods on which to include CORS response
	// headers, (GET, OPTIONS, POST, etc) Note: "*" is permitted in the list
	// of methods, and means "any method".
	Methods []string `json:"methods,omitempty"`

	// Origins is the list of Origins eligible to receive CORS response
	// headers. Note: "*" is permitted in the list of origins, and means
	// "any Origin".
	Origins []string `json:"origins,omitempty"`

	// ResponseHeaders is the list of HTTP headers other than the simple
	// response headers to give permission for the user-agent to share
	// across domains.
	ResponseHeaders []string `json:"responseHeaders,omitempty"`
}

// NewCORS creates a new instance of CORS from the storage counterpart
func NewCORS(c storage.CORS) CORS {
	return CORS{
		MaxAge:          metav1.Duration{Duration: c.MaxAge},
		Methods:         c.Methods,
		Origins:         c.Origins,
		ResponseHeaders: c.ResponseHeaders,
	}
}

// CopyToCORS create a copy in storage format
func CopyToCORS(c CORS) storage.CORS {
	return storage.CORS{
		MaxAge:          c.MaxAge.Duration,
		Methods:         c.Methods,
		Origins:         c.Origins,
		ResponseHeaders: c.ResponseHeaders,
	}
}

// NewCORSList creates a new instance of CORS list from the storage counterpart
func NewCORSList(c []storage.CORS) []CORS {
	if c == nil {
		return nil
	}
	cors := make([]CORS, len(c))
	for i, v := range c {
		cors[i] = NewCORS(v)
	}

	return cors
}

// CopyToCORSList create a copy in storage format
func CopyToCORSList(c []CORS) []storage.CORS {
	if c == nil {
		return nil
	}
	cors := make([]storage.CORS, len(c))
	for i, v := range c {
		cors[i] = CopyToCORS(v)
	}
	return cors
}

// BucketWebsite holds the bucket's website configuration, controlling how the
// service behaves when accessing bucket contents as a web site. See
// https://cloud.google.com/storage/docs/static-website for more information.
type BucketWebsite struct {
	// If the requested object path is missing, the service will ensure the path has
	// a trailing '/', append this suffix, and attempt to retrieve the resulting
	// object. This allows the creation of index.html objects to represent directory
	// pages.
	MainPageSuffix string `json:"mainPageSuffix,omitempty"`

	// If the requested object path is missing, and any mainPageSuffix object is
	// missing, if applicable, the service will return the named object from this
	// bucket as the content for a 404 Not Found result.
	NotFoundPage string `json:"notFundPage,omitempty"`
}

// NewBucketWebsite creates a new instance of BucketWebsite from the storage counterpart
func NewBucketWebsite(w *storage.BucketWebsite) *BucketWebsite {
	if w == nil {
		return nil
	}
	return &BucketWebsite{
		MainPageSuffix: w.MainPageSuffix,
		NotFoundPage:   w.NotFoundPage,
	}
}

// CopyToBucketWebsite create a copy in storage format
func CopyToBucketWebsite(w *BucketWebsite) *storage.BucketWebsite {
	if w == nil {
		return nil
	}
	return &storage.BucketWebsite{
		MainPageSuffix: w.MainPageSuffix,
		NotFoundPage:   w.NotFoundPage,
	}
}

// BucketPolicyOnly configures access checks to use only bucket-level IAM
// policies.
type BucketPolicyOnly struct {
	// Enabled specifies whether access checks use only bucket-level IAM
	// policies. Enabled may be disabled until the locked time.
	Enabled bool
	// LockedTime specifies the deadline for changing Enabled from true to
	// false.
	LockedTime metav1.Time
}

// NewBucketPolicyOnly creates new instance based on the storage object
func NewBucketPolicyOnly(bp storage.BucketPolicyOnly) BucketPolicyOnly {
	return BucketPolicyOnly{
		Enabled:    bp.Enabled,
		LockedTime: metav1.Time{Time: bp.LockedTime},
	}
}

// CopyToBucketPolicyOnly creates storage equivalent
func CopyToBucketPolicyOnly(bp BucketPolicyOnly) storage.BucketPolicyOnly {
	return storage.BucketPolicyOnly{
		Enabled:    bp.Enabled,
		LockedTime: bp.LockedTime.Time,
	}
}

// BucketUpdatableAttrs represents the subset of metadata for a Google Cloud Storage
// bucket limited to all updatable input attributes
type BucketUpdatableAttrs struct {
	// BucketPolicyOnly configures access checks to use only bucket-level IAM
	// policies.
	BucketPolicyOnly BucketPolicyOnly `json:"bucketPolicyOnly,omitempty"`

	// The bucket's Cross-Origin Resource Sharing (CORS) configuration.
	CORS []CORS `json:"cors,omitempty"`

	// DefaultEventBasedHold is the default value for event-based hold on
	// newly created objects in this bucket. It defaults to false.
	DefaultEventBasedHold bool `json:"defaultEventBasedHold,omitempty"`

	// The encryption configuration used by default for newly inserted objects.
	Encryption *BucketEncryption `json:"encryption,omitempty"`

	// Labels are the bucket's labels.
	Labels map[string]string `json:"labels,omitempty"`

	// Lifecycle is the lifecycle configuration for objects in the bucket.
	Lifecycle Lifecycle `json:"lifecycle,omitempty"`

	// The logging configuration.
	Logging *BucketLogging `json:"logging,omitempty"`

	// If not empty, applies a predefined set of access controls. It should be set
	// only when creating a bucket.
	// It is always empty for BucketAttrs returned from the service.
	// See https://cloud.google.com/storage/docs/json_api/v1/buckets/insert
	// for valid values.
	PredefinedACL string `json:"predefinedAcl,omitempty"`

	// If not empty, applies a predefined set of default object access controls.
	// It should be set only when creating a bucket.
	// It is always empty for BucketAttrs returned from the service.
	// See https://cloud.google.com/storage/docs/json_api/v1/buckets/insert
	// for valid values.
	PredefinedDefaultObjectACL string `json:"predefinedCefaultObjectAcl,omitempty"`

	// RequesterPays reports whether the bucket is a Requester Pays bucket.
	// Clients performing operations on Requester Pays buckets must provide
	// a user project (see BucketHandle.UserProject), which will be billed
	// for the operations.
	RequesterPays bool `json:"requesterPays,omitempty"`

	// Retention policy enforces a minimum retention time for all objects
	// contained in the bucket. A RetentionPolicy of nil implies the bucket
	// has no minimum data retention.
	//
	// This feature is in private alpha release. It is not currently available to
	// most customers. It might be changed in backwards-incompatible ways and is not
	// subject to any SLA or deprecation policy.
	RetentionPolicy *RetentionPolicy `json:"retentionPolicy,omitempty"`

	// VersioningEnabled reports whether this bucket has versioning enabled.
	VersioningEnabled bool `json:"versioningEnabled,omitempty"`

	// The website configuration.
	Website *BucketWebsite `json:"website,omitempty"`
}

// NewBucketUpdatableAttrs creates a new instance of BucketUpdatableAttrs from the storage BucketAttrs
func NewBucketUpdatableAttrs(ba *storage.BucketAttrs) *BucketUpdatableAttrs {
	if ba == nil {
		return nil
	}

	return &BucketUpdatableAttrs{
		BucketPolicyOnly:           NewBucketPolicyOnly(ba.BucketPolicyOnly),
		CORS:                       NewCORSList(ba.CORS),
		DefaultEventBasedHold:      ba.DefaultEventBasedHold,
		Encryption:                 NewBucketEncryption(ba.Encryption),
		Labels:                     ba.Labels,
		Lifecycle:                  *NewLifecycle(ba.Lifecycle),
		Logging:                    NewBucketLogging(ba.Logging),
		PredefinedACL:              ba.PredefinedACL,
		PredefinedDefaultObjectACL: ba.PredefinedDefaultObjectACL,
		RequesterPays:              ba.RequesterPays,
		RetentionPolicy:            NewRetentionPolicy(ba.RetentionPolicy),
		VersioningEnabled:          ba.VersioningEnabled,
		Website:                    NewBucketWebsite(ba.Website),
	}
}

// CopyToBucketAttrs create a copy in storage format
func CopyToBucketAttrs(ba *BucketUpdatableAttrs) *storage.BucketAttrs {
	if ba == nil {
		return nil
	}

	return &storage.BucketAttrs{
		BucketPolicyOnly:           storage.BucketPolicyOnly{Enabled: ba.BucketPolicyOnly.Enabled},
		CORS:                       CopyToCORSList(ba.CORS),
		DefaultEventBasedHold:      ba.DefaultEventBasedHold,
		Encryption:                 CopyToBucketEncryption(ba.Encryption),
		Labels:                     ba.Labels,
		Lifecycle:                  CopyToLifecycle(ba.Lifecycle),
		Logging:                    CopyToBucketLogging(ba.Logging),
		PredefinedACL:              ba.PredefinedACL,
		PredefinedDefaultObjectACL: ba.PredefinedDefaultObjectACL,
		RequesterPays:              ba.RequesterPays,
		RetentionPolicy:            CopyToRetentionPolicy(ba.RetentionPolicy),
		VersioningEnabled:          ba.VersioningEnabled,
		Website:                    CopyToBucketWebsite(ba.Website),
	}
}

// CopyToBucketUpdateAttrs create a copy in storage format
func CopyToBucketUpdateAttrs(ba BucketUpdatableAttrs, labels map[string]string) storage.BucketAttrsToUpdate {
	bucketPolicyOnly := CopyToBucketPolicyOnly(ba.BucketPolicyOnly)
	lifecycle := CopyToLifecycle(ba.Lifecycle)

	update := storage.BucketAttrsToUpdate{
		BucketPolicyOnly:           &bucketPolicyOnly,
		CORS:                       CopyToCORSList(ba.CORS),
		DefaultEventBasedHold:      ba.DefaultEventBasedHold,
		Encryption:                 CopyToBucketEncryption(ba.Encryption),
		Lifecycle:                  &lifecycle,
		Logging:                    CopyToBucketLogging(ba.Logging),
		PredefinedACL:              ba.PredefinedACL,
		PredefinedDefaultObjectACL: ba.PredefinedDefaultObjectACL,
		RequesterPays:              ba.RequesterPays,
		RetentionPolicy:            CopyToRetentionPolicy(ba.RetentionPolicy),
		VersioningEnabled:          ba.VersioningEnabled,
		Website:                    CopyToBucketWebsite(ba.Website),
	}

	for k, v := range ba.Labels {
		update.SetLabel(k, v)
		delete(labels, k)
	}

	for k := range labels {
		update.DeleteLabel(k)
	}

	return update
}

// BucketSpecAttrs represents the full set of metadata for a Google Cloud Storage
// bucket limited to all input attributes
type BucketSpecAttrs struct {
	BucketUpdatableAttrs `json:",inline"`

	// ACL is the list of access control rules on the bucket.
	ACL []ACLRule

	// DefaultObjectACL is the list of access controls to
	// apply to new objects when no object ACL is provided.
	DefaultObjectACL []ACLRule `json:"defaultObjectAcl,omitempty"`

	// Location is the location of the bucket. It defaults to "US".
	Location string `json:"location,omitempty"`

	// StorageClass is the default storage class of the bucket. This defines
	// how objects in the bucket are stored and determines the SLA
	// and the cost of storage. Typical values are "MULTI_REGIONAL",
	// "REGIONAL", "NEARLINE", "COLDLINE", "STANDARD" and
	// "DURABLE_REDUCED_AVAILABILITY". Defaults to "STANDARD", which
	// is equivalent to "MULTI_REGIONAL" or "REGIONAL" depending on
	// the bucket's location settings.
	// +kubebuilder:validation:Enum=MULTI_REGIONAL,REGIONAL,NEARLINE,COLDLINE,STANDARD,DURABLE_REDUCED_AVAILABILITY
	StorageClass string `json:"storageClass,omitempty"`
}

// NewBucketSpecAttrs create new instance from storage BuckateAttrs
func NewBucketSpecAttrs(ba *storage.BucketAttrs) BucketSpecAttrs {
	if ba == nil {
		return BucketSpecAttrs{}
	}
	return BucketSpecAttrs{
		BucketUpdatableAttrs: *NewBucketUpdatableAttrs(ba),
		ACL:                  NewACLRules(ba.ACL),
		DefaultObjectACL:     NewACLRules(ba.DefaultObjectACL),
		Location:             ba.Location,
		StorageClass:         ba.StorageClass,
	}
}

// CopyBucketSpecAttrs create a copy in storage format
func CopyBucketSpecAttrs(ba *BucketSpecAttrs) *storage.BucketAttrs {
	if ba == nil {
		return nil
	}
	b := CopyToBucketAttrs(&ba.BucketUpdatableAttrs)
	b.ACL = CopyToACLRules(ba.ACL)
	b.Location = ba.Location
	b.StorageClass = ba.StorageClass
	return b
}

// BucketOutputAttrs represent the subset of metadata for a Google Cloud Storage
// bucket limited to output (read-only) fields.
type BucketOutputAttrs struct {
	// BucketPolicyOnly configures access checks to use only bucket-level IAM
	// policies.
	BucketPolicyOnly BucketPolicyOnly `json:"bucketPolicyOnly,omitempty"`

	// Created is the creation time of the bucket.
	Created metav1.Time

	// Name is the name of the bucket.
	Name string `json:"name,omitempty"`

	// Retention policy enforces a minimum retention time for all objects
	// contained in the bucket. A RetentionPolicy of nil implies the bucket
	// has no minimum data retention.
	//
	// This feature is in private alpha release. It is not currently available to
	// most customers. It might be changed in backwards-incompatible ways and is not
	// subject to any SLA or deprecation policy.
	RetentionPolicy *RetentionPolicyStatus `json:"retentionPolicy,omitempty"`
}

// NewBucketOutputAttrs creates new instance of BucketOutputAttrs from storage.BucketAttrs
func NewBucketOutputAttrs(attrs *storage.BucketAttrs) BucketOutputAttrs {
	if attrs == nil {
		return BucketOutputAttrs{}
	}
	return BucketOutputAttrs{
		BucketPolicyOnly: NewBucketPolicyOnly(attrs.BucketPolicyOnly),
		Created: metav1.Time{
			Time: attrs.Created,
		},
		Name:            attrs.Name,
		RetentionPolicy: NewRetentionPolicyStatus(attrs.RetentionPolicy),
	}
}

// BucketSpec defines the desired state of Bucket
type BucketSpec struct {
	BucketSpecAttrs `json:",inline"`

	// NameFormat to format bucket name passing it a object UID
	// If not provided, defaults to "%s", i.e. UID value
	NameFormat string `json:"nameFormat,omitempty"`

	// ServiceAccountSecretRef contains GCP ServiceAccount secret that will be used
	// for bucket connection secret credentials
	ServiceAccountSecretRef *corev1.LocalObjectReference `json:"serviceAccountSecretRef,omitempty"`

	ConnectionSecretNameOverride string                      `json:"connectionSecretNameOverride,omitempty"`
	ProviderRef                  corev1.LocalObjectReference `json:"providerRef"`
	ClaimRef                     *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef                     *corev1.ObjectReference     `json:"classRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// BucketStatus defines the observed state of GoogleBucket
type BucketStatus struct {
	BucketOutputAttrs `json:"attributes"`

	corev1alpha1.DeprecatedConditionedStatus
	corev1alpha1.BindingStatusPhase
	ConnectionSecretRef corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bucket is the Schema for the instances API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STORAGE_CLASS",type="string",JSONPath=".spec.storageClass"
// +kubebuilder:printcolumn:name="LOCATION",type="string",JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="RECLAIM_POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec   `json:"spec,omitempty"`
	Status BucketStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BucketList contains a list of GCPBuckets
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

// ConnectionSecretName returns a secret name from the reference
func (b *Bucket) ConnectionSecretName() string {
	return util.IfEmptyString(b.Spec.ConnectionSecretNameOverride, b.Name)
}

// ConnectionSecret returns a connection secret for this bucket instance
func (b *Bucket) ConnectionSecret() *corev1.Secret {
	ref := meta.AsOwner(meta.ReferenceTo(b, BucketGroupVersionKind))
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       b.Namespace,
			Name:            b.ConnectionSecretName(),
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Data: map[string][]byte{
			corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(b.GetBucketName()),
		},
	}
}

// GetBucketName based on the NameFormat spec value,
// If name format is not provided, bucket name defaults to UID
// If name format provided with '%s' value, bucket name will result in formatted string + UID,
//   NOTE: only single %s substitution is supported
// If name format does not contain '%s' substitution, i.e. a constant string, the
// constant string value is returned back
//
// Examples:
//   For all examples assume "UID" = "test-uid"
//   1. NameFormat = "", BucketName = "test-uid"
//   2. NameFormat = "%s", BucketName = "test-uid"
//   3. NameFormat = "foo", BucketName = "foo"
//   4. NameFormat = "foo-%s", BucketName = "foo-test-uid"
//   5. NameFormat = "foo-%s-bar-%s", BucketName = "foo-test-uid-bar-%!s(MISSING)"
func (b *Bucket) GetBucketName() string {
	return util.ConditionalStringFormat(b.Spec.NameFormat, string(b.GetUID()))
}

// IsAvailable for usage/binding
func (b *Bucket) IsAvailable() bool {
	return b.Status.IsReady()
}

// IsBound determines if the resource is in a bound binding state
func (b *Bucket) IsBound() bool {
	return b.Status.Phase == corev1alpha1.BindingPhaseBound
}

// SetBound sets the binding state of this resource
func (b *Bucket) SetBound(state bool) {
	if state {
		b.Status.Phase = corev1alpha1.BindingPhaseBound
	} else {
		b.Status.Phase = corev1alpha1.BindingPhaseUnbound
	}
}

// ParseBucketSpec constructs Spec for this resource from the properties map
func ParseBucketSpec(p map[string]string) *BucketSpec {
	var encryption *BucketEncryption
	if v, found := p["encryptionDefaultKmsKeyName"]; found {
		encryption = &BucketEncryption{DefaultKMSKeyName: v}
	}

	lifecycle := &Lifecycle{}
	if v, found := p["lifecycle"]; found {
		lifecycle = parseLifecycle(v)
	}

	var labels map[string]string
	if v, found := p["labels"]; found {
		labels = util.ParseMap(v)
	}

	var logging *BucketLogging
	if v, found := p["logging"]; found {
		logging = parseLogging(v)
	}

	var website *BucketWebsite
	if v, found := p["website"]; found {
		website = parseWebsite(v)
	}

	var serviceAccountSecretRef *corev1.LocalObjectReference
	if v, found := p["serviceAccountSecretRef"]; found {
		serviceAccountSecretRef = &corev1.LocalObjectReference{Name: v}
	}

	bua := BucketUpdatableAttrs{
		BucketPolicyOnly:           BucketPolicyOnly{Enabled: util.ParseBool(p["bucketPolicyOnly"])},
		CORS:                       parseCORSList(p["cors"]),
		DefaultEventBasedHold:      util.ParseBool(p["defaultEventBasedHold"]),
		Encryption:                 encryption,
		Labels:                     labels,
		Lifecycle:                  *lifecycle,
		Logging:                    logging,
		Website:                    website,
		PredefinedACL:              p["predefinedACL"],
		PredefinedDefaultObjectACL: p["predefinedDefaultObjectACL"],
	}

	bsa := BucketSpecAttrs{
		BucketUpdatableAttrs: bua,
		ACL:                  parseACLRules(p["acl"]),
		DefaultObjectACL:     parseACLRules(p["defaultObjectACL"]),
		Location:             p["location"],
		StorageClass:         p["storageClass"],
	}

	return &BucketSpec{
		BucketSpecAttrs:         bsa,
		ReclaimPolicy:           corev1alpha1.ReclaimRetain,
		ServiceAccountSecretRef: serviceAccountSecretRef,
	}
}

// parseCORSList from json encoded string, return nil on error (if any)
func parseCORSList(s string) []CORS {
	var list []CORS
	_ = json.Unmarshal([]byte(s), &list) // nolint: gosec
	return list
}

// parseLifecycle parses json encoded lifecycle string
func parseLifecycle(s string) *Lifecycle {
	l := &Lifecycle{}
	_ = json.Unmarshal([]byte(s), l) // nolint: gosec
	return l
}

// parseLogging from map encoded string value
func parseLogging(s string) *BucketLogging {
	m := util.ParseMap(s)
	return &BucketLogging{
		LogBucket:       m["logBucket"],
		LogObjectPrefix: m["logObjectPrefix"],
	}
}

// parseWebsite from map encoded string value
func parseWebsite(s string) *BucketWebsite {
	m := util.ParseMap(s)
	return &BucketWebsite{
		MainPageSuffix: m["mainPageSuffix"],
		NotFoundPage:   m["notFoundPage"],
	}
}

// parseACLRules from json encoded string, return nil value on error (if any)
func parseACLRules(s string) []ACLRule {
	var rules []ACLRule
	_ = json.Unmarshal([]byte(s), &rules) // nolint: gosec
	return rules
}
