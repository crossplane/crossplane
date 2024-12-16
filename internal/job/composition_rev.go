package job

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	crossapiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
)

type CompositionRevisionCleanupJob struct {
	log              logging.Logger
	k8sClientset     *kubernetes.Clientset
	crossplaneClient client.Client
}

func (c CompositionRevisionCleanupJob) Run(ctx context.Context, itemsToKeep map[string]struct{}, keepTopNItems int) (int, error) {
	clearedRevsCount := 0

	namespaces, err := c.k8sClientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return clearedRevsCount, errors.Wrap(err, "cannot list namespaces")
	}

	for _, ns := range namespaces.Items {
		allRevisions := &crossapiextensionsv1.CompositionRevisionList{}
		err = c.crossplaneClient.List(ctx, allRevisions, client.InNamespace(ns.GetName()))
		if err != nil {
			return clearedRevsCount, errors.Wrap(err, "cannot list composition revisions")
		}

		var elements = make(map[string]struct{})

		for _, rev := range allRevisions.Items {
			elements[rev.ObjectMeta.Labels[crossapiextensionsv1.LabelCompositionName]] = struct{}{}
		}

		for uniqueKind := range elements {
			// skip clearing loop for configured items
			if _, found := itemsToKeep[uniqueKind]; found {
				continue
			}
			kindRevisions := &crossapiextensionsv1.CompositionRevisionList{}
			ml := client.MatchingLabels{}
			ml[crossapiextensionsv1.LabelCompositionName] = uniqueKind

			if err = c.crossplaneClient.List(ctx, kindRevisions, ml); err != nil {
				return clearedRevsCount, errors.Wrap(err, "cannot list composition revisions for a label")
			}

			sort.Slice(kindRevisions.Items, func(i, j int) bool {
				// sort in descending mode
				return kindRevisions.Items[i].Spec.Revision > kindRevisions.Items[j].Spec.Revision
			})

			var revisionsToKeep = make(map[string]struct{})

			for idx, rev := range kindRevisions.Items {
				// keep top N recent revisions in local cache to keep
				if keepTopNItems > idx {
					revisionsToKeep[rev.GetName()] = struct{}{}
				}
			}
			for _, rev := range kindRevisions.Items {
				// remove old revisions that aren't in top N recent revisions
				if _, found := revisionsToKeep[rev.GetName()]; !found {
					if err = c.crossplaneClient.Delete(ctx, &rev); resource.IgnoreNotFound(err) != nil {
						return clearedRevsCount, errors.Wrap(err, "cannot delete composition revision")
					}
					clearedRevsCount++
				}
			}
		}

	}

	return clearedRevsCount, nil
}

func NewCompositionRevisionCleanupJob(log logging.Logger, k8sClientset *kubernetes.Clientset, crossplaneClient client.Client) Job {
	return CompositionRevisionCleanupJob{
		log:              log,
		k8sClientset:     k8sClientset,
		crossplaneClient: crossplaneClient,
	}
}
