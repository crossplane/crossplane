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

package resource

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// A ConfiguratorChain chains multiple configurators.
type ConfiguratorChain []ManagedConfigurator

// Configure calls each ManagedConfigurator serially. It returns the first
// error it encounters, if any.
func (cc ConfiguratorChain) Configure(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error {
	for _, c := range cc {
		if err := c.Configure(ctx, cm, cs, mg); err != nil {
			return err
		}
	}
	return nil
}

// ConfigureObjectMeta sets standard object metadata (i.e. the name and
// namespace) for a dynamically provisioned resource, deriving it from the
// resource claim.
func ConfigureObjectMeta(_ context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error {
	mg.SetNamespace(cm.GetNamespace())
	mg.SetName(fmt.Sprintf("%s-%s", kindish(cm), cm.GetUID()))

	return nil
}

// kindish tries to return the name of the Claim interface's underlying type,
// e.g. rediscluster, or mysqlinstance. Fall back to simply "claim".
func kindish(obj runtime.Object) string {
	if reflect.ValueOf(obj).Type().Kind() != reflect.Ptr {
		return "claim"
	}
	return strings.ToLower(reflect.TypeOf(obj).Elem().Name())
}
