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
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Command struct {
	Run startCommand `cmd:"" help:"Start a Crossplane job."`
}

type startCommand struct {
	Job string `help:"Name of a job." short:"j"`
}

const removeUnusedCompositionRevisionJob = "removeUnusedCompositionRevision"

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

	// Create a dynamic client
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "cannot create client")
	}

	switch c.Job {
	case removeUnusedCompositionRevisionJob:
		//todo: clean composite revisions across namespaces
		gvr := schema.GroupVersionResource{
			Group:    "apiextensions.crossplane.io",
			Version:  "v1",
			Resource: "compositionrevisions",
		}
		list, err := dynamicClient.Resource(gvr).Namespace("default").List(ctx, metav1.ListOptions{})
		if err != nil {
			return errors.Wrap(err, "cannot list ")
		}
	default:
		log.Info(fmt.Sprintf("Available jobs: %s", removeUnusedCompositionRevisionJob))
	}

	return nil
}
