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

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	redismgmt "github.com/Azure/azure-sdk-for-go/services/redis/mgmt/2018-03-01/redis"
	"github.com/Azure/azure-sdk-for-go/services/redis/mgmt/2018-03-01/redis/redisapi"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

// NamePrefix is the prefix for all created Azure Cache instances.
const NamePrefix = "acr"

// A Client handles CRUD operations for Azure Cache resources. This interface is
// compatible with the upstream Azure redis client.
type Client redisapi.ClientAPI

// NewClient returns a new Azure Cache for Redis client. Credentials must be
// passed as JSON encoded data.
func NewClient(ctx context.Context, credentials []byte) (Client, error) {
	c := azure.Credentials{}
	if err := json.Unmarshal(credentials, &c); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal Azure client secret data")
	}
	client := redismgmt.NewClient(c.SubscriptionID)

	cfg := auth.ClientCredentialsConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		TenantID:     c.TenantID,
		AADEndpoint:  c.ActiveDirectoryEndpointURL,
		Resource:     c.ResourceManagerEndpointURL,
	}
	a, err := cfg.Authorizer()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create Azure authorizer from credentials config")
	}
	client.Authorizer = a
	if err := client.AddToUserAgent(azure.UserAgent); err != nil {
		return nil, errors.Wrap(err, "cannot add to Azure client user agent")
	}

	return client, nil
}

// NewResourceName returns a resource name used to identify a Redis resource in
// the Azure API.
func NewResourceName(o metav1.Object) string {
	return fmt.Sprintf("%s-%s", NamePrefix, o.GetUID())
}

// NewCreateParameters returns Redis resource creation parameters suitable for
// use with the Azure API.
func NewCreateParameters(r *v1alpha1.Redis) redismgmt.CreateParameters {
	return redismgmt.CreateParameters{
		Location: azure.ToStringPtr(r.Spec.Location),
		CreateProperties: &redismgmt.CreateProperties{
			Sku:                NewSKU(r.Spec.SKU),
			SubnetID:           azure.ToStringPtr(r.Spec.SubnetID),
			StaticIP:           azure.ToStringPtr(r.Spec.StaticIP),
			EnableNonSslPort:   azure.ToBoolPtr(r.Spec.EnableNonSSLPort),
			RedisConfiguration: azure.ToStringPtrMap(r.Spec.RedisConfiguration),
			ShardCount:         azure.ToInt32Ptr(r.Spec.ShardCount),
		},
	}
}

// NewUpdateParameters returns Redis resource update parameters suitable for use
// with the Azure API.
func NewUpdateParameters(r *v1alpha1.Redis) redismgmt.UpdateParameters {
	return redismgmt.UpdateParameters{
		UpdateProperties: &redismgmt.UpdateProperties{
			Sku:                NewSKU(r.Spec.SKU),
			RedisConfiguration: azure.ToStringPtrMap(r.Spec.RedisConfiguration),
			EnableNonSslPort:   azure.ToBoolPtr(r.Spec.EnableNonSSLPort),
			ShardCount:         azure.ToInt32Ptr(r.Spec.ShardCount),
		},
	}
}

// NewSKU returns a Redis resource SKU suitable for use with the Azure API.
func NewSKU(s v1alpha1.SKUSpec) *redismgmt.Sku {
	return &redismgmt.Sku{
		Name:     redismgmt.SkuName(s.Name),
		Family:   redismgmt.SkuFamily(s.Family),
		Capacity: azure.ToInt32Ptr(s.Capacity, azure.FieldRequired),
	}
}

// NeedsUpdate returns true if the supplied Kubernetes resource differs from the
// supplied Azure resource. It considers only fields that can be modified in
// place without deleting and recreating the instance.
func NeedsUpdate(kube *v1alpha1.Redis, az redismgmt.ResourceType) bool {
	up := NewUpdateParameters(kube)

	switch {
	case !reflect.DeepEqual(up.Sku, az.Sku):
		return true
	case !reflect.DeepEqual(up.RedisConfiguration, az.RedisConfiguration):
		return true
	case !reflect.DeepEqual(up.EnableNonSslPort, az.EnableNonSslPort):
		return true
	case !reflect.DeepEqual(up.ShardCount, az.ShardCount):
		return true
	}

	return false
}
