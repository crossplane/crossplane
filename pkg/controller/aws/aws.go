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

package aws

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplaneio/crossplane/pkg/controller/aws/cache"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/compute"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/identity/iamrole"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/identity/iamrolepolicyattachment"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/network/internetgateway"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/network/routetable"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/network/securitygroup"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/network/subnet"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/network/vpc"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/rds"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/rds/dbsubnetgroup"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/s3"
)

// Controllers passes down config and adds individual controllers to the manager.
type Controllers struct{}

// SetupWithManager adds all AWS controllers to the manager.
func (c *Controllers) SetupWithManager(mgr ctrl.Manager) error {

	controllers := []interface {
		SetupWithManager(ctrl.Manager) error
	}{
		&cache.ReplicationGroupClaimController{},
		&cache.ReplicationGroupController{},
		&compute.EKSClusterClaimController{},
		&compute.EKSClusterController{},
		&rds.PostgreSQLInstanceClaimController{},
		&rds.MySQLInstanceClaimController{},
		&rds.InstanceController{},
		&s3.BucketClaimController{},
		&s3.BucketController{},
		&iamrole.Controller{},
		&iamrolepolicyattachment.Controller{},
		&vpc.Controller{},
		&subnet.Controller{},
		&securitygroup.Controller{},
		&internetgateway.Controller{},
		&routetable.Controller{},
		&dbsubnetgroup.Controller{},
	}

	for _, c := range controllers {
		if err := c.SetupWithManager(mgr); err != nil {
			return err
		}
	}
	return nil
}
