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
	"github.com/crossplane/crossplane/apis/apiextensions"
	"k8s.io/client-go/kubernetes/scheme"
	"maps"
	"os"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/job"
)

// Command runs the Crossplane job.
type Command struct {
	Run StartCommand `cmd:"" help:"Start a Crossplane job."`
}

type StartCommand struct {
	Job           string `help:"Name of a job."                         name:"job"                     short:"j"`
	ItemsToKeep   string `help:"Comma delimited list of items to keep." name:"items-to-keep"           short:"i"`
	KeepTopNItems int    `default:"1"                                   help:"Number of items to keep" name:"keep-top-n-items" short:"n"`
}

// Run invokes a Crossplane command.
func (c *StartCommand) Run(s *runtime.Scheme, log logging.Logger) error {
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

	return c.ExecuteJob(ctx, cfg, log)
}

// ExecuteJob runs a Crossplane job.
func (c *StartCommand) ExecuteJob(ctx context.Context, cfg *rest.Config, log logging.Logger) error {
	crossplaneClient, err := client.New(cfg, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return errors.Wrap(err, "Error while initializing kube client")
	}

	// add package scheme
	_ = apiextensions.AddToScheme(crossplaneClient.Scheme())

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "cannot create K8S client set")
	}

	jobs := job.NewJobs(log, k8sClient, crossplaneClient)

	if foundJob, found := jobs[c.Job]; found {
		itemsToKeepSet := map[string]struct{}{}
		for _, itemToKeep := range strings.Split(c.ItemsToKeep, ",") {
			itemsToKeepSet[itemToKeep] = struct{}{}
		}

		processedItems, err := foundJob.Run(ctx, itemsToKeepSet, c.KeepTopNItems)
		log.Info(fmt.Sprintf("No of processed items: %d", processedItems))
		if err != nil {
			return errors.Wrap(err, "cannot complete job")
		}
	} else {
		log.Info(fmt.Sprintf("Available jobs: %s", slices.Collect(maps.Keys(jobs))))
	}

	return nil
}
