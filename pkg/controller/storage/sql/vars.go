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

package sql

import (
	"context"

	awsdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/database/v1alpha1"
	azuredbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	gcpdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
	"github.com/crossplaneio/crossplane/pkg/log"
)

var (
	logger = log.Log.WithName(v1alpha1.Group)
	ctx    = context.Background()

	// map of supported resource handlers
	handlers = map[string]corecontroller.ResourceHandler{
		awsdbv1alpha1.RDSInstanceKindAPIVersion:        &RDSInstanceHandler{},
		azuredbv1alpha1.MysqlServerKindAPIVersion:      &AzureMySQLServerHandler{},
		azuredbv1alpha1.PostgresqlServerKindAPIVersion: &AzurePostgreSQLServerHandler{},
		gcpdbv1alpha1.CloudsqlInstanceKindAPIVersion:   &CloudSQLServerHandler{},
	}
)
