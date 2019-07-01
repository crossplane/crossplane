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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
	name      = "test-instance"
)

var (
	c   client.Client
	ctx = context.TODO()
)

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, SchemeBuilder.SchemeBuilder, test.CRDs())
	c = t.StartClient()
	t.StopAndExit(m.Run())
}

func TestStorageCloudsqlInstance(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &CloudsqlInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: CloudsqlInstanceSpec{
			ResourceSpec: corev1alpha1.ResourceSpec{
				ProviderReference: &core.ObjectReference{},
			},
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &CloudsqlInstance{}
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

func TestNewCloudSQLInstanceSpec(t *testing.T) {
	tests := map[string]struct {
		args map[string]string
		want *CloudsqlInstanceSpec
	}{
		"Default": {
			args: map[string]string{},
			want: &CloudsqlInstanceSpec{
				AuthorizedNetworks: []string{},
				Labels:             map[string]string{},
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				StorageGB: DefaultStorageGB,
			},
		},
		"Values": {
			args: map[string]string{
				"databaseVersion": "POSTGRES_9_6",
				"labels":          "foo:bar,fizz:buzz,foo:notbar",
				"region":          "far-far-away",
				"storageGB":       "42",
				"storageType":     "special",
			},
			want: &CloudsqlInstanceSpec{
				AuthorizedNetworks: []string{},
				DatabaseVersion:    "POSTGRES_9_6",
				Labels: map[string]string{
					"fizz": "buzz",
					"foo":  "notbar",
				},
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				Region:      "far-far-away",
				StorageGB:   42,
				StorageType: "special",
			},
		},
		"SomeInvalidValues": {
			args: map[string]string{
				"authorizedNetworks": "1.1.1.1/1,0.0.0.0/0",
				"databaseVersion":    "POSTGRES_9_6",
				"labels":             "foo:bar,fizz:buzz,foo:notbar",
				"region":             "far-far-away",
				"storageGB":          "forty-two",
				"storageType":        "special",
			},
			want: &CloudsqlInstanceSpec{
				AuthorizedNetworks: []string{"1.1.1.1/1", "0.0.0.0/0"},
				DatabaseVersion:    "POSTGRES_9_6",
				Labels: map[string]string{
					"fizz": "buzz",
					"foo":  "notbar",
				},
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				Region:      "far-far-away",
				StorageGB:   DefaultStorageGB,
				StorageType: "special",
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewCloudSQLInstanceSpec(tt.args)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NewCloudSQLInstanceSpec() want(+), got(-): %s", diff)
			}
		})
	}
}

func TestCloudsqlInstance_ConnectionSecret(t *testing.T) {
	tests := map[string]struct {
		fields *CloudsqlInstance
		want   map[string][]byte
	}{
		"Default": {
			fields: &CloudsqlInstance{
				Spec: CloudsqlInstanceSpec{
					DatabaseVersion: "POSTGRES_9_6",
				},
			},
			want: map[string][]byte{
				corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(""),
				corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(PostgresqlDefaultUser),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, tt.fields.ConnectionSecret().Data); diff != "" {
				t.Errorf("ConnectionSecret() = %s", diff)
			}
		})
	}
}

func TestCloudsqlInstance_DatabaseInstance(t *testing.T) {
	type fields struct {
		Spec CloudsqlInstanceSpec
	}
	type args struct {
		name string
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   *sqladmin.DatabaseInstance
	}{
		"Default": {
			fields: fields{Spec: CloudsqlInstanceSpec{}},
			args:   args{name: "foo"},
			want: &sqladmin.DatabaseInstance{
				Name: "foo",
				Settings: &sqladmin.Settings{
					IpConfiguration: &sqladmin.IpConfiguration{
						AuthorizedNetworks: []*sqladmin.AclEntry{},
					},
				},
			},
		},
		"WithSpecs": {
			fields: fields{
				Spec: CloudsqlInstanceSpec{
					AuthorizedNetworks: []string{"foo", "bar"},
					DatabaseVersion:    "test-version",
					Labels:             map[string]string{"fooz": "booz"},
					Region:             "test-region",
					StorageGB:          42,
					StorageType:        "test-storage",
					Tier:               "test-tier",
				},
			},
			args: args{name: "test-name"},
			want: &sqladmin.DatabaseInstance{
				DatabaseVersion: "test-version",
				Name:            "test-name",
				Region:          "test-region",
				Settings: &sqladmin.Settings{
					DataDiskSizeGb: 42,
					DataDiskType:   "test-storage",
					IpConfiguration: &sqladmin.IpConfiguration{
						AuthorizedNetworks: []*sqladmin.AclEntry{
							{Value: "foo"},
							{Value: "bar"},
						},
					},
					Tier:       "test-tier",
					UserLabels: map[string]string{"fooz": "booz"},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &CloudsqlInstance{
				Spec: tt.fields.Spec,
			}
			if diff := cmp.Diff(tt.want, c.DatabaseInstance(tt.args.name)); diff != "" {
				t.Errorf("DatabaseInstance() = %s", diff)
			}
		})
	}
}

func TestCloudsqlInstance_DatabaseUserName(t *testing.T) {
	tests := map[string]struct {
		spec CloudsqlInstanceSpec
		want string
	}{
		"Default": {
			spec: CloudsqlInstanceSpec{},
			want: MysqlDefaultUser,
		},
		"Postgres": {
			spec: CloudsqlInstanceSpec{DatabaseVersion: "POSTGRES_9_6"},
			want: PostgresqlDefaultUser,
		},
		"MySQL": {
			spec: CloudsqlInstanceSpec{DatabaseVersion: "MYSQL_5_7"},
			want: MysqlDefaultUser,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &CloudsqlInstance{
				Spec: tt.spec,
			}
			if got := c.DatabaseUserName(); got != tt.want {
				t.Errorf("DatabaseUserName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBucket_GetResourceName(t *testing.T) {
	om := metav1.ObjectMeta{
		Namespace: "foo",
		Name:      "bar",
		UID:       "test-uid",
	}
	type fields struct {
		meta metav1.ObjectMeta
		spec CloudsqlInstanceSpec
	}
	tests := map[string]struct {
		fields fields
		want   string
	}{
		"NoNameFormat": {
			fields: fields{
				meta: om,
				spec: CloudsqlInstanceSpec{},
			},
			want: "test-uid",
		},
		"FormatString": {
			fields: fields{
				meta: om,
				spec: CloudsqlInstanceSpec{
					NameFormat: "foo-%s",
				},
			},
			want: "foo-test-uid",
		},
		"ConstantString": {
			fields: fields{
				meta: om,
				spec: CloudsqlInstanceSpec{
					NameFormat: "foo-bar",
				},
			},
			want: "foo-bar",
		},
		"InvalidMultipleSubstitutions": {
			fields: fields{
				meta: om,
				spec: CloudsqlInstanceSpec{
					NameFormat: "foo-%s-bar-%s",
				},
			},
			want: "foo-test-uid-bar-%!s(MISSING)",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b := &CloudsqlInstance{
				ObjectMeta: tt.fields.meta,
				Spec:       tt.fields.spec,
			}
			if got := b.GetResourceName(); got != tt.want {
				t.Errorf("Bucket.GetBucketName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloudsqlInstance_IsAvailable(t *testing.T) {
	tests := map[string]struct {
		status CloudsqlInstanceStatus
		want   bool
	}{
		"Default": {
			status: CloudsqlInstanceStatus{},
		},
		"Runnable": {
			status: CloudsqlInstanceStatus{
				State: StateRunnable,
			},
			want: true,
		},
		"NotRunnable": {
			status: CloudsqlInstanceStatus{
				State: "something-else",
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &CloudsqlInstance{
				Status: tt.status,
			}
			if got := c.IsAvailable(); got != tt.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloudsqlInstance_SetStatus(t *testing.T) {
	tests := map[string]struct {
		status CloudsqlInstanceStatus
		args   *sqladmin.DatabaseInstance
		want   CloudsqlInstanceStatus
	}{
		"Nil": {
			status: CloudsqlInstanceStatus{},
			args:   nil,
			want:   CloudsqlInstanceStatus{},
		},
		"Default": {
			status: CloudsqlInstanceStatus{},
			args:   &sqladmin.DatabaseInstance{},
			want: CloudsqlInstanceStatus{
				ResourceStatus: corev1alpha1.ResourceStatus{
					ConditionedStatus: corev1alpha1.ConditionedStatus{
						Conditions: []corev1alpha1.Condition{
							{
								Type:   corev1alpha1.TypeReady,
								Status: "False",
								Reason: "Managed resource is not available for use",
							},
						},
					},
				},
			},
		},
		"Available": {
			status: CloudsqlInstanceStatus{},
			args: &sqladmin.DatabaseInstance{
				IpAddresses: []*sqladmin.IpMapping{
					{
						IpAddress: "foo",
					},
				},
				State: StateRunnable,
			},
			want: CloudsqlInstanceStatus{
				ResourceStatus: corev1alpha1.ResourceStatus{
					ConditionedStatus: corev1alpha1.ConditionedStatus{
						Conditions: []corev1alpha1.Condition{
							{
								Type:   corev1alpha1.TypeReady,
								Status: "True",
								Reason: "Managed resource is available for use",
							},
						},
					},
				},
				Endpoint: "foo",
				State:    StateRunnable,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &CloudsqlInstance{
				Status: tt.status,
			}
			c.SetStatus(tt.args)
			if diff := cmp.Diff(tt.want, c.Status); diff != "" {
				t.Errorf("SetStatus() = %s", diff)
			}
		})
	}
}
