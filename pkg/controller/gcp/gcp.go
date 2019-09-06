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

package gcp

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplaneio/crossplane/pkg/controller/gcp/cache"
	"github.com/crossplaneio/crossplane/pkg/controller/gcp/compute"
	"github.com/crossplaneio/crossplane/pkg/controller/gcp/database"
	"github.com/crossplaneio/crossplane/pkg/controller/gcp/servicenetworking"
	"github.com/crossplaneio/crossplane/pkg/controller/gcp/storage"
)

// Controllers passes down config and adds individual controllers to the manager.
type Controllers struct{}

// SetupWithManager adds all GCP controllers to the manager.
func (c *Controllers) SetupWithManager(mgr ctrl.Manager) error {
	// TODO(muvaf): Move this interface and logic to controller-runtime as it's common to all.
	controllers := []interface {
		SetupWithManager(ctrl.Manager) error
	}{
		&cache.CloudMemorystoreInstanceClaimController{},
		&cache.CloudMemorystoreInstanceController{},
		&compute.GlobalAddressController{},
		&compute.GKEClusterClaimController{},
		&compute.GKEClusterController{},
		&compute.NetworkController{},
		&compute.SubnetworkController{},
		&database.PostgreSQLInstanceClaimController{},
		&database.MySQLInstanceClaimController{},
		&database.CloudsqlController{},
		&servicenetworking.ConnectionController{},
		&storage.BucketClaimController{},
		&storage.BucketController{},
	}
	for _, c := range controllers {
		if err := c.SetupWithManager(mgr); err != nil {
			return err
		}
	}
	return nil
}
