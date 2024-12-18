/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

// Package job contains implementation of Crossplane jobs.
package job

import (
	"context"


	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Job Defines a runnable task that performs a single scheduled activity.
type Job interface {
	Run(ctx context.Context, itemsToKeep map[string]struct{}, keepTopNItems int) (int, error)
}

const removeUnusedCompositionRevisionJob = "removeUnusedCompositionRevision"

// NewJobs creates a map with all available jobs that can be executed.
func NewJobs(log logging.Logger, k8sClient kubernetes.Interface, crossplaneClient client.Client) map[string]Job {
	return map[string]Job{
		removeUnusedCompositionRevisionJob: newCompositionRevisionCleanupJob(log, k8sClient, crossplaneClient),
	}
}
