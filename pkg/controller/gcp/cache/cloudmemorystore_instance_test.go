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

package cache

import (
	"context"
	"testing"
	"time"

	redisv1 "cloud.google.com/go/redis/apiv1"
	"github.com/google/go-cmp/cmp"
	gax "github.com/googleapis/gax-go"
	"github.com/pkg/errors"
	redisv1pb "google.golang.org/genproto/googleapis/cloud/redis/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/cache/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudmemorystore"
	fakecloudmemorystore "github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudmemorystore/fake"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace         = "cool-namespace"
	uid               = types.UID("definitely-a-uuid")
	region            = "us-cool1"
	project           = "coolProject"
	instanceName      = cloudmemorystore.NamePrefix + "-" + string(uid)
	qualifiedName     = "projects/" + project + "/locations/" + region + "/instances/" + instanceName
	authorizedNetwork = "default"
	memorySizeGB      = 1
	host              = "172.16.0.1"
	port              = 6379

	providerName       = "cool-gcp"
	providerSecretName = "cool-gcp-secret"
	providerSecretKey  = "credentials.json"
	providerSecretData = "definitelyjson"

	connectionSecretName = "cool-connection-secret"
)

var (
	ctx          = context.Background()
	errorBoom    = errors.New("boom")
	redisConfigs = map[string]string{"cool": "socool"}

	provider = gcpv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerName},
		Spec: gcpv1alpha1.ProviderSpec{
			ProjectID: project,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: providerSecretName},
				Key:                  providerSecretKey,
			},
		},
		Status: gcpv1alpha1.ProviderStatus{
			DeprecatedConditionedStatus: corev1alpha1.DeprecatedConditionedStatus{
				Conditions: []corev1alpha1.DeprecatedCondition{{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue}},
			},
		},
	}

	providerSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerSecretName},
		Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	}
)

type instanceModifier func(*v1alpha1.CloudMemorystoreInstance)

func withConditions(c ...corev1alpha1.DeprecatedCondition) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.Status.DeprecatedConditionedStatus.Conditions = c }
}

func withState(s string) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.Status.State = s }
}

func withFinalizers(f ...string) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.ObjectMeta.Finalizers = f }
}

func withReclaimPolicy(p corev1alpha1.ReclaimPolicy) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.Spec.ReclaimPolicy = p }
}

func withInstanceName(n string) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.Status.InstanceName = n }
}

func withProviderID(id string) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.Status.ProviderID = id }
}

func withEndpoint(e string) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.Status.Endpoint = e }
}

func withPort(p int) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.Status.Port = p }
}

func withDeletionTimestamp(t time.Time) instanceModifier {
	return func(i *v1alpha1.CloudMemorystoreInstance) { i.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t} }
}

func instance(im ...instanceModifier) *v1alpha1.CloudMemorystoreInstance {
	i := &v1alpha1.CloudMemorystoreInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       instanceName,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.CloudMemorystoreInstanceSpec{
			MemorySizeGB:        memorySizeGB,
			RedisConfigs:        redisConfigs,
			AuthorizedNetwork:   authorizedNetwork,
			ProviderRef:         corev1.LocalObjectReference{Name: providerName},
			ConnectionSecretRef: corev1.LocalObjectReference{Name: connectionSecretName},
		},
		Status: v1alpha1.CloudMemorystoreInstanceStatus{
			Endpoint:   host,
			Port:       port,
			ProviderID: qualifiedName,
		},
	}

	for _, m := range im {
		m(i)
	}

	return i
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

func TestCreate(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		i           *v1alpha1.CloudMemorystoreInstance
		want        *v1alpha1.CloudMemorystoreInstance
		wantRequeue bool
	}{
		{
			name: "SuccessfulCreate",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockCreateInstance: func(_ context.Context, _ *redisv1pb.CreateInstanceRequest, _ ...gax.CallOption) (*redisv1.CreateInstanceOperation, error) {
					return nil, nil
				}},
			},
			i: instance(),
			want: instance(
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
				withFinalizers(finalizerName),
				withInstanceName(instanceName),
			),
			wantRequeue: true,
		},
		{
			name: "FailedCreate",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockCreateInstance: func(_ context.Context, _ *redisv1pb.CreateInstanceRequest, _ ...gax.CallOption) (*redisv1.CreateInstanceOperation, error) {
					return nil, errorBoom
				},
			}},
			i: instance(),
			want: instance(withConditions(
				corev1alpha1.DeprecatedCondition{
					Type:    corev1alpha1.DeprecatedFailed,
					Status:  corev1.ConditionTrue,
					Reason:  reasonCreatingInstance,
					Message: errorBoom.Error(),
				},
			)),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Create(ctx, tc.i)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.Create(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := cmp.Diff(tc.want, tc.i); diff != "" {
				t.Errorf("i: want != got:\n%s", diff)
			}
		})
	}
}

func TestSync(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		i           *v1alpha1.CloudMemorystoreInstance
		want        *v1alpha1.CloudMemorystoreInstance
		wantRequeue bool
	}{
		{
			name: "SuccessfulSyncWhileInstanceCreating",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{State: redisv1pb.Instance_CREATING}, nil
				},
			}},
			i: instance(
				withInstanceName(instanceName),
				withConditions(
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonCreatingInstance,
						Message: errorBoom.Error(),
					},
				),
			),
			want: instance(
				withState(v1alpha1.StateCreating),
				withInstanceName(instanceName),
				withConditions(
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionFalse,
						Reason:  reasonCreatingInstance,
						Message: errorBoom.Error(),
					},
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue},
				),
			),
			wantRequeue: true,
		},
		{
			name: "SuccessfulSyncWhileInstanceDeleting",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{State: redisv1pb.Instance_DELETING}, nil
				},
			}},
			i: instance(
				withInstanceName(instanceName),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			want: instance(
				withInstanceName(instanceName),
				withState(v1alpha1.StateDeleting),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "SuccessfulSyncWhileInstanceUpdating",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{State: redisv1pb.Instance_UPDATING}, nil
				},
			}},
			i: instance(
				withInstanceName(instanceName),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			want: instance(
				withInstanceName(instanceName),
				withState(v1alpha1.StateUpdating),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionFalse}),
			),
			wantRequeue: true,
		},
		{
			name: "SuccessfulSyncWhileInstanceReadyAndDoesNotNeedUpdate",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						Name:         qualifiedName,
						State:        redisv1pb.Instance_READY,
						MemorySizeGb: memorySizeGB,
						RedisConfigs: redisConfigs,
						Host:         host,
						Port:         port,
						// This field is not in sync between Kubernetes and GCP,
						// but we cannot update it in place, so the instance
						// does not count as needing an update.
						AuthorizedNetwork: "imdifferent",
					}, nil
				},
			}},
			i: instance(
				withInstanceName(instanceName),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue}),
			),
			want: instance(
				withInstanceName(instanceName),
				withState(v1alpha1.StateReady),
				withProviderID(qualifiedName),
				withEndpoint(host),
				withPort(port),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "SuccessfulSyncWhileInstanceReadyAndNeedsUpdate",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						Name:         qualifiedName,
						State:        redisv1pb.Instance_READY,
						MemorySizeGb: memorySizeGB + 1,
						RedisConfigs: redisConfigs,
						Host:         host,
						Port:         port,
					}, nil
				},
				MockUpdateInstance: func(_ context.Context, u *redisv1pb.UpdateInstanceRequest, _ ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error) {
					// The GCP API is reporting more memory than we specified in
					// the Kubernetes API. Ensure we're resetting it to our
					// specified value.
					if u.Instance.MemorySizeGb != memorySizeGB {
						t.Errorf("u.Instance.MemorySizeGB: want %d, got %d", memorySizeGB, u.Instance.MemorySizeGb)
					}
					return nil, nil
				},
			}},
			i: instance(
				withInstanceName(instanceName),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue}),
			),
			want: instance(
				withInstanceName(instanceName),
				withState(v1alpha1.StateReady),
				withProviderID(qualifiedName),
				withEndpoint(host),
				withPort(port),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "FailedGet",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return nil, errorBoom
				},
			}},
			i: instance(
				withInstanceName(instanceName),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue}),
			),
			want: instance(
				withInstanceName(instanceName),
				withConditions(
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedCreating, Status: corev1.ConditionTrue},
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonSyncingInstance,
						Message: errorBoom.Error(),
					},
				),
			),
			wantRequeue: true,
		},
		{
			name: "FailedUpdate",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						Name:         qualifiedName,
						State:        redisv1pb.Instance_READY,
						MemorySizeGb: memorySizeGB + 1,
						RedisConfigs: redisConfigs,
						Host:         host,
						Port:         port,
					}, nil
				},
				MockUpdateInstance: func(_ context.Context, u *redisv1pb.UpdateInstanceRequest, _ ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error) {
					return nil, errorBoom
				},
			}},
			i: instance(withInstanceName(instanceName),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue}),
			),
			want: instance(
				withInstanceName(instanceName),
				withState(v1alpha1.StateReady),
				withProviderID(qualifiedName),
				withEndpoint(host),
				withPort(port),
				withConditions(
					corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue},
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonSyncingInstance,
						Message: errorBoom.Error(),
					},
				),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Sync(ctx, tc.i)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.Sync(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := cmp.Diff(tc.want, tc.i); diff != "" {
				t.Errorf("i: want != got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		i           *v1alpha1.CloudMemorystoreInstance
		want        *v1alpha1.CloudMemorystoreInstance
		wantRequeue bool
	}{
		{
			name: "ReclaimRetainSuccessfulDelete",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockDeleteInstance: func(_ context.Context, _ *redisv1pb.DeleteInstanceRequest, _ ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error) {
					return nil, nil
				}},
			},
			i: instance(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimRetain)),
			want: instance(
				withReclaimPolicy(corev1alpha1.ReclaimRetain),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "ReclaimDeleteSuccessfulDelete",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockDeleteInstance: func(_ context.Context, _ *redisv1pb.DeleteInstanceRequest, _ ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error) {
					return nil, nil
				}},
			},
			i: instance(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimDelete)),
			want: instance(
				withReclaimPolicy(corev1alpha1.ReclaimDelete),
				withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedDeleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "ReclaimDeleteFailedDelete",
			csd: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{
				MockDeleteInstance: func(_ context.Context, _ *redisv1pb.DeleteInstanceRequest, _ ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error) {
					return nil, errorBoom
				}},
			},
			i: instance(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimDelete)),
			want: instance(
				withFinalizers(finalizerName),
				withReclaimPolicy(corev1alpha1.ReclaimDelete),
				withConditions(
					corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonDeletingInstance,
						Message: errorBoom.Error(),
					},
				),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Delete(ctx, tc.i)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.Delete(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := cmp.Diff(tc.want, tc.i); diff != "" {
				t.Errorf("i: want != got:\n%s", diff)
			}
		})
	}
}

func TestConnect(t *testing.T) {
	cases := []struct {
		name    string
		conn    connecter
		i       *v1alpha1.CloudMemorystoreInstance
		want    createsyncdeleter
		wantErr error
	}{
		{
			name: "SuccessfulConnect",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*gcpv1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClient: func(_ context.Context, _ []byte) (cloudmemorystore.Client, error) {
					return &fakecloudmemorystore.MockClient{}, nil
				},
			},
			i:    instance(),
			want: &cloudMemorystore{client: &fakecloudmemorystore.MockClient{}, project: project},
		},
		{
			name: "FailedToGetProvider",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return kerrors.NewNotFound(schema.GroupResource{}, providerName)
				}},
				newClient: func(_ context.Context, _ []byte) (cloudmemorystore.Client, error) {
					return &fakecloudmemorystore.MockClient{}, nil
				},
			},
			i:       instance(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider %s/%s:  \"%s\" not found", namespace, providerName, providerName)),
		},
		{
			name: "FailedToAssetProviderIsValid",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					// This provider does not have condition ready, and thus is
					// deemed invalid.
					*obj.(*gcpv1alpha1.Provider) = gcpv1alpha1.Provider{
						ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerName},
						Spec: gcpv1alpha1.ProviderSpec{
							ProjectID: project,
							Secret: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: providerSecretName},
								Key:                  providerSecretKey,
							},
						},
					}
					return nil
				}},
				newClient: func(_ context.Context, _ []byte) (cloudmemorystore.Client, error) {
					return &fakecloudmemorystore.MockClient{}, nil
				},
			},
			i:       instance(),
			wantErr: errors.Errorf("provider %s/%s is not ready", namespace, providerName),
		},
		{
			name: "FailedToGetProviderSecret",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*gcpv1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return kerrors.NewNotFound(schema.GroupResource{}, providerSecretName)
					}
					return nil
				}},
				newClient: func(_ context.Context, _ []byte) (cloudmemorystore.Client, error) {
					return &fakecloudmemorystore.MockClient{}, nil
				},
			},
			i:       instance(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider secret %s/%s:  \"%s\" not found", namespace, providerSecretName, providerSecretName)),
		},
		{
			name: "FailedToCreateCloudMemorystoreClient",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*gcpv1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClient: func(_ context.Context, _ []byte) (cloudmemorystore.Client, error) { return nil, errorBoom },
			},
			i:       instance(),
			want:    &cloudMemorystore{project: project},
			wantErr: errors.Wrap(errorBoom, "cannot create new CloudMemorystore client"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := tc.conn.Connect(ctx, tc.i)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.conn.Connect(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(cloudMemorystore{})); diff != "" {
				t.Errorf("tc.conn.Connect(...): want != got:\n%s", diff)
			}
		})
	}
}

type mockConnector struct {
	MockConnect func(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error)
}

func (c *mockConnector) Connect(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
	return c.MockConnect(ctx, i)
}

type mockCSD struct {
	MockCreate func(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool
	MockSync   func(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool
	MockDelete func(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool
}

func (csd *mockCSD) Create(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool {
	return csd.MockCreate(ctx, i)
}

func (csd *mockCSD) Sync(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool {
	return csd.MockSync(ctx, i)
}

func (csd *mockCSD) Delete(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool {
	return csd.MockDelete(ctx, i)
}

func TestReconcile(t *testing.T) {
	cases := []struct {
		name    string
		rec     *Reconciler
		req     reconcile.Request
		want    reconcile.Result
		wantErr error
	}{
		{
			name: "SuccessfulDelete",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
					return &mockCSD{MockDelete: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) bool { return false }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.CloudMemorystoreInstance) = *(instance(withInstanceName(instanceName), withDeletionTimestamp(time.Now())))
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "SuccessfulCreate",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
					return &mockCSD{MockCreate: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) bool { return true }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.CloudMemorystoreInstance) = *(instance())
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "SuccessfulSync",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
					return &mockCSD{MockSync: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) bool { return false }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: instanceName}:
							*obj.(*v1alpha1.CloudMemorystoreInstance) = *(instance(withInstanceName(instanceName), withEndpoint(host)))
						case client.ObjectKey{Namespace: namespace, Name: connectionSecretName}:
							return kerrors.NewNotFound(schema.GroupResource{}, connectionSecretName)
						}
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
					MockCreate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "FailedToGetNonexistentInstance",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, instanceName)
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "FailedToGetExtantInstance",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errorBoom
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: false},
			wantErr: errors.Wrapf(errorBoom, "cannot get instance %s/%s", namespace, instanceName),
		},
		{
			name: "FailedToConnect",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
					return nil, errorBoom
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.CloudMemorystoreInstance) = *(instance())
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := instance(withConditions(
							corev1alpha1.DeprecatedCondition{
								Type:    corev1alpha1.DeprecatedFailed,
								Status:  corev1.ConditionTrue,
								Reason:  reasonFetchingClient,
								Message: errorBoom.Error(),
							},
						))
						got := obj.(*v1alpha1.CloudMemorystoreInstance)
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("kube.Update(...): want != got:\n%s", diff)
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "FailedToGetConnectionSecret",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
					return nil, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case types.NamespacedName{Namespace: namespace, Name: connectionSecretName}:
							return errorBoom
						case types.NamespacedName{Namespace: namespace, Name: instanceName}:
							*obj.(*v1alpha1.CloudMemorystoreInstance) = *(instance(withInstanceName(instanceName)))
						}
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := instance(
							withInstanceName(instanceName),
							withConditions(
								corev1alpha1.DeprecatedCondition{
									Type:    corev1alpha1.DeprecatedFailed,
									Status:  corev1.ConditionTrue,
									Reason:  reasonSyncingSecret,
									Message: errors.Wrapf(errorBoom, "cannot get secret %s/%s", namespace, connectionSecretName).Error(),
								},
							))
						got := obj.(*v1alpha1.CloudMemorystoreInstance)
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("kube.Update(...): want != got:\n%s", diff)
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "FailedToCreateConnectionSecret",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
					return nil, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case types.NamespacedName{Namespace: namespace, Name: connectionSecretName}:
							return kerrors.NewNotFound(schema.GroupResource{}, connectionSecretName)
						case types.NamespacedName{Namespace: namespace, Name: instanceName}:
							*obj.(*v1alpha1.CloudMemorystoreInstance) = *(instance(withInstanceName(instanceName)))
						}
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := instance(
							withInstanceName(instanceName),
							withConditions(
								corev1alpha1.DeprecatedCondition{
									Type:    corev1alpha1.DeprecatedFailed,
									Status:  corev1.ConditionTrue,
									Reason:  reasonSyncingSecret,
									Message: errors.Wrapf(errorBoom, "cannot create secret %s/%s", namespace, connectionSecretName).Error(),
								},
							))
						got := obj.(*v1alpha1.CloudMemorystoreInstance)
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("kube.Update(...): want != got:\n%s", diff)
						}
						return nil
					},
					MockCreate: func(_ context.Context, obj runtime.Object) error { return errorBoom },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "FailedToUpdateConnectionSecret",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
					return nil, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case types.NamespacedName{Namespace: namespace, Name: connectionSecretName}:
							return nil
						case types.NamespacedName{Namespace: namespace, Name: instanceName}:
							*obj.(*v1alpha1.CloudMemorystoreInstance) = *(instance(withInstanceName(instanceName)))
						}
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						switch got := obj.(type) {
						case *corev1.Secret:
							return errorBoom
						case *v1alpha1.CloudMemorystoreInstance:
							want := instance(
								withInstanceName(instanceName),
								withConditions(
									corev1alpha1.DeprecatedCondition{
										Type:    corev1alpha1.DeprecatedFailed,
										Status:  corev1.ConditionTrue,
										Reason:  reasonSyncingSecret,
										Message: errors.Wrapf(errorBoom, "cannot update secret %s/%s", namespace, connectionSecretName).Error(),
									},
								))
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("kube.Update(...): want != got:\n%s", diff)
							}
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: instanceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, gotErr := tc.rec.Reconcile(tc.req)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, gotResult); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want != got:\n%s", diff)
			}
		})
	}
}

func TestConnectionSecret(t *testing.T) {
	cases := []struct {
		name string
		i    *v1alpha1.CloudMemorystoreInstance
		want *corev1.Secret
	}{
		{
			name: "Successful",
			i:    instance(withEndpoint(host)),
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      connectionSecretName,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: v1alpha1.APIVersion,
						Kind:       v1alpha1.CloudMemorystoreInstanceKind,
						Name:       instanceName,
						UID:        uid,
					}},
				},
				Data: map[string][]byte{corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(host)},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectionSecret(tc.i)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("connectionSecret(...): want != got:\n%s", diff)
			}
		})
	}
}
