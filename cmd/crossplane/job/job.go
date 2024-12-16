/*
Copyright 2024 The Crossplane Authors.

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

// Package job implements Crossplane' jobs logic.
package job

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	crossapiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Command struct {
	Run startCommand `cmd:"" help:"Start a Crossplane job."`
}

type startCommand struct {
	Job string `help:"Name of a job." short:"j"`
}

const removeUnusedCompositionRevisionJob = "removeUnusedCompositionRevision"

const keepTopNItems = 3

var compositionsToKeep = map[string]struct{}{
	"test": struct{}{},
}

// Run a Crossplane job.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error { //nolint:gocognit // Only slightly over.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "cannot get config")
	}

	cfg.WarningHandler = rest.NewWarningWriter(os.Stderr, rest.WarningWriterOptions{
		// Warnings from API requests should be deduplicated so they are only logged once
		Deduplicate: true,
	})

	// The claim and XR controllers don't use the manager's cache or client.
	// They use their own. They're setup later in this method.
	eb := record.NewBroadcaster()

	eb.StartLogging(func(format string, args ...interface{}) {
		log.Debug(fmt.Sprintf(format, args...))
	})
	defer eb.Shutdown()

	crossplaneSchema := runtime.NewScheme()
	_ = crossapiextensionsv1.AddToScheme(crossplaneSchema)

	crossplaneClient, err := client.New(cfg, client.Options{Scheme: crossplaneSchema})
	if err != nil {
		return errors.Wrap(err, "cannot create client")
	}

	k8sClientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "cannot create K8S client set")
	}

	switch c.Job {
	case removeUnusedCompositionRevisionJob:
		// TODO, move out logic and prepare for adding more jobs, especially clearing other revisions
		namespaces, err := k8sClientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return errors.Wrap(err, "cannot list namespaces")
		}

		clearedRevsCount := 0
		defer func() {
			log.Info(fmt.Sprintf("Cleared revisions: %d", clearedRevsCount))
		}()

		for _, ns := range namespaces.Items {
			allRevisions := &crossapiextensionsv1.CompositionRevisionList{}
			err := crossplaneClient.List(ctx, allRevisions, client.InNamespace(ns.GetName()))
			if err != nil {
				return errors.Wrap(err, "cannot list composition revisions")
			}

			var elements = make(map[string]struct{})

			for _, rev := range allRevisions.Items {
				elements[rev.ObjectMeta.Labels[crossapiextensionsv1.LabelCompositionName]] = struct{}{}
			}

			for uniqueKind := range elements {
				// skip clearing loop for configured items
				if _, found := compositionsToKeep[uniqueKind]; found {
					continue
				}
				kindRevisions := &crossapiextensionsv1.CompositionRevisionList{}
				ml := client.MatchingLabels{}
				ml[crossapiextensionsv1.LabelCompositionName] = uniqueKind

				if err := crossplaneClient.List(ctx, kindRevisions, ml); err != nil {
					return errors.Wrap(err, "cannot list composition revisions for a label")
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
						if err := crossplaneClient.Delete(ctx, &rev); resource.IgnoreNotFound(err) != nil {
							return errors.Wrap(err, "cannot delete composition revision")
						}
						clearedRevsCount++
					}
				}
			}

		}
	default:
		log.Info(fmt.Sprintf("Available jobs: %s", removeUnusedCompositionRevisionJob))
	}

	return nil
}
