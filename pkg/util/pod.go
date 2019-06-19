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

package util

import (
	"context"
	"os"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/logging"
)

const (
	// PodNameEnvVar is the env variable for getting the pod name via downward api
	PodNameEnvVar = "POD_NAME"
	// PodNamespaceEnvVar is the env variable for getting the pod namespace via downward api
	PodNamespaceEnvVar = "POD_NAMESPACE"
)

var (
	log = logging.Logger.WithName("util")
)

// GetRunningPod will get the pod object for the currently running pod.  This assumes that the
// downward API has been used to inject the pod name and namespace as env vars.
func GetRunningPod(ctx context.Context, kube client.Client) (*v1.Pod, error) {
	podName := os.Getenv(PodNameEnvVar)
	if podName == "" {
		return nil, errors.New("cannot detect the pod name. Please provide it using the downward API in the manifest file")
	}
	podNamespace := os.Getenv(PodNamespaceEnvVar)
	if podNamespace == "" {
		return nil, errors.New("cannot detect the pod namespace. Please provide it using the downward API in the manifest file")
	}

	name := types.NamespacedName{Name: podName, Namespace: podNamespace}
	log.V(logging.Debug).Info("getting pod", "name", name)

	pod := &v1.Pod{}
	err := kube.Get(ctx, name, pod)

	return pod, err
}

// GetContainerImage will get the container image for the container with the given name in the
// given pod.
func GetContainerImage(pod *v1.Pod, name string) (string, error) {
	return GetSpecContainerImage(pod.Spec, name, false)
}

// GetSpecContainerImage will get the container image for the container with the given name in the
// given pod spec.
func GetSpecContainerImage(spec v1.PodSpec, name string, initContainer bool) (string, error) {
	containers := spec.Containers
	if initContainer {
		containers = spec.InitContainers
	}
	image, err := GetMatchingContainer(containers, name)
	if err != nil {
		return "", err
	}
	return image.Image, nil
}

// GetMatchingContainer returns the container from the given set of containers that matches the
// given name.  If the given container list has only 1 item then the name field is ignored and
// that container is returned.
func GetMatchingContainer(containers []v1.Container, name string) (v1.Container, error) {
	var result *v1.Container
	if len(containers) == 1 {
		// if there is only one pod, use its image rather than require a set container name
		result = &containers[0]
	} else {
		// if there are multiple pods, we require the container to have the expected name
		for _, container := range containers {
			container := container // pin range scoped container var
			if container.Name == name {
				result = &container
				break
			}
		}
	}

	if result == nil {
		return v1.Container{}, errors.Errorf("failed to find image for container %s", name)
	}

	return *result, nil
}
