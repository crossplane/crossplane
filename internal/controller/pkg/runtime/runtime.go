/*
Copyright 2023 The Crossplane Authors.

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

package runtime

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
)

const (
	// ContainerName is the name of the package runtime container.
	ContainerName = "package-runtime"
	// Providers are expected to use port 8080 if they expose Prometheus
	// metrics, which any provider built using controller-runtime will do by
	// default.

	// FieldOwnerRuntime for SSA (Server Side Apply).
	FieldOwnerRuntime = "pkg.crossplane.io/runtime"

	// MetricsPortName is the name of the metrics port.
	MetricsPortName = "metrics"
	// MetricsPortNumber is the port number for metrics.
	MetricsPortNumber = 8080

	// WebhookTLSCertDirEnvVar is the environment variable for webhook TLS certificate directory.
	WebhookTLSCertDirEnvVar = "WEBHOOK_TLS_CERT_DIR"
	// WebhookPortName is the name of the webhook port.
	WebhookPortName = "webhook"

	// See https://github.com/grpc/grpc/blob/v1.58.0/doc/naming.md

	// GRPCPortName is the name of the gRPC port.
	GRPCPortName = "grpc"
	// GRPCPort is the port number for gRPC.
	GRPCPort = 9443
	// ServiceEndpointFmt is the format string for service endpoints.
	ServiceEndpointFmt = "dns:///%s.%s:%d"

	// TLSServerCertDirEnvVar is the environment variable for TLS server certificate directory.
	TLSServerCertDirEnvVar = "TLS_SERVER_CERTS_DIR"
	// TLSServerCertsVolumeName is the name of the TLS server certificates volume.
	TLSServerCertsVolumeName = "tls-server-certs"
	// TLSServerCertsDir is the directory path for TLS server certificates.
	TLSServerCertsDir = "/tls/server"

	// TLSClientCertDirEnvVar is the environment variable for TLS client certificate directory.
	TLSClientCertDirEnvVar = "TLS_CLIENT_CERTS_DIR"
	// TLSClientCertsVolumeName is the name of the TLS client certificates volume.
	TLSClientCertsVolumeName = "tls-client-certs"
	// TLSClientCertsDir is the directory path for TLS client certificates.
	TLSClientCertsDir = "/tls/client"
)

//nolint:gochecknoglobals // We treat these as constants, but take their addresses.
var (
	// RunAsUser is the user ID to run containers as.
	RunAsUser = int64(2000)
	// RunAsGroup is the group ID to run containers as.
	RunAsGroup = int64(2000)
	// AllowPrivilegeEscalation indicates whether privilege escalation is allowed.
	AllowPrivilegeEscalation = false
	// Privileged indicates whether containers run in privileged mode.
	Privileged = false
	// RunAsNonRoot indicates whether containers must run as non-root user.
	RunAsNonRoot = true
	// AppProtocolTLS is the application protocol for TLS.
	AppProtocolTLS = "tls"
)

// A Hooks manages the runtime objects of a package's revisions. There is
// one implementation per package type, constructed once when the runtime
// controller is set up.
type Hooks interface {
	// Pre performs operations meant to happen before a revision establishes
	// its objects.
	Pre(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error

	// Post performs operations meant to happen after a revision establishes
	// its objects. Once the runtime is available it marks the revision's
	// RuntimeHealthy and RuntimeActive conditions, reporting whether the
	// runtime is scaled up or scaled to zero awaiting activation of its first
	// ManagedResourceDefinition.
	Post(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error

	// Deactivate performs operations meant to happen before deactivating a
	// revision.
	Deactivate(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error
}

const (
	errCopyRuntimeObject      = "cannot copy package runtime object for deletion"
	errGetRuntimeDeployment   = "cannot get package runtime deployment for deletion"
	errGetSharedRuntimeObject = "cannot get existing package runtime object"

	errGetServiceAccount = "cannot get Crossplane service account"
	errGetPullConfig     = "cannot get image pull secret from config"
)

func deleteRuntimeObjectControlledBy(ctx context.Context, c client.Client, owner metav1.Object, obj client.Object) error {
	current, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return errors.Errorf("%s: %T", errCopyRuntimeObject, obj)
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), current); err != nil {
		if resource.IgnoreNotFound(err) == nil {
			return nil
		}

		return errors.Wrap(err, errGetRuntimeDeployment)
	}

	if !metav1.IsControlledBy(current, owner) {
		return nil
	}

	return c.Delete(ctx, current, client.Preconditions{UID: new(current.GetUID())})
}

// applyRuntimeObject applies runtime manifests using SSA.
func applyRuntimeObject(ctx context.Context, c client.Client, obj client.Object) error {
	return c.Patch(
		ctx,
		obj,
		client.Apply, //nolint:staticcheck // Client.Apply requires typed apply configurations.
		client.FieldOwner(FieldOwnerRuntime),
		client.ForceOwnership,
	)
}

// applySharedRuntimeObject applies one of the runtime objects that every revision of a package
// shares (e.g. service and TLS secrets), and takes ownership of it by making the given owner (the
// incoming/active revision) the controller owner.
//
// The handover is done in two steps so that we correctly handle cases where a previous field
// manager owned these fields (e.g. an earlier version of Crossplane that didn't use SSA):
//
// 1) The first declares our own controlling reference alongside whichever controlling reference the
// live object already had, but demoted to a plain owner. This sets the new controller and demotes
// the old one in one write while also giving our field manager exclusive ownership.
// 2) The second apply finds no competing controller left to demote, declares only our own
// reference, and server-side apply prunes the demoted one.
//
// The incoming revision does this so the handover needs no coordination between revisions. The
// revision that wants control just takes it, instead of waiting on another revision to reconcile
// and give it up first.
func applySharedRuntimeObject(ctx context.Context, c client.Client, owner metav1.Object, obj client.Object) error {
	current, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return errors.Errorf("%s: %T", errCopyRuntimeObject, obj)
	}

	// Get the latest state of the object, but clear owner refs before the Get, so we get
	// exactly what the API server has
	current.SetOwnerReferences(nil)
	err := c.Get(ctx, client.ObjectKeyFromObject(obj), current)
	if resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errGetSharedRuntimeObject)
	}

	if err == nil {
		// The object does already exist, demote any controller owner ref from a previous revision
		obj.SetOwnerReferences(append(obj.GetOwnerReferences(), demotedControllers(current, owner)...))
	}

	return applyRuntimeObject(ctx, c, obj)
}

// demotedControllers returns the controlling owner references of obj that don't belong to owner,
// made non-controlling. References that are already non-controlling are left out, which is what
// lets SSA prune them.
func demotedControllers(obj metav1.Object, owner metav1.Object) []metav1.OwnerReference {
	ors := make([]metav1.OwnerReference, 0, len(obj.GetOwnerReferences()))

	for _, or := range obj.GetOwnerReferences() {
		if or.UID == owner.GetUID() || !ptr.Deref(or.Controller, false) {
			continue
		}

		or.Controller = new(false)
		ors = append(ors, or)
	}

	return ors
}

// applySA creates/updates a ServiceAccount as a shared runtime object and includes
// any image pull secrets that have been added by external controllers.
func applySA(ctx context.Context, cl resource.ClientApplicator, owner metav1.Object, sa *corev1.ServiceAccount) error {
	oldSa := &corev1.ServiceAccount{}
	if err := cl.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace}, oldSa); err == nil {
		// Add pull secrets created by other controllers
		existingSecrets := make(map[string]bool)
		for _, secret := range sa.ImagePullSecrets {
			existingSecrets[secret.Name] = true
		}

		for _, secret := range oldSa.ImagePullSecrets {
			if !existingSecrets[secret.Name] {
				sa.ImagePullSecrets = append(sa.ImagePullSecrets, secret)
			}
		}
	}

	return applySharedRuntimeObject(ctx, cl.Client, owner, sa)
}

// corePullSecrets returns the image pull secrets of the core Crossplane
// ServiceAccount. They're appended to the pull secrets of the ServiceAccount we
// build for a package runtime.
func corePullSecrets(ctx context.Context, c client.Client, namespace, name string) ([]corev1.LocalObjectReference, error) {
	sa := &corev1.ServiceAccount{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, sa); err != nil {
		return nil, errors.Wrap(err, errGetServiceAccount)
	}

	return sa.ImagePullSecrets, nil
}

// imageConfigPullSecrets returns the pull secrets of the ImageConfig that was
// applied to the supplied revision, if any. It reads the applied config from
// the revision's status, so the secret doesn't have to be resolved again.
func imageConfigPullSecrets(ctx context.Context, c client.Client, pr v1.PackageRevisionWithRuntime) ([]string, error) {
	for _, icr := range pr.GetAppliedImageConfigRefs() {
		if icr.Reason != v1.ImageConfigReasonSetPullSecret {
			continue
		}

		ic := &v1beta1.ImageConfig{}
		if err := c.Get(ctx, types.NamespacedName{Name: icr.Name}, ic); err != nil {
			return nil, errors.Wrap(err, errGetPullConfig)
		}

		if ic.Spec.Registry.Authentication.PullSecretRef.Name == "" {
			return nil, nil
		}

		return []string{ic.Spec.Registry.Authentication.PullSecretRef.Name}, nil
	}

	return nil, nil
}
