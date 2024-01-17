// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package initializer initializes a new installation of Crossplane.
package initializer

import (
	"context"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// New returns a new *Initializer.
func New(kube client.Client, log logging.Logger, steps ...Step) *Initializer {
	return &Initializer{kube: kube, log: log, steps: steps}
}

// Step is a blocking step of the initialization process.
type Step interface {
	Run(ctx context.Context, kube client.Client) error
}

// StepFunc is a function that implements Step.
type StepFunc func(ctx context.Context, kube client.Client) error

// Run calls the step function.
func (f StepFunc) Run(ctx context.Context, kube client.Client) error {
	return f(ctx, kube)
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
		if s == nil {
			continue
		}
		if err := s.Run(ctx, c.kube); err != nil {
			return err
		}
		t := reflect.TypeOf(s)
		var name string
		if t != nil {
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			name = t.Name()
		} else {
			name = fmt.Sprintf("%T", s)
		}
		c.log.Info("Step has been completed", "Name", name)
	}
	return nil
}
