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

package manager

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

var (
	runAsUser                = int64(2000)
	runAsGroup               = int64(2000)
	allowPrivilegeEscalation = false
	privileged               = false
	runAsNonRoot             = true
)

// A PodManager manages pods.
type PodManager interface {
	Sync(context.Context, v1alpha1.Package) (string, error)
	GarbageCollect(context.Context, string, v1alpha1.Package) error
}

// PackagePodManager manages pods for a package.
type PackagePodManager struct {
	client    client.Client
	namespace string
}

// NewPackagePodManager creates a new PackagePodManager.
func NewPackagePodManager(client client.Client, namespace string) *PackagePodManager {
	return &PackagePodManager{
		client:    client,
		namespace: namespace,
	}
}

// Sync manages pods for the package. It is meant to be called repeatedly if the
// pod is to be continuously managed.
func (m *PackagePodManager) Sync(ctx context.Context, p v1alpha1.Package) (string, error) {
	// Identify or create pod.
	pod := &corev1.Pod{}
	podName := imageToPod(p.GetSource())
	if err := m.client.Get(ctx, types.NamespacedName{Name: podName, Namespace: m.namespace}, pod); err != nil {
		if kerrors.IsNotFound(err) {
			pod = buildInstallPod(podName, m.namespace, p)
			return "", m.client.Create(ctx, pod)
		}
		return "", err
	}

	// Ensure that Pod is successfully completed.
	if pod.Status.Phase != corev1.PodSucceeded {
		// Check to see if it is in an error state.
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown {
			// If Pod is in error state we should delete it.
			return "", m.client.Delete(ctx, pod)
		}
		// If Pod is not in an error or unknown state, return empty hash.
		return "", nil
	}

	if len(pod.Status.ContainerStatuses) != 1 {
		// If number of container statuses for pod is not 1 and the Pod
		// succeeded then there is a problem. Delete and return empty hash.
		if err := m.client.Delete(ctx, pod); err != nil {
			return "", err
		}
		return "", nil
	}

	imageIDSlice := strings.Split(pod.Status.ContainerStatuses[0].ImageID, ":")
	hash := imageIDSlice[len(imageIDSlice)-1]

	// Do not create a revision if the image ID has not yet been populated.
	if hash == "" {
		// If the Pod is in completed state and has only 1 container status,
		// there should not be an empty image ID. We need to delete the Pod and
		// try again.
		return "", m.client.Delete(ctx, pod)
	}

	return hash, nil
}

// GarbageCollect cleans up old pods.
func (m *PackagePodManager) GarbageCollect(ctx context.Context, source string, p v1alpha1.Package) error {
	pod := buildInstallPod(imageToPod(source), m.namespace, p)
	return m.client.Delete(ctx, pod)
}

// buildInstallPod builds a Pod that parses and outputs a package.
func buildInstallPod(name, namespace string, p v1alpha1.Package) *corev1.Pod {
	pullPolicy := corev1.PullIfNotPresent
	if p.GetPackagePullPolicy() != nil {
		pullPolicy = *p.GetPackagePullPolicy()
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          map[string]string{parentLabel: p.GetName()},
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(p, p.GetObjectKind().GroupVersionKind()))},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &runAsUser,
				RunAsGroup:   &runAsGroup,
			},
			ImagePullSecrets: p.GetPackagePullSecrets(),
			RestartPolicy:    corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "unpack",
					Image:           p.GetSource(),
					ImagePullPolicy: pullPolicy,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:                &runAsUser,
						RunAsGroup:               &runAsGroup,
						AllowPrivilegeEscalation: &allowPrivilegeEscalation,
						Privileged:               &privileged,
						RunAsNonRoot:             &runAsNonRoot,
					},
				},
			},
		},
	}
}
