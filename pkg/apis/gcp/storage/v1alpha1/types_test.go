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
	"log"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/go-test/deep"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

var (
	cfg *rest.Config
	c   client.Client
	ctx = context.TODO()
)

func TestMain(m *testing.M) {
	err := SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	t := test.NewEnv("default", test.CRDs())
	cfg = t.Start()

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

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
		{"nil", nil, nil},
		{"val", testProjectTeam, testStorageProjectTeam},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToProjectTeam(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("CopyToProjectTeam() = %v, want %v\n%s", got, tt.want, diff)
			}
			gotBack := NewProjectTeam(got)
			if diff := deep.Equal(gotBack, tt.args); diff != nil {
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

func TestACLRule(t *testing.T) {
	tests := []struct {
		name string
		args ACLRule
		want storage.ACLRule
	}{
		{"default value args", ACLRule{}, storage.ACLRule{}},
		{"values", testACLRule, testStorageACLRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToACLRule(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("CopyToACLRule() = %v, want %v\n%s", got, tt.want, diff)
			}
			gotBack := NewACLRule(got)
			if diff := deep.Equal(gotBack, tt.args); diff != nil {
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
		{"nil", nil, nil},
		{"empty", []storage.ACLRule{}, nil},
		{"values", []storage.ACLRule{testStorageACLRule}, []ACLRule{testACLRule}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewACLRules(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"empty", []ACLRule{}, nil},
		{"values", []ACLRule{testACLRule}, []storage.ACLRule{testStorageACLRule}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToACLRules(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"val", testStorageLifecyleAction, testLifecycleAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLifecyleAction(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", testLifecycleAction, testStorageLifecyleAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToLifecyleAction(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", testStorageLifecycleCondition, testLifecycleCondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLifecycleCondition(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", testLifecycleCondition, testStorageLifecycleCondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToLifecycleCondition(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", testStorageLifecycleRule, testLifecycleRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLifecycleRule(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", testLifecycleRule, testStorageLifecycleRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToLifecyleRule(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"rules-nil", storage.Lifecycle{Rules: nil}, Lifecycle{Rules: nil}},
		{"rules-val", testStorageLifecycle, testLifecycle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLifecycle(tt.args)
			if diff := deep.Equal(*got, tt.want); diff != nil {
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
		{"rules-nil", Lifecycle{Rules: nil}, storage.Lifecycle{Rules: nil}},
		{"rules-val", Lifecycle{Rules: []LifecycleRule{testLifecycleRule}},
			storage.Lifecycle{Rules: []storage.LifecycleRule{testStorageLifecycleRule}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToLifecycle(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testStorageRetentionPolicy, testRetentionPolicy},
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
		{"nil", nil, &storage.RetentionPolicy{RetentionPeriod: time.Duration(0)}},
		{"val", testRetentionPolicy, testStorageRetentionPolicy},
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
		{"nil", nil, nil},
		{"val", testStorageRetentionPolicy, testRetentionPolicyStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewRetentionPolicyStatus(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testStorageBucketEncryption, testBucketEncryption},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketEncryption(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testBucketEncryption, testStorageBucketEncryption},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketEncryption(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testStorageBucketLogging, testBucketLogging},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketLogging(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testBucketLogging, testStorageBucketLogging},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketLogging(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", testStorageCORS, testCORS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCORS(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", testCORS, testStorageCORS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToCORS(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"empty", []storage.CORS{}, []CORS{}},
		{"val", []storage.CORS{testStorageCORS}, []CORS{testCORS}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCORSList(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"empty", []CORS{}, []storage.CORS{}},
		{"val", []CORS{testCORS}, []storage.CORS{testStorageCORS}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToCORSList(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testStorageBucketWebsite, testBucketWebsite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketWebsite(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testBucketWebsite, testStorageBucketWebsite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketWebsite(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testStorageBucketAttrs, testBucketUpdateAttrs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketUpdatableAttrs(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testBucketUpdateAttrs, testStorageBucketAttrs},
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
				if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", args{*testBucketUpdateAttrs, map[string]string{"application": "crossplane", "foo": "bar"}},
			testStorageBucketAttrsToUpdate},
	}
	for _, tt := range tests {
		tt.want.SetLabel("application", "crossplane")
		tt.want.DeleteLabel("foo")
		t.Run(tt.name, func(t *testing.T) {
			got := CopyToBucketUpdateAttrs(tt.args.ba, tt.args.labels)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, BucketSpecAttrs{}},
		{"val", testStorageBucketAttrs2, *testBucketSpecAttrs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketSpecAttrs(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, nil},
		{"val", testBucketSpecAttrs, testStorageBucketAttrs2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args != nil && tt.args.RetentionPolicy == nil && tt.want.RetentionPolicy == nil {
				tt.want.RetentionPolicy = &storage.RetentionPolicy{RetentionPeriod: time.Duration(0)}
			}
			got := CopyBucketSpecAttrs(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"nil", nil, BucketOutputAttrs{}},
		{"val", testStorageBucketAttrs3, testBucketOutputAttrs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketOutputAttrs(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"default", Bucket{}, ""},
		{"named", Bucket{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}, "foo"},
		{"override",
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

func TestBucket_ObjectReference(t *testing.T) {
	tests := []struct {
		name   string
		bucket Bucket
		want   *corev1.ObjectReference
	}{
		{"test", Bucket{}, &corev1.ObjectReference{APIVersion: APIVersion, Kind: BucketKind}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bucket.ObjectReference()
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"test", Bucket{}, metav1.OwnerReference{APIVersion: APIVersion, Kind: BucketKind}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bucket.OwnerReference()
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{"no conditions", b, false},
		{"running active", bReady, true},
		{"running and failed active", bReadyAndFailed, true},
		{"not running and failed active", bNotReadyAndFailed, false},
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
		{"bound", v1alpha1.BindingStateBound, true},
		{"not-bound", v1alpha1.BindingStateUnbound, false},
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
		{"not-bound", false, v1alpha1.BindingStateUnbound},
		{"bound", true, v1alpha1.BindingStateBound},
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

func Test_parseBool(t *testing.T) {
	tests := []struct {
		name string
		args string
		want bool
	}{
		{name: "empty", args: "", want: false},
		{name: "true", args: "true", want: true},
		{name: "True", args: "True", want: true},
		{name: "tRue", args: "tRue", want: false},
		{name: "_true", args: " true", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBool(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("parseBool() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_parseMap(t *testing.T) {
	tests := []struct {
		name string
		args string
		want map[string]string
	}{
		{name: "empty", args: "", want: map[string]string{}},
		{name: "single", args: "foo:bar", want: map[string]string{"foo": "bar"}},
		{name: "multi", args: "foo:bar, one:two", want: map[string]string{"foo": "bar", "one": "two"}},
		{name: "dupe key", args: "foo:bar,foo:buz", want: map[string]string{"foo": "buz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMap(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("parseMap() = %v, want %v\n%s", got, tt.want, diff)
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
		{name: "empty", args: "", want: nil},
		{name: "invalid", args: "foo", want: nil},
		{
			name: "valid",
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
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{name: "empty", args: "", want: &Lifecycle{}},
		{name: "invalid", args: "foo", want: &Lifecycle{}},
		{
			name: "valid",
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
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{name: "empty", args: "", want: &BucketLogging{}},
		{name: "invalid", args: "foo", want: &BucketLogging{}},
		{
			name: "valid",
			args: "logBucket:foo,logObjectPrefix:bar",
			want: &BucketLogging{LogBucket: "foo", LogObjectPrefix: "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLogging(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
		{name: "empty", args: "", want: &BucketWebsite{}},
		{name: "invalid", args: "foo", want: &BucketWebsite{}},
		{
			name: "valid",
			args: "mainPageSuffix:foo,notFoundPage:bar",
			want: &BucketWebsite{MainPageSuffix: "foo", NotFoundPage: "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWebsite(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
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
			name: "empty",
			args: "",
			want: nil,
		},
		{
			name: "invalid",
			args: "foo",
			want: nil,
		},
		{
			name: "single rule",
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
			name: "single rule",
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
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("parseACLRules() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestNewBucketSpec(t *testing.T) {
	tf := func(s string) time.Time {
		t, _ := time.Parse(time.RFC3339, s)
		return t
	}
	tests := []struct {
		name string
		args map[string]string
		want *BucketSpec
	}{
		{name: "empty", args: map[string]string{}, want: &BucketSpec{ReclaimPolicy: v1alpha1.ReclaimRetain}},
		{name: "invalid", args: map[string]string{"foo": "bar"}, want: &BucketSpec{ReclaimPolicy: v1alpha1.ReclaimRetain}},
		{
			name: "valid",
			args: map[string]string{
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
				"location":     "test-location",
				"storageClass": "test-storage-class",
			},
			want: &BucketSpec{
				ReclaimPolicy: v1alpha1.ReclaimRetain,
				BucketSpecAttrs: BucketSpecAttrs{
					BucketUpdatableAttrs: BucketUpdatableAttrs{
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBucketSpec(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("NewBucketSpec() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}
