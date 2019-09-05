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

package azure

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	"github.com/Azure/azure-sdk-for-go/services/postgresql/mgmt/2017-12-01/postgresql"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	databasev1alpha1 "github.com/crossplaneio/crossplane/azure/apis/database/v1alpha1"
)

const (
	uid           = types.UID("definitely-a-uuid")
	vnetRuleName  = "myvnetrule"
	serverName    = "myserver"
	rgName        = "myrg"
	vnetSubnetID  = "a/very/important/subnet"
	ignoreMissing = true

	id           = "very-cool-id"
	resourceType = "very-cool-type"
)

func TestSQLServerStatusMessage(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		state           mysql.ServerState
		expectedMessage string
	}{
		{mysql.ServerStateDisabled, "SQL Server instance foo is disabled"},
		{mysql.ServerStateDropping, "SQL Server instance foo is dropping"},
		{mysql.ServerStateReady, "SQL Server instance foo is ready"},
		{mysql.ServerState("FooState"), "SQL Server instance foo is in an unknown state FooState"},
	}

	for _, tt := range cases {
		message := SQLServerStatusMessage("foo", string(tt.state))
		g.Expect(message).To(gomega.Equal(tt.expectedMessage))
	}
}

func TestSQLServerSkuName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		pricingTier     databasev1alpha1.PricingTierSpec
		expectedSkuName string
		expectedErr     string
	}{
		// empty pricing tier spec
		{databasev1alpha1.PricingTierSpec{}, "", "tier '' is not one of the supported values: [Basic GeneralPurpose MemoryOptimized]"},
		// spec that has unknown tier
		{databasev1alpha1.PricingTierSpec{Tier: "Foo", Family: "Gen4", VCores: 1}, "", "tier 'Foo' is not one of the supported values: [Basic GeneralPurpose MemoryOptimized]"},
		// B_Gen4_1
		{databasev1alpha1.PricingTierSpec{Tier: "Basic", Family: "Gen4", VCores: 1}, "B_Gen4_1", ""},
		// MO_Gen5_8
		{databasev1alpha1.PricingTierSpec{Tier: "MemoryOptimized", Family: "Gen5", VCores: 8}, "MO_Gen5_8", ""},
	}

	for _, tt := range cases {
		skuName, err := SQLServerSkuName(tt.pricingTier)
		if tt.expectedErr != "" {
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err.Error()).To(gomega.Equal(tt.expectedErr))
		} else {
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(skuName).To(gomega.Equal(tt.expectedSkuName))
		}
	}
}

func TestToSslEnforcement(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		sslEnforced bool
		expected    mysql.SslEnforcementEnum
	}{
		{true, mysql.SslEnforcementEnumEnabled},
		{false, mysql.SslEnforcementEnumDisabled},
	}

	for _, tt := range cases {
		actual := ToSslEnforcement(tt.sslEnforced)
		g.Expect(actual).To(gomega.Equal(tt.expected))
	}
}

func TestToGeoRedundantBackup(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		geoRedundantBackup bool
		expected           mysql.GeoRedundantBackup
	}{
		{true, mysql.Enabled},
		{false, mysql.Disabled},
	}

	for _, tt := range cases {
		actual := ToGeoRedundantBackup(tt.geoRedundantBackup)
		g.Expect(actual).To(gomega.Equal(tt.expected))
	}
}

func TestNewMySQLVirtualNetworkRuleParameters(t *testing.T) {
	cases := []struct {
		name string
		r    *databasev1alpha1.MysqlServerVirtualNetworkRule
		want mysql.VirtualNetworkRule
	}{
		{
			name: "Successful",
			r: &databasev1alpha1.MysqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID:           vnetSubnetID,
						IgnoreMissingVnetServiceEndpoint: ignoreMissing,
					},
				},
			},
			want: mysql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &mysql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           to.StringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: to.BoolPtr(ignoreMissing),
				},
			},
		},
		{
			name: "SuccessfulPartial",
			r: &databasev1alpha1.MysqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID: vnetSubnetID,
					},
				},
			},
			want: mysql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &mysql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           to.StringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: to.BoolPtr(false),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewMySQLVirtualNetworkRuleParameters(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewMySQLVirtualNetworkRuleParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestMySQLServerVirtualNetworkRuleNeedsUpdate(t *testing.T) {
	cases := []struct {
		name string
		kube *databasev1alpha1.MysqlServerVirtualNetworkRule
		az   mysql.VirtualNetworkRule
		want bool
	}{
		{
			name: "NoUpdateNeeded",
			kube: &databasev1alpha1.MysqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID:           vnetSubnetID,
						IgnoreMissingVnetServiceEndpoint: ignoreMissing,
					},
				},
			},
			az: mysql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &mysql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(ignoreMissing),
				},
			},
			want: false,
		},
		{
			name: "UpdateNeededVirtualNetworkSubnetID",
			kube: &databasev1alpha1.MysqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID:           vnetSubnetID,
						IgnoreMissingVnetServiceEndpoint: ignoreMissing,
					},
				},
			},
			az: mysql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &mysql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr("some/other/subnet"),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(ignoreMissing),
				},
			},
			want: true,
		},
		{
			name: "UpdateNeededIgnoreMissingVnetServiceEndpoint",
			kube: &databasev1alpha1.MysqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID:           vnetSubnetID,
						IgnoreMissingVnetServiceEndpoint: ignoreMissing,
					},
				},
			},
			az: mysql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &mysql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(!ignoreMissing),
				},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MySQLServerVirtualNetworkRuleNeedsUpdate(tc.kube, tc.az)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MySQLServerVirtualNetworkRuleNeedsUpdate(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestMySQLVirtualNetworkRuleStatusFromAzure(t *testing.T) {
	cases := []struct {
		name string
		r    mysql.VirtualNetworkRule
		want databasev1alpha1.VirtualNetworkRuleStatus
	}{
		{
			name: "SuccessfulFull",
			r: mysql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				ID:   ToStringPtr(id),
				Type: ToStringPtr(resourceType),
				VirtualNetworkRuleProperties: &mysql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(ignoreMissing),
					State:                            mysql.Ready,
				},
			},
			want: databasev1alpha1.VirtualNetworkRuleStatus{
				State: "Ready",
				ID:    id,
				Type:  resourceType,
			},
		},
		{
			name: "SuccessfulPartial",
			r: mysql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				ID:   ToStringPtr(id),
				VirtualNetworkRuleProperties: &mysql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(ignoreMissing),
					State:                            mysql.Ready,
				},
			},
			want: databasev1alpha1.VirtualNetworkRuleStatus{
				State: "Ready",
				ID:    id,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MySQLVirtualNetworkRuleStatusFromAzure(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewVirtualNetworkParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestNewPostgreSQLVirtualNetworkRuleParameters(t *testing.T) {
	cases := []struct {
		name string
		r    *databasev1alpha1.PostgresqlServerVirtualNetworkRule
		want postgresql.VirtualNetworkRule
	}{
		{
			name: "Successful",
			r: &databasev1alpha1.PostgresqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID:           vnetSubnetID,
						IgnoreMissingVnetServiceEndpoint: ignoreMissing,
					},
				},
			},
			want: postgresql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &postgresql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           to.StringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: to.BoolPtr(ignoreMissing),
				},
			},
		},
		{
			name: "SuccessfulPartial",
			r: &databasev1alpha1.PostgresqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID: vnetSubnetID,
					},
				},
			},
			want: postgresql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &postgresql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           to.StringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: to.BoolPtr(false),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewPostgreSQLVirtualNetworkRuleParameters(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MySQLVirtualNetworkRuleStatusFromAzure(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestPostgreSQLServerVirtualNetworkRuleNeedsUpdate(t *testing.T) {
	cases := []struct {
		name string
		kube *databasev1alpha1.PostgresqlServerVirtualNetworkRule
		az   postgresql.VirtualNetworkRule
		want bool
	}{
		{
			name: "NoUpdateNeeded",
			kube: &databasev1alpha1.PostgresqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID:           vnetSubnetID,
						IgnoreMissingVnetServiceEndpoint: ignoreMissing,
					},
				},
			},
			az: postgresql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &postgresql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(ignoreMissing),
				},
			},
			want: false,
		},
		{
			name: "UpdateNeededVirtualNetworkSubnetID",
			kube: &databasev1alpha1.PostgresqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID:           vnetSubnetID,
						IgnoreMissingVnetServiceEndpoint: ignoreMissing,
					},
				},
			},
			az: postgresql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &postgresql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr("some/other/subnet"),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(ignoreMissing),
				},
			},
			want: true,
		},
		{
			name: "UpdateNeededIgnoreMissingVnetServiceEndpoint",
			kube: &databasev1alpha1.PostgresqlServerVirtualNetworkRule{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: databasev1alpha1.VirtualNetworkRuleSpec{
					Name:              vnetRuleName,
					ServerName:        serverName,
					ResourceGroupName: rgName,
					VirtualNetworkRuleProperties: databasev1alpha1.VirtualNetworkRuleProperties{
						VirtualNetworkSubnetID:           vnetSubnetID,
						IgnoreMissingVnetServiceEndpoint: ignoreMissing,
					},
				},
			},
			az: postgresql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				VirtualNetworkRuleProperties: &postgresql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(!ignoreMissing),
				},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PostgreSQLServerVirtualNetworkRuleNeedsUpdate(tc.kube, tc.az)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MySQLServerVirtualNetworkRuleNeedsUpdate(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestPostgreSQLVirtualNetworkRuleStatusFromAzure(t *testing.T) {
	cases := []struct {
		name string
		r    postgresql.VirtualNetworkRule
		want databasev1alpha1.VirtualNetworkRuleStatus
	}{
		{
			name: "SuccessfulFull",
			r: postgresql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				ID:   ToStringPtr(id),
				Type: ToStringPtr(resourceType),
				VirtualNetworkRuleProperties: &postgresql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(ignoreMissing),
					State:                            postgresql.Ready,
				},
			},
			want: databasev1alpha1.VirtualNetworkRuleStatus{
				State: "Ready",
				ID:    id,
				Type:  resourceType,
			},
		},
		{
			name: "SuccessfulPartial",
			r: postgresql.VirtualNetworkRule{
				Name: ToStringPtr(vnetRuleName),
				ID:   ToStringPtr(id),
				VirtualNetworkRuleProperties: &postgresql.VirtualNetworkRuleProperties{
					VirtualNetworkSubnetID:           ToStringPtr(vnetSubnetID),
					IgnoreMissingVnetServiceEndpoint: ToBoolPtr(ignoreMissing),
					State:                            postgresql.Ready,
				},
			},
			want: databasev1alpha1.VirtualNetworkRuleStatus{
				State: "Ready",
				ID:    id,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PostgreSQLVirtualNetworkRuleStatusFromAzure(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("PostgreSQLVirtualNetworkRuleStatusFromAzure(...): -want, +got\n%s", diff)
			}
		})
	}
}
