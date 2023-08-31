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

// Package controller contains options specific to pkg controllers.
package controller

import (
	"github.com/crossplane/crossplane-runtime/pkg/controller"

	"github.com/crossplane/crossplane/internal/xpkg"
)

// Options specific to pkg controllers.
type Options struct {
	controller.Options

	// Cache for package OCI images.
	Cache xpkg.PackageCache

	// Namespace used to unpack and run packages.
	Namespace string

	// ServiceAccount is the core Crossplane ServiceAccount name.
	ServiceAccount string

	// DefaultRegistry used to pull packages.
	DefaultRegistry string

	// FetcherOptions can be used to add optional parameters to
	// NewK8sFetcher.
	FetcherOptions []xpkg.FetcherOpt

	// WebhookTLSSecretName is the Secret that will be mounted to provider Pods
	// so that they can use it to serve webhooks and also the CA bundle will be
	// injected to CRDs so that API server can make calls to the providers.
	WebhookTLSSecretName string
}
