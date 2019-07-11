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
	"github.com/onsi/gomega"

	databasev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
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
