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

package s3

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

var s3ACL = map[storagev1alpha1.PredefinedACL]s3.BucketCannedACL{
	storagev1alpha1.ACLPrivate:           s3.BucketCannedACLPrivate,
	storagev1alpha1.ACLPublicRead:        s3.BucketCannedACLPublicRead,
	storagev1alpha1.ACLPublicReadWrite:   s3.BucketCannedACLPublicReadWrite,
	storagev1alpha1.ACLAuthenticatedRead: s3.BucketCannedACLAuthenticatedRead,
}

// AddClaim adds a controller that reconciles Bucket resource claims by
// managing Bucket resources to the supplied Manager.
func AddClaim(mgr manager.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(storagev1alpha1.BucketGroupVersionKind),
		resource.ClassKind(corev1alpha1.ResourceClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.S3BucketGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureS3Bucket),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", storagev1alpha1.BucketKind, controllerName))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrapf(err, "cannot create %s controller", name)
	}

	if err := c.Watch(&source.Kind{Type: &v1alpha1.S3Bucket{}}, &resource.EnqueueRequestForClaim{}); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", v1alpha1.S3BucketGroupVersionKind)
	}

	p := v1alpha1.S3BucketKindAPIVersion
	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &storagev1alpha1.Bucket{}},
		&handler.EnqueueRequestForObject{},
		resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p)),
	), "cannot watch for %s", storagev1alpha1.BucketGroupVersionKind)
}

// ConfigureS3Bucket configures the supplied resource (presumed
// to be a S3Bucket) using the supplied resource claim (presumed
// to be a Bucket) and resource class.
func ConfigureS3Bucket(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	b, cmok := cm.(*storagev1alpha1.Bucket)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), storagev1alpha1.BucketGroupVersionKind)
	}

	rs, csok := cs.(*corev1alpha1.ResourceClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), corev1alpha1.ResourceClassGroupVersionKind)
	}

	s3b, mgok := mg.(*v1alpha1.S3Bucket)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.S3BucketGroupVersionKind)
	}

	spec := v1alpha1.NewS3BucketSpec(rs.Parameters)

	if b.Spec.Name != "" {
		spec.NameFormat = b.Spec.Name
	}

	var err error
	spec.CannedACL, err = resolveClassClaimACL(spec.CannedACL, translateACL(b.Spec.PredefinedACL))
	if err != nil {
		return err
	}

	spec.LocalPermission, err = resolveClassClaimLocalPermissions(spec.LocalPermission, b.Spec.LocalPermission)
	if err != nil {
		return err
	}

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.ProviderReference
	spec.ReclaimPolicy = rs.ReclaimPolicy

	s3b.Spec = *spec

	return nil
}

func resolveClassClaimACL(classValue, claimValue *s3.BucketCannedACL) (*s3.BucketCannedACL, error) {
	if classValue == nil {
		return claimValue, nil
	}
	if claimValue == nil {
		return classValue, nil
	}
	v, err := resource.ResolveClassClaimValues(string(*classValue), string(*claimValue))
	acl := s3.BucketCannedACL(v)
	return &acl, err
}

func resolveClassClaimLocalPermissions(classValue, claimValue *storagev1alpha1.LocalPermissionType) (*storagev1alpha1.LocalPermissionType, error) {
	if classValue == nil {
		return claimValue, nil
	}
	if claimValue == nil {
		return classValue, nil
	}
	v, err := resource.ResolveClassClaimValues(string(*classValue), string(*claimValue))
	perm := storagev1alpha1.LocalPermissionType(v)
	return &perm, err
}

func translateACL(acl *storagev1alpha1.PredefinedACL) *s3.BucketCannedACL {
	if acl == nil {
		return nil
	}
	s3acl, found := s3ACL[*acl]
	if !found {
		return nil
	}
	return &s3acl
}
