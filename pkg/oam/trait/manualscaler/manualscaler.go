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

package manualscaler

import (
	"context"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/resource"

	oamv1alpha2 "github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

const (
	errNotDeployment        = "object to be modified is not a deployment"
	errNotManualScalerTrait = "trait is not a manual scaler"
)

// DeploymentModifier modifies the replica count of a Deployment.
func DeploymentModifier(ctx context.Context, obj runtime.Object, t resource.Trait) error {
	d, ok := obj.(*appsv1.Deployment)
	if !ok {
		return errors.New(errNotDeployment)
	}

	ms, ok := t.(*oamv1alpha2.ManualScalerTrait)
	if !ok {
		return errors.New(errNotManualScalerTrait)
	}
	d.Spec.Replicas = &ms.Spec.ReplicaCount

	return nil
}
