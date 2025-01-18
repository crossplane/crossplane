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

package job

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	crossapiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

const (
	errListNamespace            = "cannot list namespaces"
	errListCompositionRevs      = "cannot list composition revisions"
	errListLabelCompositionRevs = "cannot list composition revisions for a label"
	errDeleteCompositionRev     = "cannot delete composition revision"
)

type compositionRevisionCleanupJob struct {
	log              logging.Logger
	k8sClientset     kubernetes.Interface
	crossplaneClient client.Client
}

func (c compositionRevisionCleanupJob) Run(ctx context.Context, itemsToKeep map[string]struct{}, keepTopNItems int) (int, error) {
	totalClearedRevsCount := 0

	namespaces, err := c.k8sClientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return totalClearedRevsCount, errors.Wrap(err, errListNamespace)
	}

	for _, ns := range namespaces.Items {
		allRevisions := &crossapiextensionsv1.CompositionRevisionList{}
		err = c.crossplaneClient.List(ctx, allRevisions, client.InNamespace(ns.GetName()))
		if err != nil {
			return totalClearedRevsCount, errors.Wrap(err, errListCompositionRevs)
		}

		elements := make(map[string]struct{})

		for _, rev := range allRevisions.Items {
			elements[rev.ObjectMeta.Labels[crossapiextensionsv1.LabelCompositionName]] = struct{}{}
		}

		for uniqueKind := range elements {
			// skip clearing loop for configured items
			if _, found := itemsToKeep[uniqueKind]; found {
				continue
			}
			clearedRevsCount, err := c.processComposition(ctx, uniqueKind, keepTopNItems)
			totalClearedRevsCount += clearedRevsCount
			if err != nil {
				return totalClearedRevsCount, err
			}
		}
	}

	return totalClearedRevsCount, nil
}

func (c compositionRevisionCleanupJob) processComposition(ctx context.Context, uniqueKind string, keepTopNItems int) (int, error) {
	clearedRevsCount := 0

	kindRevisions := &crossapiextensionsv1.CompositionRevisionList{}
	ml := client.MatchingLabels{}
	ml[crossapiextensionsv1.LabelCompositionName] = uniqueKind

	if err := c.crossplaneClient.List(ctx, kindRevisions, ml); err != nil {
		return clearedRevsCount, errors.Wrap(err, errListLabelCompositionRevs)
	}

	sort.Slice(kindRevisions.Items, func(i, j int) bool {
		// sort in descending mode
		return kindRevisions.Items[i].Spec.Revision > kindRevisions.Items[j].Spec.Revision
	})

	revisionsToKeep := make(map[string]struct{})

	for idx, rev := range kindRevisions.Items {
		// keep top N recent revisions in local cache to keep
		if keepTopNItems > idx {
			revisionsToKeep[rev.GetName()] = struct{}{}
		}
	}
	for _, rev := range kindRevisions.Items {
		// Remove old revisions that aren't in top N recent revisions.
		if _, found := revisionsToKeep[rev.GetName()]; !found {
			if err := c.crossplaneClient.Delete(ctx, &rev); resource.IgnoreNotFound(err) != nil {
				return clearedRevsCount, errors.Wrap(err, errDeleteCompositionRev)
			}
			clearedRevsCount++
		}
	}
	return clearedRevsCount, nil
}

func newCompositionRevisionCleanupJob(log logging.Logger, k8sClientset kubernetes.Interface, crossplaneClient client.Client) Job {
	return compositionRevisionCleanupJob{
		log:              log,
		k8sClientset:     k8sClientset,
		crossplaneClient: crossplaneClient,
	}
}