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

	redismgmt "github.com/Azure/azure-sdk-for-go/services/redis/mgmt/2018-03-01/redis"
	"github.com/go-test/deep"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/cache/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/redis"
	fakeredis "github.com/crossplaneio/crossplane/pkg/clients/azure/redis/fake"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace         = "cool-namespace"
	uid               = types.UID("definitely-a-uuid")
	resourceName      = redis.NamePrefix + "-" + string(uid)
	resourceGroupName = "coolgroup"
	location          = "coolplace"
	subscription      = "totally-a-uuid"
	qualifiedName     = "/subscriptions/" + subscription + "/resourceGroups/" + resourceGroupName + "/providers/Microsoft.Cache/Redis/" + resourceName
	host              = "172.16.0.1"
	port              = 6379
	sslPort           = 6380
	enableNonSSLPort  = true
	shardCount        = 3
	skuName           = v1alpha1.SKUNameBasic
	skuFamily         = v1alpha1.SKUFamilyC
	skuCapacity       = 1

	primaryAccessKey = "sosecret"

	providerName       = "cool-azure"
	providerSecretName = "cool-azure-secret"
	providerSecretKey  = "credentials"
	providerSecretData = "definitelyjson"

	connectionSecretName = "cool-connection-secret"
)

var (
	ctx                = context.Background()
	errorBoom          = errors.New("boom")
	redisConfiguration = map[string]string{"cool": "socool"}

	provider = azurev1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerName},
		Spec: azurev1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: providerSecretName},
				Key:                  providerSecretKey,
			},
		},
		Status: azurev1alpha1.ProviderStatus{
			ConditionedStatus: corev1alpha1.ConditionedStatus{
				Conditions: []corev1alpha1.Condition{{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue}},
			},
		},
	}

	providerSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerSecretName},
		Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	}
)

type resourceModifier func(*v1alpha1.Redis)

func withConditions(c ...corev1alpha1.Condition) resourceModifier {
	return func(r *v1alpha1.Redis) { r.Status.ConditionedStatus.Conditions = c }
}

func withState(s string) resourceModifier {
	return func(r *v1alpha1.Redis) { r.Status.State = s }
}

func withFinalizers(f ...string) resourceModifier {
	return func(r *v1alpha1.Redis) { r.ObjectMeta.Finalizers = f }
}

func withReclaimPolicy(p corev1alpha1.ReclaimPolicy) resourceModifier {
	return func(r *v1alpha1.Redis) { r.Spec.ReclaimPolicy = p }
}

func withResourceName(n string) resourceModifier {
	return func(r *v1alpha1.Redis) { r.Status.ResourceName = n }
}

func withProviderID(id string) resourceModifier {
	return func(r *v1alpha1.Redis) { r.Status.ProviderID = id }
}

func withEndpoint(e string) resourceModifier {
	return func(r *v1alpha1.Redis) { r.Status.Endpoint = e }
}

func withPort(p int) resourceModifier {
	return func(r *v1alpha1.Redis) { r.Status.Port = p }
}

func withSSLPort(p int) resourceModifier {
	return func(r *v1alpha1.Redis) { r.Status.SSLPort = p }
}

func withDeletionTimestamp(t time.Time) resourceModifier {
	return func(r *v1alpha1.Redis) { r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t} }
}

func resource(rm ...resourceModifier) *v1alpha1.Redis {
	r := &v1alpha1.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       resourceName,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.RedisSpec{
			ResourceGroupName:  resourceGroupName,
			Location:           location,
			RedisConfiguration: redisConfiguration,
			EnableNonSSLPort:   enableNonSSLPort,
			ShardCount:         shardCount,
			SKU: v1alpha1.SKUSpec{
				Name:     skuName,
				Family:   skuFamily,
				Capacity: skuCapacity,
			},
			ProviderRef:         corev1.LocalObjectReference{Name: providerName},
			ConnectionSecretRef: corev1.LocalObjectReference{Name: connectionSecretName},
		},
		Status: v1alpha1.RedisStatus{
			Endpoint:   host,
			Port:       port,
			ProviderID: qualifiedName,
		},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

func TestCreate(t *testing.T) {
	cases := []struct {
		name        string
		csdk        createsyncdeletekeyer
		r           *v1alpha1.Redis
		want        *v1alpha1.Redis
		wantRequeue bool
	}{
		{
			name: "SuccessfulCreate",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockCreate: func(_ context.Context, _, _ string, _ redismgmt.CreateParameters) (redismgmt.CreateFuture, error) {
					return redismgmt.CreateFuture{}, nil
				},
			}},
			r: resource(),
			want: resource(
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Creating, Status: corev1.ConditionTrue}),
				withFinalizers(finalizerName),
				withResourceName(resourceName),
			),
			wantRequeue: true,
		},
		{
			name: "FailedCreate",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockCreate: func(_ context.Context, _, _ string, _ redismgmt.CreateParameters) (redismgmt.CreateFuture, error) {
					return redismgmt.CreateFuture{}, errorBoom
				},
			}},
			r: resource(),
			want: resource(withConditions(
				corev1alpha1.Condition{
					Type:    corev1alpha1.Failed,
					Status:  corev1.ConditionTrue,
					Reason:  reasonCreatingResource,
					Message: errorBoom.Error(),
				},
			)),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csdk.Create(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csdk.Create(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := deep.Equal(tc.want, tc.r); diff != nil {
				t.Errorf("r: want != got:\n%s", diff)
			}
		})
	}
}

func TestSync(t *testing.T) {
	cases := []struct {
		name        string
		csdk        createsyncdeletekeyer
		r           *v1alpha1.Redis
		want        *v1alpha1.Redis
		wantRequeue bool
	}{
		{
			name: "SuccessfulSyncWhileResourceCreating",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockGet: func(_ context.Context, _, _ string) (redismgmt.ResourceType, error) {
					return redismgmt.ResourceType{Properties: &redismgmt.Properties{ProvisioningState: redismgmt.Creating}}, nil
				},
			}},
			r: resource(
				withResourceName(resourceName),
				withConditions(
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonCreatingResource,
						Message: errorBoom.Error(),
					},
				),
			),
			want: resource(
				withState(v1alpha1.ProvisioningStateCreating),
				withResourceName(resourceName),
				withConditions(
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionFalse,
						Reason:  reasonCreatingResource,
						Message: errorBoom.Error(),
					},
					corev1alpha1.Condition{Type: corev1alpha1.Creating, Status: corev1.ConditionTrue},
				),
			),
			wantRequeue: true,
		},
		{
			name: "SuccessfulSyncWhileResourceDeleting",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockGet: func(_ context.Context, _, _ string) (redismgmt.ResourceType, error) {
					return redismgmt.ResourceType{Properties: &redismgmt.Properties{ProvisioningState: redismgmt.Deleting}}, nil
				},
			}},
			r: resource(
				withResourceName(resourceName),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Deleting, Status: corev1.ConditionTrue}),
			),
			want: resource(
				withResourceName(resourceName),
				withState(v1alpha1.ProvisioningStateDeleting),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Deleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "SuccessfulSyncWhileResourceUpdating",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockGet: func(_ context.Context, _, _ string) (redismgmt.ResourceType, error) {
					return redismgmt.ResourceType{Properties: &redismgmt.Properties{ProvisioningState: redismgmt.Updating}}, nil
				},
			}},
			r: resource(
				withResourceName(resourceName),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Deleting, Status: corev1.ConditionTrue}),
			),
			want: resource(
				withResourceName(resourceName),
				withState(v1alpha1.ProvisioningStateUpdating),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Deleting, Status: corev1.ConditionFalse}),
			),
			wantRequeue: true,
		},
		{
			name: "SuccessfulSyncWhileResourceReadyAndDoesNotNeedUpdate",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockGet: func(_ context.Context, _, _ string) (redismgmt.ResourceType, error) {
					return redismgmt.ResourceType{
						ID: azure.ToStringPtr(qualifiedName),
						Properties: &redismgmt.Properties{
							ProvisioningState: redismgmt.Succeeded,
							Sku: &redismgmt.Sku{
								Name:     redismgmt.SkuName(skuName),
								Family:   redismgmt.SkuFamily(skuFamily),
								Capacity: azure.ToInt32Ptr(skuCapacity),
							},
							EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
							RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
							ShardCount:         azure.ToInt32Ptr(shardCount),
							HostName:           azure.ToStringPtr(host),
							Port:               azure.ToInt32Ptr(port),
							SslPort:            azure.ToInt32Ptr(sslPort),
						},
					}, nil
				},
			}},
			r: resource(
				withResourceName(resourceName),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue}),
			),
			want: resource(
				withResourceName(resourceName),
				withState(v1alpha1.ProvisioningStateSucceeded),
				withProviderID(qualifiedName),
				withEndpoint(host),
				withPort(port),
				withSSLPort(sslPort),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "SuccessfulSyncWhileResourceReadyAndNeedsUpdate",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockGet: func(_ context.Context, _, _ string) (redismgmt.ResourceType, error) {
					return redismgmt.ResourceType{
						ID: azure.ToStringPtr(qualifiedName),
						Properties: &redismgmt.Properties{
							ProvisioningState: redismgmt.Succeeded,
							Sku: &redismgmt.Sku{
								Name:     redismgmt.SkuName(skuName),
								Family:   redismgmt.SkuFamily(skuFamily),
								Capacity: azure.ToInt32Ptr(skuCapacity),
							},
							EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
							RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
							ShardCount:         azure.ToInt32Ptr(shardCount + 1),
							HostName:           azure.ToStringPtr(host),
							Port:               azure.ToInt32Ptr(port),
							SslPort:            azure.ToInt32Ptr(sslPort),
						},
					}, nil
				},
				MockUpdate: func(_ context.Context, _, _ string, p redismgmt.UpdateParameters) (redismgmt.ResourceType, error) {
					if azure.ToInt(p.ShardCount) != shardCount {
						t.Errorf("p.ShardCount: want %d, got %d", shardCount, azure.ToInt(p.ShardCount))
					}
					return redismgmt.ResourceType{}, nil
				},
			}},
			r: resource(
				withResourceName(resourceName),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue}),
			),
			want: resource(
				withResourceName(resourceName),
				withState(v1alpha1.ProvisioningStateSucceeded),
				withProviderID(qualifiedName),
				withEndpoint(host),
				withPort(port),
				withSSLPort(sslPort),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "FailedGet",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockGet: func(_ context.Context, _, _ string) (redismgmt.ResourceType, error) {
					return redismgmt.ResourceType{}, errorBoom
				},
			}},
			r: resource(
				withResourceName(resourceName),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Creating, Status: corev1.ConditionTrue}),
			),
			want: resource(
				withResourceName(resourceName),
				withConditions(
					corev1alpha1.Condition{Type: corev1alpha1.Creating, Status: corev1.ConditionTrue},
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonSyncingResource,
						Message: errorBoom.Error(),
					},
				),
			),
			wantRequeue: true,
		},
		{
			name: "FailedUpdate",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockGet: func(_ context.Context, _, _ string) (redismgmt.ResourceType, error) {
					return redismgmt.ResourceType{
						ID: azure.ToStringPtr(qualifiedName),
						Properties: &redismgmt.Properties{
							ProvisioningState: redismgmt.Succeeded,
							Sku: &redismgmt.Sku{
								Name:     redismgmt.SkuName(skuName),
								Family:   redismgmt.SkuFamily(skuFamily),
								Capacity: azure.ToInt32Ptr(skuCapacity),
							},
							EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
							RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
							ShardCount:         azure.ToInt32Ptr(shardCount + 1),
							HostName:           azure.ToStringPtr(host),
							Port:               azure.ToInt32Ptr(port),
							SslPort:            azure.ToInt32Ptr(sslPort),
						},
					}, nil
				},
				MockUpdate: func(_ context.Context, _, _ string, _ redismgmt.UpdateParameters) (redismgmt.ResourceType, error) {
					return redismgmt.ResourceType{}, errorBoom
				},
			}},
			r: resource(withResourceName(resourceName),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue}),
			),
			want: resource(
				withResourceName(resourceName),
				withState(v1alpha1.ProvisioningStateSucceeded),
				withProviderID(qualifiedName),
				withEndpoint(host),
				withPort(port),
				withSSLPort(sslPort),
				withConditions(
					corev1alpha1.Condition{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue},
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonSyncingResource,
						Message: errorBoom.Error(),
					},
				),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csdk.Sync(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csdk.Sync(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := deep.Equal(tc.want, tc.r); diff != nil {
				t.Errorf("r: want != got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		name        string
		csdk        createsyncdeletekeyer
		r           *v1alpha1.Redis
		want        *v1alpha1.Redis
		wantRequeue bool
	}{
		{
			name: "ReclaimRetainSuccessfulDelete",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockDelete: func(_ context.Context, _, _ string) (redismgmt.DeleteFuture, error) {
					return redismgmt.DeleteFuture{}, nil
				},
			}},
			r: resource(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimRetain)),
			want: resource(
				withReclaimPolicy(corev1alpha1.ReclaimRetain),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Deleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "ReclaimDeleteSuccessfulDelete",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockDelete: func(_ context.Context, _, _ string) (redismgmt.DeleteFuture, error) {
					return redismgmt.DeleteFuture{}, nil
				},
			}},
			r: resource(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimDelete)),
			want: resource(
				withReclaimPolicy(corev1alpha1.ReclaimDelete),
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Deleting, Status: corev1.ConditionTrue}),
			),
			wantRequeue: false,
		},
		{
			name: "ReclaimDeleteFailedDelete",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockDelete: func(_ context.Context, _, _ string) (redismgmt.DeleteFuture, error) {
					return redismgmt.DeleteFuture{}, errorBoom
				},
			}},
			r: resource(withFinalizers(finalizerName), withReclaimPolicy(corev1alpha1.ReclaimDelete)),
			want: resource(
				withFinalizers(finalizerName),
				withReclaimPolicy(corev1alpha1.ReclaimDelete),
				withConditions(
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonDeletingResource,
						Message: errorBoom.Error(),
					},
				),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csdk.Delete(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csdk.Delete(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := deep.Equal(tc.want, tc.r); diff != nil {
				t.Errorf("r: want != got:\n%s", diff)
			}
		})
	}
}
func TestKey(t *testing.T) {
	cases := []struct {
		name    string
		csdk    createsyncdeletekeyer
		r       *v1alpha1.Redis
		want    *v1alpha1.Redis
		wantKey string
	}{
		{
			name: "Successful",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockListKeys: func(_ context.Context, _, _ string) (redismgmt.AccessKeys, error) {
					return redismgmt.AccessKeys{PrimaryKey: azure.ToStringPtr(primaryAccessKey)}, nil
				},
			}},
			r:       resource(),
			want:    resource(),
			wantKey: primaryAccessKey,
		},
		{
			name: "Failed",
			csdk: &azureRedisCache{client: &fakeredis.MockClient{
				MockListKeys: func(_ context.Context, _, _ string) (redismgmt.AccessKeys, error) {
					return redismgmt.AccessKeys{}, errorBoom
				},
			}},
			r: resource(),
			want: resource(
				withConditions(
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonGettingKey,
						Message: errorBoom.Error(),
					},
				),
			),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey := tc.csdk.Key(ctx, tc.r)

			if gotKey != tc.wantKey {
				t.Errorf("tc.csdk.Key(...): want: %s got: %s", tc.wantKey, gotKey)
			}

			if diff := deep.Equal(tc.want, tc.r); diff != nil {
				t.Errorf("r: want != got:\n%s", diff)
			}
		})
	}
}

func TestConnect(t *testing.T) {
	cases := []struct {
		name    string
		conn    connecter
		i       *v1alpha1.Redis
		want    createsyncdeletekeyer
		wantErr error
	}{
		{
			name: "SuccessfulConnect",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*azurev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClient: func(_ context.Context, _ []byte) (redis.Client, error) { return &fakeredis.MockClient{}, nil },
			},
			i:    resource(),
			want: &azureRedisCache{client: &fakeredis.MockClient{}},
		},
		{
			name: "FailedToGetProvider",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return kerrors.NewNotFound(schema.GroupResource{}, providerName)
				}},
				newClient: func(_ context.Context, _ []byte) (redis.Client, error) { return &fakeredis.MockClient{}, nil },
			},
			i:       resource(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider %s/%s:  \"%s\" not found", namespace, providerName, providerName)),
		},
		{
			name: "FailedToAssetProviderIsValid",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					// This provider does not have condition ready, and thus is
					// deemed invalid.
					*obj.(*azurev1alpha1.Provider) = azurev1alpha1.Provider{
						ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerName},
						Spec: azurev1alpha1.ProviderSpec{
							Secret: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: providerSecretName},
								Key:                  providerSecretKey,
							},
						},
					}
					return nil
				}},
				newClient: func(_ context.Context, _ []byte) (redis.Client, error) { return &fakeredis.MockClient{}, nil },
			},
			i:       resource(),
			wantErr: errors.Errorf("provider %s/%s is not ready", namespace, providerName),
		},
		{
			name: "FailedToGetProviderSecret",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*azurev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return kerrors.NewNotFound(schema.GroupResource{}, providerSecretName)
					}
					return nil
				}},
				newClient: func(_ context.Context, _ []byte) (redis.Client, error) { return &fakeredis.MockClient{}, nil },
			},
			i:       resource(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider secret %s/%s:  \"%s\" not found", namespace, providerSecretName, providerSecretName)),
		},
		{
			name: "FailedToCreateAzureCacheClient",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*azurev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClient: func(_ context.Context, _ []byte) (redis.Client, error) { return nil, errorBoom },
			},
			i:       resource(),
			want:    &azureRedisCache{},
			wantErr: errors.Wrap(errorBoom, "cannot create new Azure Cache client"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := tc.conn.Connect(ctx, tc.i)

			if diff := deep.Equal(tc.wantErr, gotErr); diff != nil {
				t.Errorf("tc.conn.Connect(...): want error != got error:\n%s", diff)
			}

			if diff := deep.Equal(tc.want, got); diff != nil {
				t.Errorf("tc.conn.Connect(...): want != got:\n%s", diff)
			}
		})
	}
}

type mockConnector struct {
	MockConnect func(ctx context.Context, i *v1alpha1.Redis) (createsyncdeletekeyer, error)
}

func (c *mockConnector) Connect(ctx context.Context, i *v1alpha1.Redis) (createsyncdeletekeyer, error) {
	return c.MockConnect(ctx, i)
}

type mockCSDK struct {
	MockCreate func(ctx context.Context, i *v1alpha1.Redis) bool
	MockSync   func(ctx context.Context, i *v1alpha1.Redis) bool
	MockDelete func(ctx context.Context, i *v1alpha1.Redis) bool
	MockKey    func(ctx context.Context, i *v1alpha1.Redis) string
}

func (csdk *mockCSDK) Create(ctx context.Context, i *v1alpha1.Redis) bool {
	return csdk.MockCreate(ctx, i)
}

func (csdk *mockCSDK) Sync(ctx context.Context, i *v1alpha1.Redis) bool {
	return csdk.MockSync(ctx, i)
}

func (csdk *mockCSDK) Delete(ctx context.Context, i *v1alpha1.Redis) bool {
	return csdk.MockDelete(ctx, i)
}

func (csdk *mockCSDK) Key(ctx context.Context, i *v1alpha1.Redis) string {
	return csdk.MockKey(ctx, i)
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
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.Redis) (createsyncdeletekeyer, error) {
					return &mockCSDK{MockDelete: func(_ context.Context, _ *v1alpha1.Redis) bool { return false }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.Redis) = *(resource(withResourceName(resourceName), withDeletionTimestamp(time.Now())))
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "SuccessfulCreate",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.Redis) (createsyncdeletekeyer, error) {
					return &mockCSDK{MockCreate: func(_ context.Context, _ *v1alpha1.Redis) bool { return true }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.Redis) = *(resource())
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "SuccessfulSync",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.Redis) (createsyncdeletekeyer, error) {
					return &mockCSDK{
						MockSync: func(_ context.Context, _ *v1alpha1.Redis) bool { return false },
						MockKey:  func(_ context.Context, _ *v1alpha1.Redis) string { return "" },
					}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: resourceName}:
							*obj.(*v1alpha1.Redis) = *(resource(withResourceName(resourceName), withEndpoint(host)))
						case client.ObjectKey{Namespace: namespace, Name: connectionSecretName}:
							return kerrors.NewNotFound(schema.GroupResource{}, connectionSecretName)
						}
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
					MockCreate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "FailedToGetNonexistentResource",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, resourceName)
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "FailedToGetExtantResource",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errorBoom
					},
					MockUpdate: func(_ context.Context, _ runtime.Object) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: false},
			wantErr: errors.Wrapf(errorBoom, "cannot get resource %s/%s", namespace, resourceName),
		},
		{
			name: "FailedToConnect",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.Redis) (createsyncdeletekeyer, error) {
					return nil, errorBoom
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.Redis) = *(resource())
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := resource(withConditions(
							corev1alpha1.Condition{
								Type:    corev1alpha1.Failed,
								Status:  corev1.ConditionTrue,
								Reason:  reasonFetchingClient,
								Message: errorBoom.Error(),
							},
						))
						got := obj.(*v1alpha1.Redis)
						if diff := deep.Equal(want, got); diff != nil {
							t.Errorf("kube.Update(...): want != got:\n%s", diff)
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "FailedToGetConnectionSecret",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.Redis) (createsyncdeletekeyer, error) {
					return &mockCSDK{MockKey: func(_ context.Context, _ *v1alpha1.Redis) string { return "" }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case types.NamespacedName{Namespace: namespace, Name: connectionSecretName}:
							return errorBoom
						case types.NamespacedName{Namespace: namespace, Name: resourceName}:
							*obj.(*v1alpha1.Redis) = *(resource(withResourceName(resourceName)))
						}
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := resource(
							withResourceName(resourceName),
							withConditions(
								corev1alpha1.Condition{
									Type:    corev1alpha1.Failed,
									Status:  corev1.ConditionTrue,
									Reason:  reasonSyncingSecret,
									Message: errors.Wrapf(errorBoom, "cannot get secret %s/%s", namespace, connectionSecretName).Error(),
								},
							))
						got := obj.(*v1alpha1.Redis)
						if diff := deep.Equal(want, got); diff != nil {
							t.Errorf("kube.Update(...): want != got:\n%s", diff)
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "FailedToCreateConnectionSecret",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.Redis) (createsyncdeletekeyer, error) {
					return &mockCSDK{MockKey: func(_ context.Context, _ *v1alpha1.Redis) string { return "" }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case types.NamespacedName{Namespace: namespace, Name: connectionSecretName}:
							return kerrors.NewNotFound(schema.GroupResource{}, connectionSecretName)
						case types.NamespacedName{Namespace: namespace, Name: resourceName}:
							*obj.(*v1alpha1.Redis) = *(resource(withResourceName(resourceName)))
						}
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						want := resource(
							withResourceName(resourceName),
							withConditions(
								corev1alpha1.Condition{
									Type:    corev1alpha1.Failed,
									Status:  corev1.ConditionTrue,
									Reason:  reasonSyncingSecret,
									Message: errors.Wrapf(errorBoom, "cannot create secret %s/%s", namespace, connectionSecretName).Error(),
								},
							))
						got := obj.(*v1alpha1.Redis)
						if diff := deep.Equal(want, got); diff != nil {
							t.Errorf("kube.Update(...): want != got:\n%s", diff)
						}
						return nil
					},
					MockCreate: func(_ context.Context, obj runtime.Object) error { return errorBoom },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "FailedToUpdateConnectionSecret",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *v1alpha1.Redis) (createsyncdeletekeyer, error) {
					return &mockCSDK{MockKey: func(_ context.Context, _ *v1alpha1.Redis) string { return "" }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case types.NamespacedName{Namespace: namespace, Name: connectionSecretName}:
							return nil
						case types.NamespacedName{Namespace: namespace, Name: resourceName}:
							*obj.(*v1alpha1.Redis) = *(resource(withResourceName(resourceName)))
						}
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object) error {
						switch got := obj.(type) {
						case *corev1.Secret:
							return errorBoom
						case *v1alpha1.Redis:
							want := resource(
								withResourceName(resourceName),
								withConditions(
									corev1alpha1.Condition{
										Type:    corev1alpha1.Failed,
										Status:  corev1.ConditionTrue,
										Reason:  reasonSyncingSecret,
										Message: errors.Wrapf(errorBoom, "cannot update secret %s/%s", namespace, connectionSecretName).Error(),
									},
								))
							if diff := deep.Equal(want, got); diff != nil {
								t.Errorf("kube.Update(...): want != got:\n%s", diff)
							}
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: resourceName}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, gotErr := tc.rec.Reconcile(tc.req)

			if diff := deep.Equal(tc.wantErr, gotErr); diff != nil {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}

			if diff := deep.Equal(tc.want, gotResult); diff != nil {
				t.Errorf("tc.rec.Reconcile(...): want != got:\n%s", diff)
			}
		})
	}
}

func TestConnectionSecret(t *testing.T) {
	cases := []struct {
		name     string
		r        *v1alpha1.Redis
		password string
		want     *corev1.Secret
	}{
		{
			name:     "Successful",
			r:        resource(withEndpoint(host)),
			password: primaryAccessKey,
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      connectionSecretName,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: v1alpha1.APIVersion,
						Kind:       v1alpha1.RedisKind,
						Name:       resourceName,
						UID:        uid,
					}},
				},
				Data: map[string][]byte{
					corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(host),
					corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(primaryAccessKey),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectionSecret(tc.r, tc.password)
			if diff := deep.Equal(tc.want, got); diff != nil {
				t.Errorf("connectionSecret(...): want != got:\n%s", diff)
			}
		})
	}
}
