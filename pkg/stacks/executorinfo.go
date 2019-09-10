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

package stacks

import (
	"context"
	"os"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/util"
)

var (
	// PodImageNameEnvVar is the env variable for setting the image name used for the stack manager unpack/install process.
	// When this env variable is not set the parent Pod will be detected and the associated image will be used.
	// Overriding this variable is only useful when debugging the main stack manager process, since there is no Pod to detect.
	PodImageNameEnvVar = "STACK_MANAGER_IMAGE"
)

// ExecutorInfo stores information about an executing container
type ExecutorInfo struct {
	Image string
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

// Discover the container image from the predefined Stack Manager pod
func (eif *KubeExecutorInfoDiscoverer) Discover(ctx context.Context) (*ExecutorInfo, error) {
	eif.mutex.Lock()
	defer eif.mutex.Unlock()

	if eif.Image != "" {
		return &eif.ExecutorInfo, nil
	}

	image := os.Getenv(PodImageNameEnvVar)

	if image != "" {
		return &ExecutorInfo{Image: image}, nil
	}

	if pod, err := util.GetRunningPod(ctx, eif.Client); err != nil {
		log.Error(err, "failed to get running pod")
		return nil, err
	} else if image, err = util.GetContainerImage(pod, ""); err != nil {
		log.Error(err, "failed to get image for pod", "image", image)
		return nil, err
	}

	return &ExecutorInfo{Image: image}, nil
}
