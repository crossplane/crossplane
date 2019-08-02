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

package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	databasev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// ConfigurePostgreSQLCloudsqlInstance configures the supplied instance (presumed
// to be a CloudsqlInstance) using the supplied instance claim (presumed to be a
// PostgreSQLInstance) and instance class.
func ConfigurePostgreSQLCloudsqlInstance(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	pg, cmok := cm.(*databasev1alpha1.PostgreSQLInstance)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), databasev1alpha1.PostgreSQLInstanceGroupVersionKind)
	}

	rs, csok := cs.(*corev1alpha1.ResourceClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), corev1alpha1.ResourceClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.CloudsqlInstance)
	if !mgok {
		return errors.Errorf("expected managed instance %s to be %s", mg.GetName(), v1alpha1.CloudsqlInstanceGroupVersionKind)
	}

	spec := v1alpha1.NewCloudSQLInstanceSpec(rs.Parameters)
	translated := translateVersion(pg.Spec.EngineVersion, v1alpha1.PostgresqlDBVersionPrefix)
	v, err := resource.ResolveClassClaimValues(spec.DatabaseVersion, translated)
	if err != nil {
		return err
	}
	spec.DatabaseVersion = v

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.ProviderReference
	spec.ReclaimPolicy = rs.ReclaimPolicy

	i.Spec = *spec

	return nil
}

// ConfigureMyCloudsqlInstance configures the supplied instance (presumed to be
// a CloudsqlInstance) using the supplied instance claim (presumed to be a
// MySQLInstance) and instance class.
func ConfigureMyCloudsqlInstance(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	my, cmok := cm.(*databasev1alpha1.MySQLInstance)
	if !cmok {
		return errors.Errorf("expected instance claim %s to be %s", cm.GetName(), databasev1alpha1.MySQLInstanceGroupVersionKind)
	}

	rs, csok := cs.(*corev1alpha1.ResourceClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), corev1alpha1.ResourceClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.CloudsqlInstance)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.CloudsqlInstanceGroupVersionKind)
	}

	spec := v1alpha1.NewCloudSQLInstanceSpec(rs.Parameters)
	translated := translateVersion(my.Spec.EngineVersion, v1alpha1.MysqlDBVersionPrefix)
	v, err := resource.ResolveClassClaimValues(spec.DatabaseVersion, translated)
	if err != nil {
		return err
	}
	spec.DatabaseVersion = v

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.ProviderReference
	spec.ReclaimPolicy = rs.ReclaimPolicy

	i.Spec = *spec

	return nil
}

func translateVersion(version, versionPrefix string) string {
	if version == "" {
		return ""
	}
	return fmt.Sprintf("%s_%s", versionPrefix, strings.Replace(version, ".", "_", -1))
}
