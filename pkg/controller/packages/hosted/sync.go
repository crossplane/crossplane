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

package hosted

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane/pkg/packages"
	"github.com/crossplane/crossplane/pkg/packages/truncate"
)

const (
	labelForKind     = "host.packages.crossplane.io/for-kind"
	labelForAPIGroup = "host.packages.crossplane.io/for-apiGroup"
	labelForName     = "host.packages.crossplane.io/for-name"

	labelSourceKind      = "host.packages.crossplane.io/source-kind"
	labelSourceAPIGroup  = "host.packages.crossplane.io/source-apiGroup"
	labelSourceName      = "host.packages.crossplane.io/source-name"
	labelSourceNamespace = "host.packages.crossplane.io/source-namespace"

	errSecretNotFoundWithPrefixFmt = "failed to find ImagePullSecret with prefix %q in host resource"
)

// SyncImagePullSecrets copies imagePullSecrets from the tenant to the host
// using the supplied secret names and returns the name of the secrets on the
// host.
//
// The secrets are searched on the host using a name prefix based on the tenant
// secret name. If the secrets are not present they are created with a host
// owner resource reference for garbage collection.
func SyncImagePullSecrets(ctx context.Context, tenantKube, hostKube client.Client, tenantNS string, tenantSecretRefs []corev1.LocalObjectReference, hostSecretRefs []corev1.LocalObjectReference, hostObj packages.KindlyIdentifier) error {
	gvk := hostObj.GroupVersionKind()
	v, k := gvk.ToAPIVersionAndKind()
	name := hostObj.GetName()
	hostNS := hostObj.GetNamespace()

	ref := &corev1.ObjectReference{
		APIVersion: v,
		Kind:       k,
		Namespace:  hostNS,
		Name:       name,
		UID:        hostObj.GetUID(),
	}

	forLabels := map[string]string{
		labelForKind:     k,
		labelForAPIGroup: gvk.Group,
		labelForName:     truncate.LabelValue(name),
	}
	hostSecrets := &corev1.SecretList{}

	if err := hostKube.List(ctx, hostSecrets, client.MatchingLabels(forLabels), client.InNamespace(hostNS)); err != nil {
		return err
	}

	if len(tenantSecretRefs) == 0 && len(hostSecrets.Items) > 0 {
		if err := hostKube.DeleteAllOf(ctx, hostSecrets, client.MatchingLabels(forLabels)); err != nil {
			return err
		}
	}

TENANT:
	for _, secName := range tenantSecretRefs {
		sourceLabels := map[string]string{
			labelSourceKind:      "Secret",
			labelSourceAPIGroup:  "",
			labelSourceName:      truncate.LabelValue(secName.Name),
			labelSourceNamespace: tenantNS,
		}
		hostSecPrefix := ImagePullSecretPrefixOnHost(tenantNS, secName.Name)

		for _, hostSec := range hostSecrets.Items {
			if strings.HasPrefix(hostSec.GetName(), hostSecPrefix) {
				// Found on host. Verify the next tenantSecret.
				continue TENANT
			}
		}

		hostSecName, err := findImagePullSecretName(hostSecPrefix, hostSecretRefs)
		if err != nil {
			return err
		}

		// Not Found. Fetch the secret from the tenant and create the
		// secret on the host.
		_, err = syncImagePullSecret(ctx,
			tenantKube,
			hostKube,
			types.NamespacedName{Namespace: tenantNS, Name: secName.Name},
			types.NamespacedName{Namespace: hostNS, Name: hostSecName},
			labels.Merge(forLabels, sourceLabels),
			ObjectReferenceAnnotationsOnHost("secret", secName.Name, tenantNS),
			meta.AsOwner(ref))
		if err != nil {
			return err
		}
	}

	return nil
}

// findImagePullSecretName returns the first secret name from a list of secret
// references to start with the given prefix
func findImagePullSecretName(hostSecPrefix string, hostSecretRefs []corev1.LocalObjectReference) (string, error) {
	var hostSecName string
	for i := range hostSecretRefs {
		if strings.HasPrefix(hostSecretRefs[i].Name, hostSecPrefix) {
			hostSecName = hostSecretRefs[i].Name
			break
		}
	}

	if hostSecName == "" {
		return "", fmt.Errorf(errSecretNotFoundWithPrefixFmt, hostSecPrefix)
	}
	return hostSecName, nil
}

// syncImagePullSecret reads the named tenant secrets from the tenant API and
// creates a copy on the host with the supplied name, annotations, and labels
func syncImagePullSecret(ctx context.Context, tenantKube, hostKube client.Client, tenantName, hostName types.NamespacedName, labels, annotations map[string]string, owner metav1.OwnerReference) (*corev1.Secret, error) {
	sec := &corev1.Secret{}

	if err := tenantKube.Get(ctx, types.NamespacedName{Name: tenantName.Name, Namespace: tenantName.Namespace}, sec); err != nil {
		return nil, err
	}
	hostSec := &corev1.Secret{}
	hostSec.Data = sec.Data
	hostSec.Type = sec.Type

	hostSec.SetName(hostName.Name)
	hostSec.SetNamespace(hostName.Namespace)
	hostSec.SetLabels(labels)
	hostSec.SetAnnotations(annotations)
	hostSec.SetOwnerReferences([]metav1.OwnerReference{owner})
	if err := hostKube.Create(ctx, hostSec); err != nil {
		return nil, err
	}
	return hostSec, nil
}

// uuidName produces a string suitable as a resource name using a random UUIDv4
func uuidName() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
