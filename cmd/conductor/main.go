/*
Copyright 2018 The Conductor Authors.

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

package main

import (
	"log"
	"time"

	awsapis "github.com/upbound/conductor/pkg/apis/aws"
	azureapis "github.com/upbound/conductor/pkg/apis/azure"
	gcpapis "github.com/upbound/conductor/pkg/apis/gcp"
	awscontroller "github.com/upbound/conductor/pkg/controller/aws"
	azurecontroller "github.com/upbound/conductor/pkg/controller/azure"
	gcpcontroller "github.com/upbound/conductor/pkg/controller/gcp"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Re-sync resources every minutes
	// TODO: 1 minute could be too aggressive - we will update this in the future.
	syncPeriod := 1 * time.Minute

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{SyncPeriod: &syncPeriod})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Adding schemes")

	// Setup Scheme for all resources
	if err := awsapis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatal(err)
	}

	if err := gcpapis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatal(err)
	}

	if err := azureapis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatal(err)
	}

	log.Printf("Adding controllers")

	// Setup all Controllers
	if err := awscontroller.AddToManager(mgr); err != nil {
		log.Fatal(err)
	}

	if err := gcpcontroller.AddToManager(mgr); err != nil {
		log.Fatal(err)
	}

	if err := azurecontroller.AddToManager(mgr); err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting the manager")

	// Start the Cmd
	log.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
