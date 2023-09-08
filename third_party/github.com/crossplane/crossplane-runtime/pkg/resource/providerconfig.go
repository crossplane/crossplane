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

package resource

import (
	"context"
	"os"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

const (
	errExtractEnv            = "cannot extract from environment variable when none specified"
	errExtractFs             = "cannot extract from filesystem when no path specified"
	errExtractSecretKey      = "cannot extract from secret key when none specified"
	errGetCredentialsSecret  = "cannot get credentials secret"
	errNoHandlerForSourceFmt = "no extraction handler registered for source: %s"
	errMissingPCRef          = "managed resource does not reference a ProviderConfig"
	errApplyPCU              = "cannot apply ProviderConfigUsage"
)

type errMissingRef struct{ error }

func (m errMissingRef) MissingReference() bool { return true }

// IsMissingReference returns true if an error indicates that a managed
// resource is missing a required reference..
func IsMissingReference(err error) bool {
	_, ok := err.(interface { //nolint: errorlint // Skip errorlint for interface type
		MissingReference() bool
	})
	return ok
}

// EnvLookupFn looks up an environment variable.
type EnvLookupFn func(string) string

// ExtractEnv extracts credentials from an environment variable.
func ExtractEnv(_ context.Context, e EnvLookupFn, s xpv1.CommonCredentialSelectors) ([]byte, error) {
	if s.Env == nil {
		return nil, errors.New(errExtractEnv)
	}
	return []byte(e(s.Env.Name)), nil
}

// ExtractFs extracts credentials from the filesystem.
func ExtractFs(_ context.Context, fs afero.Fs, s xpv1.CommonCredentialSelectors) ([]byte, error) {
	if s.Fs == nil {
		return nil, errors.New(errExtractFs)
	}
	return afero.ReadFile(fs, s.Fs.Path)
}

// ExtractSecret extracts credentials from a Kubernetes secret.
func ExtractSecret(ctx context.Context, client client.Client, s xpv1.CommonCredentialSelectors) ([]byte, error) {
	if s.SecretRef == nil {
		return nil, errors.New(errExtractSecretKey)
	}
	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: s.SecretRef.Namespace, Name: s.SecretRef.Name}, secret); err != nil {
		return nil, errors.Wrap(err, errGetCredentialsSecret)
	}
	return secret.Data[s.SecretRef.Key], nil
}

// CommonCredentialExtractor extracts credentials from common sources.
func CommonCredentialExtractor(ctx context.Context, source xpv1.CredentialsSource, client client.Client, selector xpv1.CommonCredentialSelectors) ([]byte, error) {
	switch source {
	case xpv1.CredentialsSourceEnvironment:
		return ExtractEnv(ctx, os.Getenv, selector)
	case xpv1.CredentialsSourceFilesystem:
		return ExtractFs(ctx, afero.NewOsFs(), selector)
	case xpv1.CredentialsSourceSecret:
		return ExtractSecret(ctx, client, selector)
	case xpv1.CredentialsSourceNone:
		return nil, nil
	case xpv1.CredentialsSourceInjectedIdentity:
		// There is no common injected identity extractor. Each provider must
		// implement their own.
		fallthrough
	default:
		return nil, errors.Errorf(errNoHandlerForSourceFmt, source)
	}
}

// A Tracker tracks managed resources.
type Tracker interface {
	// Track the supplied managed resource.
	Track(ctx context.Context, mg Managed) error
}

// A TrackerFn is a function that tracks managed resources.
type TrackerFn func(ctx context.Context, mg Managed) error

// Track the supplied managed resource.
func (fn TrackerFn) Track(ctx context.Context, mg Managed) error {
	return fn(ctx, mg)
}

// A ProviderConfigUsageTracker tracks usages of a ProviderConfig by creating or
// updating the appropriate ProviderConfigUsage.
type ProviderConfigUsageTracker struct {
	c  Applicator
	of ProviderConfigUsage
}

// NewProviderConfigUsageTracker creates a ProviderConfigUsageTracker.
func NewProviderConfigUsageTracker(c client.Client, of ProviderConfigUsage) *ProviderConfigUsageTracker {
	return &ProviderConfigUsageTracker{c: NewAPIUpdatingApplicator(c), of: of}
}

// Track that the supplied Managed resource is using the ProviderConfig it
// references by creating or updating a ProviderConfigUsage. Track should be
// called _before_ attempting to use the ProviderConfig. This ensures the
// managed resource's usage is updated if the managed resource is updated to
// reference a misconfigured ProviderConfig.
func (u *ProviderConfigUsageTracker) Track(ctx context.Context, mg Managed) error {
	pcu := u.of.DeepCopyObject().(ProviderConfigUsage)
	gvk := mg.GetObjectKind().GroupVersionKind()
	ref := mg.GetProviderConfigReference()
	if ref == nil {
		return errMissingRef{errors.New(errMissingPCRef)}
	}

	pcu.SetName(string(mg.GetUID()))
	pcu.SetLabels(map[string]string{xpv1.LabelKeyProviderName: ref.Name})
	pcu.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(mg, gvk))})
	pcu.SetProviderConfigReference(xpv1.Reference{Name: ref.Name})
	pcu.SetResourceReference(xpv1.TypedReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       mg.GetName(),
	})

	err := u.c.Apply(ctx, pcu,
		MustBeControllableBy(mg.GetUID()),
		AllowUpdateIf(func(current, _ runtime.Object) bool {
			return current.(ProviderConfigUsage).GetProviderConfigReference() != pcu.GetProviderConfigReference()
		}),
	)
	return errors.Wrap(Ignore(IsNotAllowed, err), errApplyPCU)
}
