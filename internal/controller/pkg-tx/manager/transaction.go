/*
Copyright 2025 The Crossplane Authors.

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

package manager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
)

// TransactionName returns a deterministic Transaction name for a package.
func TransactionName(pkg v1.Package) string {
	input := fmt.Sprintf("%s/%s/%s/gen-%d",
		pkg.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		pkg.GetObjectKind().GroupVersionKind().Kind,
		pkg.GetName(),
		pkg.GetGeneration())
	hash := sha256.Sum256([]byte(input))
	shortHash := hex.EncodeToString(hash[:4])
	return fmt.Sprintf("tx-%s-%s", pkg.GetName(), shortHash)
}

// NewTransaction creates a new Transaction for the given package and change type.
func NewTransaction(pkg v1.Package, changeType v1alpha1.ChangeType) *v1alpha1.Transaction {
	tx := &v1alpha1.Transaction{
		ObjectMeta: metav1.ObjectMeta{
			Name: TransactionName(pkg),
			Labels: map[string]string{
				"pkg.crossplane.io/package":      pkg.GetName(),
				"pkg.crossplane.io/package-type": pkg.GetObjectKind().GroupVersionKind().Kind,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         pkg.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:               pkg.GetObjectKind().GroupVersionKind().Kind,
					Name:               pkg.GetName(),
					UID:                pkg.GetUID(),
					Controller:         ptr.To(false),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: v1alpha1.TransactionSpec{
			Change: changeType,
		},
	}

	switch changeType {
	case v1alpha1.ChangeTypeInstall:
		tx.Spec.Install = &v1alpha1.InstallSpec{
			Package: PackageSnapshot(pkg),
		}
	case v1alpha1.ChangeTypeDelete:
		tx.Spec.Delete = &v1alpha1.DeleteSpec{
			Source: pkg.GetSource(),
		}
	case v1alpha1.ChangeTypeReplace:
		// TODO(negz): Implement replace.
	}

	return tx
}

// PackageSnapshot creates a PackageSnapshot from a Package resource.
func PackageSnapshot(pkg v1.Package) v1alpha1.PackageSnapshot {
	spec := v1alpha1.PackageSnapshotSpec{
		PackageSpec: v1.PackageSpec{
			Package:                     pkg.GetSource(),
			RevisionActivationPolicy:    pkg.GetActivationPolicy(),
			RevisionHistoryLimit:        pkg.GetRevisionHistoryLimit(),
			PackagePullSecrets:          pkg.GetPackagePullSecrets(),
			PackagePullPolicy:           pkg.GetPackagePullPolicy(),
			IgnoreCrossplaneConstraints: pkg.GetIgnoreCrossplaneConstraints(),
			SkipDependencyResolution:    pkg.GetSkipDependencyResolution(),
			CommonLabels:                pkg.GetCommonLabels(),
		},
	}

	if pkgrt, ok := pkg.(v1.PackageWithRuntime); ok {
		spec.PackageRuntimeSpec = v1.PackageRuntimeSpec{
			RuntimeConfigReference: pkgrt.GetRuntimeConfigRef(),
		}
	}

	return v1alpha1.PackageSnapshot{
		APIVersion: pkg.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       pkg.GetObjectKind().GroupVersionKind().Kind,
		Metadata: v1alpha1.PackageMetadata{
			Name:   pkg.GetName(),
			UID:    pkg.GetUID(),
			Labels: pkg.GetLabels(),
		},
		Spec: spec,
	}
}
