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

package authorization

import (
	"context"
	"testing"

	authorizationmgmt "github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/authorization/fake"
)

const (
	subscription     = "a-cool-subscription"
	servicePrincipal = "cool-sp"
	subnet           = "cool-subnet"
	name             = "cool-role"
)

var (
	ctx       = context.Background()
	errorBoom = errors.New("boom")
)

func TestCreateRoleAssignment(t *testing.T) {
	cases := []struct {
		name         string
		client       *RoleAssignmentsClient
		sp           string
		vnetSubnetID string
		roleName     string
		wantErr      error
	}{
		{
			name: "Successful",
			client: &RoleAssignmentsClient{
				subscriptionID: subscription,
				client: &fake.MockRoleAssignmentsClient{
					MockCreate: func(_ context.Context, _ string, _ string, _ authorizationmgmt.RoleAssignmentCreateParameters) (result authorizationmgmt.RoleAssignment, err error) {
						return authorizationmgmt.RoleAssignment{}, nil
					},
				},
			},
			sp:           servicePrincipal,
			vnetSubnetID: subnet,
			roleName:     name,
		},
		{
			name: "Unsuccessful",
			client: &RoleAssignmentsClient{
				subscriptionID: subscription,
				client: &fake.MockRoleAssignmentsClient{
					MockCreate: func(_ context.Context, _ string, _ string, _ authorizationmgmt.RoleAssignmentCreateParameters) (result authorizationmgmt.RoleAssignment, err error) {
						return authorizationmgmt.RoleAssignment{}, errorBoom
					},
				},
			},
			sp:           servicePrincipal,
			vnetSubnetID: subnet,
			roleName:     name,
			wantErr:      errorBoom,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.client.CreateRoleAssignment(ctx, tc.sp, tc.vnetSubnetID, tc.roleName)

			if diff := cmp.Diff(tc.wantErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.CreateRoleAssignment(...): want error != got error:\n%s", diff)
			}

		})
	}
}

func TestDeleteRoleAssignment(t *testing.T) {
	cases := []struct {
		name         string
		client       *RoleAssignmentsClient
		vnetSubnetID string
		roleName     string
		wantErr      error
	}{
		{
			name: "Successful",
			client: &RoleAssignmentsClient{
				subscriptionID: subscription,
				client: &fake.MockRoleAssignmentsClient{
					MockDelete: func(ctx context.Context, scope string, roleAssignmentName string) (result authorizationmgmt.RoleAssignment, err error) {
						return authorizationmgmt.RoleAssignment{}, nil
					},
				},
			},
			vnetSubnetID: subnet,
			roleName:     name,
		},
		{
			name: "Unsuccessful",
			client: &RoleAssignmentsClient{
				subscriptionID: subscription,
				client: &fake.MockRoleAssignmentsClient{
					MockDelete: func(ctx context.Context, scope string, roleAssignmentName string) (result authorizationmgmt.RoleAssignment, err error) {
						return authorizationmgmt.RoleAssignment{}, errorBoom
					},
				},
			},
			vnetSubnetID: subnet,
			roleName:     name,
			wantErr:      errorBoom,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.client.DeleteRoleAssignment(ctx, tc.vnetSubnetID, tc.roleName)

			if diff := cmp.Diff(tc.wantErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.DeleteRoleAssignment(...): want error != got error:\n%s", diff)
			}

		})
	}
}
