/*
Copyright 2021 The Crossplane Authors.

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

package initializer

import (
	"context"

	"github.com/crossplane/crossplane/apis"

	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// NewInitializer returns a new *Initializer.
func NewInitializer(steps ...Step) *Initializer {
	return &Initializer{Steps: steps}
}

// TODO(muvaf): We will have some options to inject CA Bundles to those CRDs
// before applying them.

// Step is a blocking step of the initialization process.
type Step interface {
	Run(ctx context.Context, kube resource.ClientApplicator) error
}

// Initializer makes sure the CRDs Crossplane reconciles are ready to go before
// starting main Crossplane routines.
type Initializer struct {
	Steps []Step
}

// Init does all operations necessary for controllers and webhooks to work.
func (c *Initializer) Init(ctx context.Context) error {
	s := runtime.NewScheme()
	for _, f := range []func(scheme *runtime.Scheme) error{
		extv1.AddToScheme,
		apis.AddToScheme,
	} {
		if err := f(s); err != nil {
			return err
		}
	}
	cl, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: s})
	if err != nil {
		return errors.Wrap(err, "cannot create new kubernetes client")
	}
	kube := resource.ClientApplicator{
		Client:     cl,
		Applicator: resource.NewAPIPatchingApplicator(cl),
	}
	for _, s := range c.Steps {
		if err := s.Run(ctx, kube); err != nil {
			return err
		}
	}

	return nil
}
