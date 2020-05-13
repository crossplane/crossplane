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

package hosted

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/packages"
)

const (
	namespace        = "cool-namespace"
	tenantSecretName = "foo"

	// hostResource represents the PackageInstall Job or Package Deployment
	hostResourceName      = "host-foo"
	hostResourceNamespace = "host-ns"
	hostResourceUID       = "host-uid"
)

type resource struct {
	name, namespace, uid string
	gvk                  schema.GroupVersionKind
}

var (
	testGVK = schema.GroupVersionKind{
		Group:   "group",
		Version: "version",
		Kind:    "Kind",
	}
	errBoom = errors.New("boom")
)

func secret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func testResource(name, namespace, uid string, gvk schema.GroupVersionKind) *resource {
	return &resource{
		name:      name,
		namespace: namespace,
		uid:       uid,
		gvk:       gvk,
	}
}

func (r *resource) GetName() string {
	return r.name
}

func (r *resource) GetNamespace() string {
	return r.namespace
}

func (r *resource) GetUID() types.UID {
	return types.UID(r.uid)
}

func (r *resource) GroupVersionKind() schema.GroupVersionKind {
	return r.gvk
}

func TestSyncImagePullSecrets(t *testing.T) {
	type args struct {
		ctx              context.Context
		tenantKube       client.Client
		hostKube         client.Client
		tenantNS         string
		tenantSecretRefs []corev1.LocalObjectReference
		hostSecretRefs   []corev1.LocalObjectReference
		hostObj          packages.KindlyIdentifier
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "NoSecrets",
			args: args{
				ctx:        context.TODO(),
				tenantKube: nil,
				tenantNS:   namespace,
				hostKube:   fake.NewFakeClient(),
				hostObj:    &v1alpha1.PackageInstall{},
			},
			wantErr: nil,
		},
		{
			name: "MissingHostSecretName",
			args: args{
				ctx:              context.TODO(),
				tenantKube:       fake.NewFakeClient(),
				hostKube:         fake.NewFakeClient(),
				tenantNS:         namespace,
				tenantSecretRefs: []corev1.LocalObjectReference{{Name: tenantSecretName}},
				hostSecretRefs:   []corev1.LocalObjectReference{},
				hostObj:          &v1alpha1.PackageInstall{},
			},
			wantErr: fmt.Errorf(errSecretNotFoundWithPrefixFmt, namespace+"."+tenantSecretName),
		},
		{
			name: "TenantSecretNotPresent",
			args: args{
				ctx:              context.TODO(),
				tenantKube:       fake.NewFakeClient(),
				hostKube:         fake.NewFakeClient(),
				tenantNS:         namespace,
				tenantSecretRefs: []corev1.LocalObjectReference{{Name: tenantSecretName}},
				hostSecretRefs:   []corev1.LocalObjectReference{{Name: namespace + "." + tenantSecretName}},
				hostObj:          &v1alpha1.PackageInstall{},
			},
			wantErr: kerrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, tenantSecretName),
		},
		{
			name: "HostSecretNotPresent",
			args: args{
				ctx:              context.TODO(),
				tenantKube:       fake.NewFakeClient(secret(tenantSecretName, namespace)),
				hostKube:         fake.NewFakeClient(),
				tenantNS:         namespace,
				tenantSecretRefs: []corev1.LocalObjectReference{{Name: tenantSecretName}},
				hostSecretRefs:   []corev1.LocalObjectReference{{Name: namespace + "." + tenantSecretName}},
				hostObj:          &v1alpha1.PackageInstall{},
			},
			wantErr: nil,
		},
		{
			name: "HostCreateFails",
			args: args{
				ctx:        context.TODO(),
				tenantKube: fake.NewFakeClient(secret(tenantSecretName, namespace)),
				hostKube: func() client.Client {
					fc := fake.NewFakeClient()
					c := &test.MockClient{
						MockList:   fc.List,
						MockCreate: test.NewMockCreateFn(errBoom),
					}
					return c
				}(),
				tenantNS:         namespace,
				tenantSecretRefs: []corev1.LocalObjectReference{{Name: tenantSecretName}},
				hostSecretRefs:   []corev1.LocalObjectReference{{Name: namespace + "." + tenantSecretName}},
				hostObj:          &v1alpha1.PackageInstall{},
			},
			wantErr: errBoom,
		},

		{
			name: "HostSecretPresentButMislabeled",
			args: args{
				ctx:        context.TODO(),
				tenantKube: fake.NewFakeClient(secret(tenantSecretName, namespace)),
				hostKube: func() client.Client {
					sec := secret(namespace+"."+tenantSecretName, hostResourceNamespace)

					fc := fake.NewFakeClient(sec)
					c := &test.MockClient{
						MockList:   fc.List,
						MockCreate: test.NewMockCreateFn(errBoom),
					}
					return c
				}(),
				tenantNS:         namespace,
				tenantSecretRefs: []corev1.LocalObjectReference{{Name: tenantSecretName}},
				hostSecretRefs:   []corev1.LocalObjectReference{{Name: namespace + "." + tenantSecretName}},
				hostObj:          testResource(hostResourceName, hostResourceNamespace, hostResourceUID, testGVK),
			},
			wantErr: errBoom,
		},
		{
			name: "HostSecretPresentAndLabeled",
			args: args{
				ctx:        context.TODO(),
				tenantKube: fake.NewFakeClient(secret(tenantSecretName, namespace)),
				hostKube: func() client.Client {
					sec := secret(namespace+"."+tenantSecretName, hostResourceNamespace)

					sec.SetLabels(map[string]string{
						labelForKind:     testGVK.Kind,
						labelForAPIGroup: testGVK.Group,
						labelForName:     hostResourceName,
					})

					fc := fake.NewFakeClient(sec)
					c := &test.MockClient{
						MockList:   fc.List,
						MockCreate: test.NewMockCreateFn(errBoom),
					}
					return c
				}(),
				tenantNS:         namespace,
				tenantSecretRefs: []corev1.LocalObjectReference{{Name: tenantSecretName}},
				hostSecretRefs:   []corev1.LocalObjectReference{{Name: namespace + "." + tenantSecretName}},
				hostObj:          testResource(hostResourceName, hostResourceNamespace, hostResourceUID, testGVK),
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := SyncImagePullSecrets(tt.args.ctx, tt.args.tenantKube, tt.args.hostKube, tt.args.tenantNS, tt.args.tenantSecretRefs, tt.args.hostSecretRefs, tt.args.hostObj)

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("SyncImagePullSecrets() -want error, +got error:\n%s", diff)
			}
		})
	}
}
