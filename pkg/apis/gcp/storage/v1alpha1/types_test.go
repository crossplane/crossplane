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
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const namespace = "default"

var (
	c   client.Client
	ctx = context.TODO()
)

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, SchemeBuilder.SchemeBuilder, test.CRDs())
	c = t.StartClient()
	t.StopAndExit(m.Run())
}

func TestStorageGCPBucket(t *testing.T) {
	key := types.NamespacedName{Name: "test", Namespace: "default"}
	created := &Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: BucketSpec{
			BucketSpecAttrs: BucketSpecAttrs{
				Location:     "US",
				StorageClass: "STANDARD",
			},
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &Bucket{}
	g.Expect(c.Create(ctx, created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(gomega.HaveOccurred())
}

var (
	testProjectTeam        = &ProjectTeam{ProjectNumber: "foo", Team: "bar"}
	testStorageProjectTeam = &storage.ProjectTeam{ProjectNumber: "foo", Team: "bar"}
)

func TestProjectTeam(t *testing.T) {
	tests := []struct {
		name string
		args *ProjectTeam
		want *storage.ProjectTeam
	}{
		{"Nil", nil, nil},
		{"Val", testProjectTeam, testStorageProjectTeam},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToProjectTeam(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToProjectTeam() = %v, want %v\n%s", got, tt.want, diff)
			}
			gotBack := NewProjectTeam(got)
			if diff := cmp.Diff(gotBack, tt.args); diff != "" {
				t.Errorf("NewProjectTeam() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}

}

var (
	testACLRule = ACLRule{
		Domain:      "test-domain",
		Email:       "test-email",
		EntityID:    "test-entity-id",
		Entity:      "test-entity",
		ProjectTeam: testProjectTeam,
		Role:        "role",
	}

	testStorageACLRule = storage.ACLRule{
		Domain:      "test-domain",
		Email:       "test-email",
		EntityID:    "test-entity-id",
		Entity:      "test-entity",
		ProjectTeam: testStorageProjectTeam,
		Role:        "role",
	}
)

func TestNewBucketPolicyOnly(t *testing.T) {
	tests := []struct {
		name string
		args storage.BucketPolicyOnly
		want BucketPolicyOnly
	}{
		{name: "Default", args: storage.BucketPolicyOnly{}, want: BucketPolicyOnly{}},
		{name: "Values", args: storage.BucketPolicyOnly{Enabled: true}, want: BucketPolicyOnly{Enabled: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketPolicyOnly(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewBucketPolicyOnly() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToBucketPolicyOnly(t *testing.T) {
	tests := []struct {
		name string
		args BucketPolicyOnly
		want storage.BucketPolicyOnly
	}{
		{name: "Default", args: BucketPolicyOnly{}, want: storage.BucketPolicyOnly{}},
		{name: "Values", args: BucketPolicyOnly{Enabled: true}, want: storage.BucketPolicyOnly{Enabled: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketPolicyOnly(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToBucketPolicyOnly() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestACLRule(t *testing.T) {
	tests := []struct {
		name string
		args ACLRule
		want storage.ACLRule
	}{
		{"DefaultValueArgs", ACLRule{}, storage.ACLRule{}},
		{"Values", testACLRule, testStorageACLRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToACLRule(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToACLRule() = %v, want %v\n%s", got, tt.want, diff)
			}
			gotBack := NewACLRule(got)
			if diff := cmp.Diff(gotBack, tt.args); diff != "" {
				t.Errorf("NewACLRule() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestNewACLRules(t *testing.T) {
	tests := []struct {
		name string
		args []storage.ACLRule
		want []ACLRule
	}{
		{"Nil", nil, nil},
		{"Empty", []storage.ACLRule{}, nil},
		{"Values", []storage.ACLRule{testStorageACLRule}, []ACLRule{testACLRule}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewACLRules(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToACLRules() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToACLRules(t *testing.T) {
	tests := []struct {
		name string
		args []ACLRule
		want []storage.ACLRule
	}{
		{"Nil", nil, nil},
		{"Empty", []ACLRule{}, nil},
		{"Values", []ACLRule{testACLRule}, []storage.ACLRule{testStorageACLRule}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToACLRules(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToACLRules() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testLifecycleAction = LifecycleAction{
		StorageClass: "STANDARD",
		Type:         "SetStorageClass",
	}

	testStorageLifecyleAction = storage.LifecycleAction{
		StorageClass: "STANDARD",
		Type:         "SetStorageClass",
	}
)

func TestNewLifecyleAction(t *testing.T) {
	tests := []struct {
		name string
		args storage.LifecycleAction
		want LifecycleAction
	}{
		{"Val", testStorageLifecyleAction, testLifecycleAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLifecyleAction(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewLifecyleAction() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToLifecyleAction(t *testing.T) {
	tests := []struct {
		name string
		args LifecycleAction
		want storage.LifecycleAction
	}{
		{"Test", testLifecycleAction, testStorageLifecyleAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToLifecyleAction(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToLifecyleAction() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	now = time.Now()

	testLifecycleCondition = LifecycleCondition{
		AgeInDays:             10,
		CreatedBefore:         metav1.NewTime(now.Add(24 * time.Hour)),
		Liveness:              storage.Liveness(1),
		MatchesStorageClasses: []string{"STANDARD"},
		NumNewerVersions:      5,
	}

	testStorageLifecycleCondition = storage.LifecycleCondition{
		AgeInDays:             10,
		CreatedBefore:         now.Add(24 * time.Hour),
		Liveness:              storage.Liveness(1),
		MatchesStorageClasses: []string{"STANDARD"},
		NumNewerVersions:      5,
	}
)

func TestNewLifecycleCondition(t *testing.T) {
	tests := []struct {
		name string
		args storage.LifecycleCondition
		want LifecycleCondition
	}{
		{"Test", testStorageLifecycleCondition, testLifecycleCondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLifecycleCondition(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewLifecycleCondition() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToLifecycleCondition(t *testing.T) {
	tests := []struct {
		name string
		args LifecycleCondition
		want storage.LifecycleCondition
	}{
		{"Test", testLifecycleCondition, testStorageLifecycleCondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToLifecycleCondition(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToLifecycleCondition() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testLifecycleRule = LifecycleRule{
		Action:    testLifecycleAction,
		Condition: testLifecycleCondition,
	}

	testStorageLifecycleRule = storage.LifecycleRule{
		Action:    testStorageLifecyleAction,
		Condition: testStorageLifecycleCondition,
	}
)

func TestNewLifecycleRule(t *testing.T) {
	tests := []struct {
		name string
		args storage.LifecycleRule
		want LifecycleRule
	}{
		{"Test", testStorageLifecycleRule, testLifecycleRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLifecycleRule(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewLifecycleRule() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToLifecyleRule(t *testing.T) {
	tests := []struct {
		name string
		args LifecycleRule
		want storage.LifecycleRule
	}{
		{"Test", testLifecycleRule, testStorageLifecycleRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToLifecyleRule(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToLifecyleRule() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testLifecycle        = Lifecycle{Rules: []LifecycleRule{testLifecycleRule}}
	testStorageLifecycle = storage.Lifecycle{Rules: []storage.LifecycleRule{testStorageLifecycleRule}}
)

func TestNewLifecycle(t *testing.T) {
	tests := []struct {
		name string
		args storage.Lifecycle
		want Lifecycle
	}{
		{"RulesNil", storage.Lifecycle{Rules: nil}, Lifecycle{Rules: nil}},
		{"RulesVal", testStorageLifecycle, testLifecycle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLifecycle(tt.args)
			if diff := cmp.Diff(*got, tt.want); diff != "" {
				t.Errorf("NewLifecycle() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToLifecycle(t *testing.T) {
	tests := []struct {
		name string
		args Lifecycle
		want storage.Lifecycle
	}{
		{"RulesNil", Lifecycle{Rules: nil}, storage.Lifecycle{Rules: nil}},
		{"RulesVal", Lifecycle{Rules: []LifecycleRule{testLifecycleRule}},
			storage.Lifecycle{Rules: []storage.LifecycleRule{testStorageLifecycleRule}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToLifecycle(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToLifecycle() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testStorageRetentionPolicy = &storage.RetentionPolicy{
		EffectiveTime:   now,
		IsLocked:        true,
		RetentionPeriod: 100 * time.Second,
	}

	testRetentionPolicy = &RetentionPolicy{RetentionPeriodSeconds: 100}

	testRetentionPolicyStatus = &RetentionPolicyStatus{
		EffectiveTime: metav1.NewTime(now),
		IsLocked:      true,
	}
)

func TestNewRetentionPolicy(t *testing.T) {
	tests := []struct {
		name string
		args *storage.RetentionPolicy
		want *RetentionPolicy
	}{
		{"Nil", nil, nil},
		{"Val", testStorageRetentionPolicy, testRetentionPolicy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewRetentionPolicy(tt.args)
			if tt.want == nil {
				if got != nil {
					t.Errorf("NewRetentionPolicy() = %v, want %v", got, tt.want)
				}
			} else {
				if tt.want.RetentionPeriodSeconds != got.RetentionPeriodSeconds {
					t.Errorf("NewRetentionPolicy() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestCopyToRetentionPolicy(t *testing.T) {
	tests := []struct {
		name string
		args *RetentionPolicy
		want *storage.RetentionPolicy
	}{
		{"Nil", nil, &storage.RetentionPolicy{RetentionPeriod: time.Duration(0)}},
		{"Val", testRetentionPolicy, testStorageRetentionPolicy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToRetentionPolicy(tt.args)
			if tt.want == nil {
				if got != nil {
					t.Errorf("NewRetentionPolicy() = %v, want %v", got, tt.want)
				}
			} else {
				if tt.want.RetentionPeriod != got.RetentionPeriod {
					t.Errorf("CopyToRetentionPolicy() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestNewRetentionPolicyStatus(t *testing.T) {
	tests := []struct {
		name string
		args *storage.RetentionPolicy
		want *RetentionPolicyStatus
	}{
		{"Nil", nil, nil},
		{"Val", testStorageRetentionPolicy, testRetentionPolicyStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewRetentionPolicyStatus(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewRetentionPolicyStatus() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testBucketEncryption        = &BucketEncryption{DefaultKMSKeyName: "test-kms"}
	testStorageBucketEncryption = &storage.BucketEncryption{DefaultKMSKeyName: "test-kms"}
)

func TestNewBucketEncryption(t *testing.T) {
	tests := []struct {
		name string
		args *storage.BucketEncryption
		want *BucketEncryption
	}{
		{"Nil", nil, nil},
		{"Val", testStorageBucketEncryption, testBucketEncryption},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketEncryption(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewBucketEncryption() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToBucketEncryption(t *testing.T) {
	tests := []struct {
		name string
		args *BucketEncryption
		want *storage.BucketEncryption
	}{
		{"Nil", nil, nil},
		{"Val", testBucketEncryption, testStorageBucketEncryption},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketEncryption(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToBucketEncryption() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testBucketLogging = &BucketLogging{
		LogBucket:       "dest-bucket",
		LogObjectPrefix: "test-prefix",
	}

	testStorageBucketLogging = &storage.BucketLogging{
		LogBucket:       "dest-bucket",
		LogObjectPrefix: "test-prefix",
	}
)

func TestNewBucketLogging(t *testing.T) {
	tests := []struct {
		name string
		args *storage.BucketLogging
		want *BucketLogging
	}{
		{"Nil", nil, nil},
		{"Val", testStorageBucketLogging, testBucketLogging},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketLogging(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewBucketLogging() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToBucketLogging(t *testing.T) {
	tests := []struct {
		name string
		args *BucketLogging
		want *storage.BucketLogging
	}{
		{"Nil", nil, nil},
		{"Val", testBucketLogging, testStorageBucketLogging},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketLogging(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToBucketLogging() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testCORS = CORS{
		MaxAge:          metav1.Duration{Duration: 1 * time.Minute},
		Methods:         []string{"GET", "POST"},
		Origins:         []string{},
		ResponseHeaders: nil,
	}

	testStorageCORS = storage.CORS{
		MaxAge:          1 * time.Minute,
		Methods:         []string{"GET", "POST"},
		Origins:         []string{},
		ResponseHeaders: nil,
	}
)

func TestNewCORS(t *testing.T) {
	tests := []struct {
		name string
		args storage.CORS
		want CORS
	}{
		{"Test", testStorageCORS, testCORS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCORS(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewCORS() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToCORS(t *testing.T) {
	tests := []struct {
		name string
		args CORS
		want storage.CORS
	}{
		{"Test", testCORS, testStorageCORS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToCORS(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToCORS() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestNewCORSs(t *testing.T) {
	tests := []struct {
		name string
		args []storage.CORS
		want []CORS
	}{
		{"Nil", nil, nil},
		{"Empty", []storage.CORS{}, []CORS{}},
		{"Val", []storage.CORS{testStorageCORS}, []CORS{testCORS}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCORSList(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewCORSList() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToCORSs(t *testing.T) {
	tests := []struct {
		name string
		args []CORS
		want []storage.CORS
	}{
		{"Nil", nil, nil},
		{"Empty", []CORS{}, []storage.CORS{}},
		{"Val", []CORS{testCORS}, []storage.CORS{testStorageCORS}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToCORSList(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToCORSList() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testBucketWebsite        = &BucketWebsite{MainPageSuffix: "test-sfx", NotFoundPage: "oh-no"}
	testStorageBucketWebsite = &storage.BucketWebsite{MainPageSuffix: "test-sfx", NotFoundPage: "oh-no"}
)

func TestNewBucketWebsite(t *testing.T) {
	tests := []struct {
		name string
		args *storage.BucketWebsite
		want *BucketWebsite
	}{
		{"Nil", nil, nil},
		{"Val", testStorageBucketWebsite, testBucketWebsite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketWebsite(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewBucketWebsite() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToBucketWebsite(t *testing.T) {
	tests := []struct {
		name string
		args *BucketWebsite
		want *storage.BucketWebsite
	}{
		{"Nil", nil, nil},
		{"Val", testBucketWebsite, testStorageBucketWebsite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketWebsite(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToBucketWebsite() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testBucketUpdateAttrs = &BucketUpdatableAttrs{
		CORS:                       []CORS{testCORS},
		DefaultEventBasedHold:      true,
		Encryption:                 testBucketEncryption,
		Labels:                     map[string]string{"application": "crossplane"},
		Lifecycle:                  testLifecycle,
		Logging:                    testBucketLogging,
		PredefinedACL:              "test-predefined-acl",
		PredefinedDefaultObjectACL: "test-predefined-default-object-acl",
		RequesterPays:              true,
		RetentionPolicy:            nil,
		VersioningEnabled:          true,
		Website:                    testBucketWebsite,
	}

	testStorageBucketAttrs = &storage.BucketAttrs{
		CORS:                       []storage.CORS{testStorageCORS},
		DefaultEventBasedHold:      true,
		Encryption:                 testStorageBucketEncryption,
		Labels:                     map[string]string{"application": "crossplane"},
		Lifecycle:                  testStorageLifecycle,
		Logging:                    testStorageBucketLogging,
		PredefinedACL:              "test-predefined-acl",
		PredefinedDefaultObjectACL: "test-predefined-default-object-acl",
		RequesterPays:              true,
		RetentionPolicy:            nil,
		VersioningEnabled:          true,
		Website:                    testStorageBucketWebsite,
	}

	testStorageBucketAttrsToUpdate = storage.BucketAttrsToUpdate{
		BucketPolicyOnly:           &storage.BucketPolicyOnly{},
		CORS:                       []storage.CORS{testStorageCORS},
		DefaultEventBasedHold:      true,
		Encryption:                 testStorageBucketEncryption,
		Lifecycle:                  &testStorageLifecycle,
		Logging:                    testStorageBucketLogging,
		PredefinedACL:              "test-predefined-acl",
		PredefinedDefaultObjectACL: "test-predefined-default-object-acl",
		RequesterPays:              true,
		RetentionPolicy:            &storage.RetentionPolicy{RetentionPeriod: time.Duration(0)},
		VersioningEnabled:          true,
		Website:                    testStorageBucketWebsite,
	}
)

func TestNewBucketUpdateAttrs(t *testing.T) {
	tests := []struct {
		name string
		args *storage.BucketAttrs
		want *BucketUpdatableAttrs
	}{
		{"Nil", nil, nil},
		{"Val", testStorageBucketAttrs, testBucketUpdateAttrs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketUpdatableAttrs(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewBucketUpdatableAttrs() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyToBucketAttrs(t *testing.T) {
	tests := []struct {
		name string
		args *BucketUpdatableAttrs
		want *storage.BucketAttrs
	}{
		{"Nil", nil, nil},
		{"Val", testBucketUpdateAttrs, testStorageBucketAttrs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args == nil {
				if got := CopyToBucketAttrs(tt.args); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("CopyToBucketAttrs() = %+v, want %+v", got, tt.want)
				}
			} else {
				got := CopyToBucketAttrs(tt.args)
				got.RetentionPolicy = nil
				if diff := cmp.Diff(got, tt.want); diff != "" {
					t.Errorf("CopyToBucketAttrs() = %+v, want %+v\n%s", got, tt.want, diff)
				}
			}
		})
	}
}

func TestCopyToBucketUpdateAttrs(t *testing.T) {
	type args struct {
		ba     BucketUpdatableAttrs
		labels map[string]string
	}
	tests := []struct {
		name string
		args args
		want storage.BucketAttrsToUpdate
	}{
		{
			name: "Test",
			args: args{*testBucketUpdateAttrs, map[string]string{"application": "crossplane", "foo": "bar"}},
			want: testStorageBucketAttrsToUpdate,
		},
	}
	for _, tt := range tests {
		tt.want.SetLabel("application", "crossplane")
		tt.want.DeleteLabel("foo")
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketUpdateAttrs(tt.args.ba, tt.args.labels)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyToBucketUpdateAttrs()\n%+v, want \n%+v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testBucketSpecAttrs = &BucketSpecAttrs{
		BucketUpdatableAttrs: *testBucketUpdateAttrs,
		ACL:                  []ACLRule{testACLRule},
		DefaultObjectACL:     nil,
		Location:             "US",
		StorageClass:         "STANDARD",
	}

	testStorageBucketAttrs2 = &storage.BucketAttrs{
		ACL:                        []storage.ACLRule{testStorageACLRule},
		CORS:                       []storage.CORS{testStorageCORS},
		DefaultEventBasedHold:      true,
		DefaultObjectACL:           nil,
		Encryption:                 testStorageBucketEncryption,
		Labels:                     map[string]string{"application": "crossplane"},
		Lifecycle:                  testStorageLifecycle,
		Location:                   "US",
		Logging:                    testStorageBucketLogging,
		PredefinedACL:              "test-predefined-acl",
		PredefinedDefaultObjectACL: "test-predefined-default-object-acl",
		RequesterPays:              true,
		RetentionPolicy:            nil,
		StorageClass:               "STANDARD",
		VersioningEnabled:          true,
		Website:                    testStorageBucketWebsite,
	}
)

func TestNewBucketSpecAttrs(t *testing.T) {
	tests := []struct {
		name string
		args *storage.BucketAttrs
		want BucketSpecAttrs
	}{
		{"Nil", nil, BucketSpecAttrs{}},
		{"Val", testStorageBucketAttrs2, *testBucketSpecAttrs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketSpecAttrs(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewBucketSpecAttrs() = \n%+v, want \n%+v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestCopyBucketSpecAttrs(t *testing.T) {
	tests := []struct {
		name string
		args *BucketSpecAttrs
		want *storage.BucketAttrs
	}{
		{"Nil", nil, nil},
		{"Val", testBucketSpecAttrs, testStorageBucketAttrs2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args != nil && tt.args.RetentionPolicy == nil && tt.want.RetentionPolicy == nil {
				tt.want.RetentionPolicy = &storage.RetentionPolicy{RetentionPeriod: time.Duration(0)}
			}
			got := CopyBucketSpecAttrs(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CopyBucketSpecAttrs() = \n%+v, want \n%+v\n%s", got, tt.want, diff)
			}
		})
	}
}

var (
	testBucketOutputAttrs = BucketOutputAttrs{
		Created:         metav1.NewTime(now),
		Name:            "test-name",
		RetentionPolicy: testRetentionPolicyStatus,
	}

	testStorageBucketAttrs3 = &storage.BucketAttrs{
		Created:         now,
		Name:            "test-name",
		RetentionPolicy: testStorageRetentionPolicy,
	}
)

func TestNewBucketOutputAttrs(t *testing.T) {
	tests := []struct {
		name string
		args *storage.BucketAttrs
		want BucketOutputAttrs
	}{
		{"Nil", nil, BucketOutputAttrs{}},
		{"Val", testStorageBucketAttrs3, testBucketOutputAttrs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketOutputAttrs(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewBucketOutputAttrs() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestBucket_ConnectionSecretName(t *testing.T) {
	tests := []struct {
		name   string
		bucket Bucket
		want   string
	}{
		{"Default", Bucket{}, ""},
		{"Named", Bucket{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}, "foo"},
		{"Override",
			Bucket{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec:       BucketSpec{ConnectionSecretNameOverride: "bar"}}, "bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bucket.ConnectionSecretName(); got != tt.want {
				t.Errorf("Bucket.ConnectionSecretName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBucket_ConnectionSecret(t *testing.T) {
	bucket := Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "bucket",
			UID:       "test-uid",
		},
	}
	tests := []struct {
		name   string
		bucket Bucket
		want   *corev1.Secret
	}{
		{
			name:   "test",
			bucket: bucket,
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       "default",
					Name:            "bucket",
					OwnerReferences: []metav1.OwnerReference{bucket.OwnerReference()},
				},
				Data: map[string][]byte{
					corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(bucket.GetBucketName()),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bucket.ConnectionSecret()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("Bucket.ConnectionSecret() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestBucket_ObjectReference(t *testing.T) {
	tests := []struct {
		name   string
		bucket Bucket
		want   *corev1.ObjectReference
	}{
		{"Test", Bucket{}, &corev1.ObjectReference{APIVersion: APIVersion, Kind: BucketKind}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bucket.ObjectReference()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("Bucket.ObjectReference() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestBucket_OwnerReference(t *testing.T) {
	tests := []struct {
		name   string
		bucket Bucket
		want   metav1.OwnerReference
	}{
		{"Test", Bucket{}, metav1.OwnerReference{APIVersion: APIVersion, Kind: BucketKind}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bucket.OwnerReference()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("Bucket.OwnerReference() = \n%+v, want \n%+v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestBucket_IsAvailable(t *testing.T) {
	b := Bucket{}

	bReady := b
	bReady.Status.SetReady()

	bReadyAndFailed := bReady
	bReadyAndFailed.Status.SetFailed("", "")

	bNotReadyAndFailed := bReadyAndFailed
	bNotReadyAndFailed.Status.UnsetCondition(v1alpha1.Ready)

	tests := []struct {
		name   string
		bucket Bucket
		want   bool
	}{
		{"NoConditions", b, false},
		{"RunningActive", bReady, true},
		{"RunningAndFailedActive", bReadyAndFailed, true},
		{"NotRunningAndFailedActive", bNotReadyAndFailed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bucket.IsAvailable(); got != tt.want {
				t.Errorf("Bucket.IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBucket_IsBound(t *testing.T) {
	tests := []struct {
		name  string
		phase v1alpha1.BindingState
		want  bool
	}{
		{"Bound", v1alpha1.BindingStateBound, true},
		{"NotBound", v1alpha1.BindingStateUnbound, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := Bucket{
				Status: BucketStatus{
					BindingStatusPhase: v1alpha1.BindingStatusPhase{
						Phase: tt.phase,
					},
				},
			}
			if got := b.IsBound(); got != tt.want {
				t.Errorf("Bucket.IsBound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBucket_SetBound(t *testing.T) {
	tests := []struct {
		name  string
		state bool
		want  v1alpha1.BindingState
	}{
		{"NotBound", false, v1alpha1.BindingStateUnbound},
		{"Bound", true, v1alpha1.BindingStateBound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Bucket{}
			c.SetBound(tt.state)
			if c.Status.Phase != tt.want {
				t.Errorf("Bucket.SetBound(%v) = %v, want %v", tt.state, c.Status.Phase, tt.want)
			}
		})
	}
}

func Test_parseCORSList(t *testing.T) {
	tests := []struct {
		name string
		args string
		want []CORS
	}{
		{name: "Empty", args: "", want: nil},
		{name: "Invalid", args: "foo", want: nil},
		{
			name: "Valid",
			args: `[{"maxAge":"1s","methods":["GET","POST"],"origins":["foo","bar"]}]`,
			want: []CORS{
				{
					MaxAge:          metav1.Duration{Duration: 1 * time.Second},
					Methods:         []string{"GET", "POST"},
					Origins:         []string{"foo", "bar"},
					ResponseHeaders: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCORSList(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("parseCORSList() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_parseLifecycle(t *testing.T) {
	tf := func(s string) time.Time {
		t, _ := time.Parse(time.RFC3339, s)
		return t
	}
	tests := []struct {
		name string
		args string
		want *Lifecycle
	}{
		{name: "Empty", args: "", want: &Lifecycle{}},
		{name: "Invalid", args: "foo", want: &Lifecycle{}},
		{
			name: "Valid",
			args: `{"rules":[{"action":{"storageClass":"test-storage-class","type":"test-action-type"},` +
				`"condition":{"ageInDays":10,"createdBefore":"2019-03-26T21:58:58Z",` +
				`"liveness":3,"matchesStorageClasses":["foo","bar"],"numNewerVersions":42}}]}`,
			want: &Lifecycle{
				Rules: []LifecycleRule{
					{
						Action: LifecycleAction{
							StorageClass: "test-storage-class",
							Type:         "test-action-type",
						},
						Condition: LifecycleCondition{
							AgeInDays:             10,
							CreatedBefore:         metav1.NewTime(tf("2019-03-26T21:58:58Z")),
							Liveness:              3,
							MatchesStorageClasses: []string{"foo", "bar"},
							NumNewerVersions:      42,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLifecycle(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("parseLifecycle() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_parseLogging(t *testing.T) {
	tests := []struct {
		name string
		args string
		want *BucketLogging
	}{
		{name: "Empty", args: "", want: &BucketLogging{}},
		{name: "Invalid", args: "foo", want: &BucketLogging{}},
		{
			name: "Valid",
			args: "logBucket:foo,logObjectPrefix:bar",
			want: &BucketLogging{LogBucket: "foo", LogObjectPrefix: "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLogging(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("parseLogging() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_parseWebsite(t *testing.T) {
	tests := []struct {
		name string
		args string
		want *BucketWebsite
	}{
		{name: "Empty", args: "", want: &BucketWebsite{}},
		{name: "Invalid", args: "foo", want: &BucketWebsite{}},
		{
			name: "Valid",
			args: "mainPageSuffix:foo,notFoundPage:bar",
			want: &BucketWebsite{MainPageSuffix: "foo", NotFoundPage: "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWebsite(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("parseWebsite() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_parseACLRules(t *testing.T) {
	tests := []struct {
		name string
		args string
		want []ACLRule
	}{
		{
			name: "Empty",
			args: "",
			want: nil,
		},
		{
			name: "Invalid",
			args: "foo",
			want: nil,
		},
		{
			name: "SingleRule",
			args: `[{"Entity":"test-entity","EntityID":"42","Role":"test-role","Domain":"test-domain","Email":"test-email","ProjectTeam":{"ProjectNumber":"test-project-number","Team":"test-team"}}]`,
			want: []ACLRule{
				{
					Entity:   "test-entity",
					EntityID: "42",
					Role:     "test-role",
					Domain:   "test-domain",
					Email:    "test-email",
					ProjectTeam: &ProjectTeam{
						ProjectNumber: "test-project-number",
						Team:          "test-team",
					},
				},
			},
		},
		{
			name: "SingleRule",
			args: `[{"Entity":"test-entity","EntityID":"42","Role":"test-role","Domain":"test-domain","Email":"test-email","ProjectTeam":{"ProjectNumber":"test-project-number","Team":"test-team"}},` +
				`{"Entity":"another-entity","EntityID":"42","Role":"test-role","Domain":"test-domain","Email":"test-email","ProjectTeam":{"ProjectNumber":"test-project-number","Team":"test-team"}}]`,
			want: []ACLRule{
				{
					Entity:   "test-entity",
					EntityID: "42",
					Role:     "test-role",
					Domain:   "test-domain",
					Email:    "test-email",
					ProjectTeam: &ProjectTeam{
						ProjectNumber: "test-project-number",
						Team:          "test-team",
					},
				},
				{
					Entity:   "another-entity",
					EntityID: "42",
					Role:     "test-role",
					Domain:   "test-domain",
					Email:    "test-email",
					ProjectTeam: &ProjectTeam{
						ProjectNumber: "test-project-number",
						Team:          "test-team",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseACLRules(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("parseACLRules() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestParseBucketSpec(t *testing.T) {
	tf := func(s string) time.Time {
		t, _ := time.Parse(time.RFC3339, s)
		return t
	}
	tests := []struct {
		name string
		args map[string]string
		want *BucketSpec
	}{
		{name: "Empty", args: map[string]string{}, want: &BucketSpec{ReclaimPolicy: v1alpha1.ReclaimRetain}},
		{name: "Invalid", args: map[string]string{"foo": "bar"}, want: &BucketSpec{ReclaimPolicy: v1alpha1.ReclaimRetain}},
		{
			name: "Valid",
			args: map[string]string{
				"bucketPolicyOnly":            "true",
				"cors":                        `[{"maxAge":"1s","methods":["GET","POST"],"origins":["foo","bar"]}]`,
				"defaultEventBasedHold":       "true",
				"encryptionDefaultKmsKeyName": "test-encryption",
				"labels":                      "foo:bar",
				"lifecycle": `{"rules":[{"action":{"storageClass":"test-storage-class","type":"test-action-type"},` +
					`"condition":{"ageInDays":10,"createdBefore":"2019-03-26T21:58:58Z",` +
					`"liveness":3,"matchesStorageClasses":["foo","bar"],"numNewerVersions":42}}]}`,
				"logging":                    "logBucket:foo,logObjectPrefix:bar",
				"website":                    "mainPageSuffix:foo,notFoundPage:bar",
				"predefinedACL":              "test-predefined-acl",
				"predefinedDefaultObjectACL": "test-predefined-default-object-acl",
				"acl": `[{"Entity":"test-entity","EntityID":"42","Role":"test-role","Domain":"test-domain",` +
					`"Email":"test-email","ProjectTeam":{"ProjectNumber":"test-project-number","Team":"test-team"}}]`,
				"location":                "test-location",
				"storageClass":            "test-storage-class",
				"serviceAccountSecretRef": "testAccount",
			},
			want: &BucketSpec{
				BucketSpecAttrs: BucketSpecAttrs{
					BucketUpdatableAttrs: BucketUpdatableAttrs{
						BucketPolicyOnly: BucketPolicyOnly{
							Enabled: true,
						},
						CORS: []CORS{
							{
								MaxAge:          metav1.Duration{Duration: 1 * time.Second},
								Methods:         []string{"GET", "POST"},
								Origins:         []string{"foo", "bar"},
								ResponseHeaders: nil,
							},
						},
						DefaultEventBasedHold: true,
						Encryption:            &BucketEncryption{DefaultKMSKeyName: "test-encryption"},
						Labels:                map[string]string{"foo": "bar"},
						Lifecycle: Lifecycle{
							Rules: []LifecycleRule{
								{
									Action: LifecycleAction{
										StorageClass: "test-storage-class",
										Type:         "test-action-type",
									},
									Condition: LifecycleCondition{
										AgeInDays:             10,
										CreatedBefore:         metav1.NewTime(tf("2019-03-26T21:58:58Z")),
										Liveness:              3,
										MatchesStorageClasses: []string{"foo", "bar"},
										NumNewerVersions:      42,
									},
								},
							},
						},
						Logging:                    &BucketLogging{LogBucket: "foo", LogObjectPrefix: "bar"},
						Website:                    &BucketWebsite{MainPageSuffix: "foo", NotFoundPage: "bar"},
						PredefinedACL:              "test-predefined-acl",
						PredefinedDefaultObjectACL: "test-predefined-default-object-acl",
					},
					ACL: []ACLRule{
						{
							Entity:   "test-entity",
							EntityID: "42",
							Role:     "test-role",
							Domain:   "test-domain",
							Email:    "test-email",
							ProjectTeam: &ProjectTeam{
								ProjectNumber: "test-project-number",
								Team:          "test-team",
							},
						},
					},
					Location:     "test-location",
					StorageClass: "test-storage-class",
				},
				ReclaimPolicy:           v1alpha1.ReclaimRetain,
				ServiceAccountSecretRef: &corev1.LocalObjectReference{Name: "testAccount"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBucketSpec(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("ParseBucketSpec() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestBucket_GetBucketName(t *testing.T) {
	om := metav1.ObjectMeta{
		Namespace: "foo",
		Name:      "bar",
		UID:       "test-uid",
	}
	type fields struct {
		ObjectMeta metav1.ObjectMeta
		Spec       BucketSpec
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "NoNameFormat",
			fields: fields{
				ObjectMeta: om,
				Spec:       BucketSpec{},
			},
			want: "test-uid",
		},
		{
			name: "FormatString",
			fields: fields{
				ObjectMeta: om,
				Spec: BucketSpec{
					NameFormat: "foo-%s",
				},
			},
			want: "foo-test-uid",
		},
		{
			name: "ConstantString",
			fields: fields{
				ObjectMeta: om,
				Spec: BucketSpec{
					NameFormat: "foo-bar",
				},
			},
			want: "foo-bar",
		},
		{
			name: "InvalidMultipleSubstitutions",
			fields: fields{
				ObjectMeta: om,
				Spec: BucketSpec{
					NameFormat: "foo-%s-bar-%s",
				},
			},
			want: "foo-test-uid-bar-%!s(MISSING)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Bucket{
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			if got := b.GetBucketName(); got != tt.want {
				t.Errorf("Bucket.GetBucketName() = %v, want %v", got, tt.want)
			}
		})
	}
}
