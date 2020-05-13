/*
Copyright 2019 The Crossplane Authors.

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

package packages

import (
	"context"
	"os"
	"sync"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// PodImageNameEnvVar is the env variable for setting the image name used
	// for the package manager unpack/install process. When this env variable is
	// not set the parent Pod will be detected and the associated image will be
	// used. Overriding this variable is only useful when debugging the main
	// package manager process, since there is no Pod to detect. Use of this env
	// variable requires use of its ImagePullPolicy counterpart.
	PodImageNameEnvVar = "PACKAGE_MANAGER_IMAGE"

	// PodImagePullPolicyEnvVar is the env variable for setting the image pull
	// policy used for the package manager unpack/install process. When this env
	// variable is not set the parent Pod will be detected and the associated
	// image pull policy will be used. Overriding this variable is only useful
	// when debugging the main package manager process, since there is no Pod to
	// detect. Use of this env variable requires use of its Image counterpart.
	PodImagePullPolicyEnvVar = "PACKAGE_MANAGER_IMAGEPULLPOLICY"
)

// ExecutorInfo stores information about an executing container
type ExecutorInfo struct {
	Image           string
	ImagePullPolicy corev1.PullPolicy
}

// KubeExecutorInfoDiscoverer discovers container information about an executing Kubernetes pod
type KubeExecutorInfoDiscoverer struct {
	ExecutorInfo
	Client client.Client
	mutex  sync.Mutex
}

// ExecutorInfoDiscoverer implementations can Discover an Image
type ExecutorInfoDiscoverer interface {
	Discover(context.Context) (*ExecutorInfo, error)
}

// Discover the container image from the predefined Package Manager pod.
// ExecutorInfo is not expected to change at runtime, so lookups will be cached.
// Clear the cache by reseting Image before running Discover.
func (eif *KubeExecutorInfoDiscoverer) Discover(ctx context.Context) (*ExecutorInfo, error) {
	eif.mutex.Lock()
	defer eif.mutex.Unlock()

	if eif.Image != "" {
		return &eif.ExecutorInfo, nil
	}

	image := os.Getenv(PodImageNameEnvVar)
	imagePullPolicy := corev1.PullPolicy(os.Getenv(PodImagePullPolicyEnvVar))

	if image != "" {
		return &ExecutorInfo{Image: image, ImagePullPolicy: imagePullPolicy}, nil
	}

	pod, err := GetRunningPod(ctx, eif.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get running pod")
	}
	if image, err = GetContainerImage(pod, "", false); err != nil {
		return nil, errors.Wrap(err, "failed to get image for pod")
	}
	if imagePullPolicy, err = GetContainerImagePullPolicy(pod, "", false); err != nil {
		return nil, errors.Wrap(err, "failed to get imagepullpolicy for pod")
	}

	return &ExecutorInfo{Image: image, ImagePullPolicy: imagePullPolicy}, nil
}
