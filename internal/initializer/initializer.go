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
	"reflect"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New returns a new *Initializer.
func New(kube client.Client, log logging.Logger, steps ...Step) *Initializer {
	return &Initializer{kube: kube, log: log, steps: steps}
}

// Step is a blocking step of the initialization process.
type Step interface {
	Run(ctx context.Context, kube client.Client) error
}

// Initializer makes sure the CRDs Crossplane reconciles are ready to go before
// starting main Crossplane routines.
type Initializer struct {
	steps []Step
	kube  client.Client
	log   logging.Logger
}

// Init does all operations necessary for controllers and webhooks to work.
func (c *Initializer) Init(ctx context.Context) error {
	for _, s := range c.steps {
		if err := s.Run(ctx, c.kube); err != nil {
			return err
		}
		c.log.Info("Step has been completed", "Name", reflect.TypeOf(s).Elem().Name())
	}
	return nil
}
