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

package v1alpha1

import (
	"strings"

	"github.com/docker/distribution/reference"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// +kubebuilder:object:root=true

// A PackageInstall requests a package be installed to Crossplane.
// +kubebuilder:resource:categories=crossplane
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditionedStatus.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SOURCE",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="PACKAGE",type="string",JSONPath=".spec.package"
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".spec.crd"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type PackageInstall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageInstallSpec   `json:"spec,omitempty"`
	Status PackageInstallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PackageInstallList contains a list of PackageInstall.
type PackageInstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PackageInstall `json:"items"`
}

// PackageInstallSpec specifies details about a request to install a package to
// Crossplane.
type PackageInstallSpec struct {
	PackageControllerOptions `json:",inline"`

	// Source is the domain name for the package registry hosting the package
	// being requested, e.g., registry.crossplane.io
	Source string `json:"source,omitempty"`

	// Package is the name of the package package that is being requested, e.g.,
	// myapp. Either Package or CustomResourceDefinition can be specified.
	Package string `json:"package,omitempty"`

	// CustomResourceDefinition is the full name of a CRD that is owned by the
	// package being requested. This can be a convenient way of installing a
	// package when the desired CRD is known, but the package name that contains
	// it is not known. Either Package or CustomResourceDefinition can be
	// specified.
	CustomResourceDefinition string `json:"crd,omitempty"`
}

// PackageControllerOptions allow for changes in the Package extraction and
// deployment controllers. These can affect how images are fetched and how
// Package derived resources are created.
type PackageControllerOptions struct {
	// ImagePullSecrets are named secrets in the same workspace that can be used
	// to fetch Packages from private repositories and to run controllers from
	// private repositories
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// ImagePullPolicy defines the pull policy for all images used during
	// Package extraction and when running the Package controller.
	// https://kubernetes.io/docs/concepts/configuration/overview/#container-images
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ServiceAccount options allow for changes to the ServiceAccount the
	// Package Manager creates for the Package's controller
	ServiceAccount *ServiceAccountOptions `json:"serviceAccount,omitempty"`
}

// PackageInstallStatus represents the observed state of a PackageInstall.
type PackageInstallStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`

	InstallJob    *corev1.ObjectReference `json:"installJob,omitempty"`
	PackageRecord *corev1.ObjectReference `json:"packageRecord,omitempty"`
}

// Image returns the Package prefixed with a source (if available). If the
// package format is not understood it is returned unmodified to be handled by
// Kubernetes.
func (spec PackageInstallSpec) Image() string {
	image, err := spec.ImageWithSource(spec.Package)
	if err != nil {
		return spec.Package
	}
	return image
}

// ImageWithSource applies a source to a container image URI only if the image
// does not appear to contain a source. Source is some combination of scheme,
// host, port, and prefix where host is required.
func (spec PackageInstallSpec) ImageWithSource(image string) (string, error) {
	// no alternate source to substitute
	if len(spec.Source) == 0 {
		return image, nil
	}

	// prepend source to image when it does not have a source
	named, err := reference.ParseNormalizedNamed(image)

	if err != nil {
		return "", err
	}

	// reference.Domain returns docker.io when no domain is found. If the image
	// didn't explicitly start with docker.io, ignore it. In these cases, we
	// want to apply the PackageInstall source.
	domain := reference.Domain(named)
	if strings.Index(image, domain) != 0 {
		return strings.Trim(spec.Source, "/") + "/" + image, nil
	}

	// image contained a source
	return image, nil
}

// GetPackage gets the Spec of the PackageInstall
func (si *PackageInstall) GetPackage() string {
	return si.Spec.Package
}

// GetPackage gets the Spec of the ClusterPackageInstall
func (si *ClusterPackageInstall) GetPackage() string {
	return si.Spec.Package
}

// SetSource sets the Source of the PackageInstall Spec
func (si *PackageInstall) SetSource(src string) {
	si.Spec.Source = src
}

// SetSource sets the Source of the ClusterPackageInstall Spec
func (si *ClusterPackageInstall) SetSource(src string) {
	si.Spec.Source = src
}

// ImageWithSource modifies the supplied image with the source of the
// PackageInstall
func (si *PackageInstall) ImageWithSource(img string) (string, error) {
	return si.Spec.ImageWithSource(img)
}

// ImageWithSource modifies the supplied image with the source of the
// ClusterPackageInstall
func (si *ClusterPackageInstall) ImageWithSource(img string) (string, error) {
	return si.Spec.ImageWithSource(img)
}

// PermissionScope gets the required app.yaml permissionScope value
// ("Namespaced") for PackageInstall
func (si *PackageInstall) PermissionScope() string { return string(apiextensions.NamespaceScoped) }

// PermissionScope gets the required app.yaml permissionScope value ("Cluster")
// for ClusterPackageInstall
func (si *ClusterPackageInstall) PermissionScope() string { return string(apiextensions.ClusterScoped) }

// SetConditions sets the PackageInstall's Status conditions
func (si *PackageInstall) SetConditions(c ...runtimev1alpha1.Condition) {
	si.Status.SetConditions(c...)
}

// SetConditions sets the ClusterPackageInstall's Status conditions
func (si *ClusterPackageInstall) SetConditions(c ...runtimev1alpha1.Condition) {
	si.Status.SetConditions(c...)
}

// GetImagePullSecrets gets the ImagePullSecrets of the ClusterPackageInstall
// Spec
func (si *ClusterPackageInstall) GetImagePullSecrets() []corev1.LocalObjectReference {
	return si.Spec.ImagePullSecrets
}

// GetImagePullSecrets gets the ImagePullSecrets of the PackageInstall Spec
func (si *PackageInstall) GetImagePullSecrets() []corev1.LocalObjectReference {
	return si.Spec.ImagePullSecrets
}

// SetImagePullSecrets sets the ImagePullSecrets of the PackageInstall Spec
func (si *PackageInstall) SetImagePullSecrets(secrets []corev1.LocalObjectReference) {
	si.Spec.ImagePullSecrets = secrets
}

// SetImagePullSecrets sets the ImagePullSecrets of the ClusterPackageInstall
// Spec
func (si *ClusterPackageInstall) SetImagePullSecrets(secrets []corev1.LocalObjectReference) {
	si.Spec.ImagePullSecrets = secrets
}

// GetImagePullPolicy gets the ImagePullPolicy of the ClusterPackageInstall Spec
func (si *ClusterPackageInstall) GetImagePullPolicy() corev1.PullPolicy {
	return si.Spec.ImagePullPolicy
}

// GetImagePullPolicy gets the ImagePullPolicy of the PackageInstall Spec
func (si *PackageInstall) GetImagePullPolicy() corev1.PullPolicy {
	return si.Spec.ImagePullPolicy
}

// SetImagePullPolicy sets the ImagePullPolicy of the PackageInstall Spec
func (si *PackageInstall) SetImagePullPolicy(policy corev1.PullPolicy) {
	si.Spec.ImagePullPolicy = policy
}

// SetImagePullPolicy sets the ImagePullPolicy of the ClusterPackageInstall Spec
func (si *ClusterPackageInstall) SetImagePullPolicy(policy corev1.PullPolicy) {
	si.Spec.ImagePullPolicy = policy
}

// GetServiceAccountAnnotations gets the Annotations of the
// ClusterPackageInstall Spec ServiceAccount
func (si *ClusterPackageInstall) GetServiceAccountAnnotations() map[string]string {
	if si.Spec.ServiceAccount == nil {
		return map[string]string{}
	}
	return si.Spec.ServiceAccount.Annotations
}

// GetServiceAccountAnnotations gets the Annotations of the PackageInstall Spec
// ServiceAccount
func (si *PackageInstall) GetServiceAccountAnnotations() map[string]string {
	if si.Spec.ServiceAccount == nil {
		return map[string]string{}
	}
	return si.Spec.ServiceAccount.Annotations
}

// SetServiceAccountAnnotations sets the Annotations of the PackageInstall Spec
// ServiceAccount
func (si *PackageInstall) SetServiceAccountAnnotations(annotations map[string]string) {
	if si.Spec.ServiceAccount == nil {
		si.Spec.ServiceAccount = &ServiceAccountOptions{}
	}
	si.Spec.ServiceAccount.Annotations = annotations
}

// SetServiceAccountAnnotations sets the Annotations of the
// ClusterPackageInstall Spec ServiceAccount
func (si *ClusterPackageInstall) SetServiceAccountAnnotations(annotations map[string]string) {
	if si.Spec.ServiceAccount == nil {
		si.Spec.ServiceAccount = &ServiceAccountOptions{}
	}
	si.Spec.ServiceAccount.Annotations = annotations
}

// InstallJob gets the ClusterPackageInstall's Status InstallJob
func (si *ClusterPackageInstall) InstallJob() *corev1.ObjectReference {
	return si.Status.InstallJob
}

// InstallJob gets the PackageInstall's Status InstallJob
func (si *PackageInstall) InstallJob() *corev1.ObjectReference {
	return si.Status.InstallJob
}

// SetInstallJob sets the ClusterPackageInstall's Status InstallJob
func (si *ClusterPackageInstall) SetInstallJob(job *corev1.ObjectReference) {
	si.Status.InstallJob = job
}

// SetInstallJob sets the PackageInstall's Status InstallJob
func (si *PackageInstall) SetInstallJob(job *corev1.ObjectReference) {
	si.Status.InstallJob = job
}

// PackageRecord gets the ClusterPackageInstall's Status PackageRecord
func (si *ClusterPackageInstall) PackageRecord() *corev1.ObjectReference {
	return si.Status.PackageRecord
}

// SetPackageRecord sets the ClusterPackageInstall's Status PackageRecord
func (si *ClusterPackageInstall) SetPackageRecord(job *corev1.ObjectReference) {
	si.Status.PackageRecord = job
}

// SetPackageRecord sets the PackageInstall's Status PackageRecord
func (si *PackageInstall) SetPackageRecord(job *corev1.ObjectReference) {
	si.Status.PackageRecord = job
}

// PackageRecord gets the PackageInstall's Status PackageRecord
func (si *PackageInstall) PackageRecord() *corev1.ObjectReference {
	return si.Status.PackageRecord
}

// GroupVersionKind gets the GroupVersionKind of the PackageInstall
func (si *PackageInstall) GroupVersionKind() schema.GroupVersionKind {
	return PackageInstallGroupVersionKind
}

// GroupVersionKind gets the GroupVersionKind of the ClusterPackageInstall
func (si *ClusterPackageInstall) GroupVersionKind() schema.GroupVersionKind {
	return ClusterPackageInstallGroupVersionKind
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:generate=false

// PackageInstaller provides a common interface for PackageInstall and ClusterPackageInstall to share controller and reconciler logic
type PackageInstaller interface {
	metav1.Object
	runtime.Object
	schema.ObjectKind

	GetPackage() string
	GetImagePullPolicy() corev1.PullPolicy
	GetImagePullSecrets() []corev1.LocalObjectReference
	GetServiceAccountAnnotations() map[string]string
	ImageWithSource(string) (string, error)
	InstallJob() *corev1.ObjectReference
	PermissionScope() string
	SetConditions(c ...runtimev1alpha1.Condition)
	SetImagePullPolicy(corev1.PullPolicy)
	SetImagePullSecrets([]corev1.LocalObjectReference)
	SetServiceAccountAnnotations(map[string]string)
	SetSource(string)
	SetPackageRecord(*corev1.ObjectReference)
	SetInstallJob(*corev1.ObjectReference)
	PackageRecord() *corev1.ObjectReference
}

// +kubebuilder:object:root=true

// ClusterPackageInstall is the CRD type for a request to add a package to Crossplane.
// +kubebuilder:resource:categories=crossplane
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditionedStatus.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SOURCE",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="PACKAGE",type="string",JSONPath=".spec.package"
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".spec.crd"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterPackageInstall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageInstallSpec   `json:"spec,omitempty"`
	Status PackageInstallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterPackageInstallList contains a list of PackageInstall
type ClusterPackageInstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPackageInstall `json:"items"`
}

// +kubebuilder:object:root=true

// A Package that has been added to Crossplane.
// +kubebuilder:resource:categories=crossplane
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditionedStatus.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Package struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageSpec   `json:"spec,omitempty"`
	Status PackageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PackageList contains a list of Package.
type PackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Package `json:"items"`
}

// PackageSpec specifies the desired state of a Package.
type PackageSpec struct {
	AppMetadataSpec `json:",inline"`
	CRDs            CRDList         `json:"customresourcedefinitions,omitempty"`
	Controller      ControllerSpec  `json:"controller,omitempty"`
	Permissions     PermissionsSpec `json:"permissions,omitempty"`
}

// ServiceAccountAnnotations guarantees a map of annotations from a PackageSpec
func (spec PackageSpec) ServiceAccountAnnotations() map[string]string {
	annotations := map[string]string{}
	if sa := spec.Controller.ServiceAccount; sa != nil {
		for k, v := range sa.Annotations {
			annotations[k] = v
		}
	}
	return annotations
}

// PackageStatus represents the observed state of a Package.
type PackageStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`
	ControllerRef                     *corev1.ObjectReference `json:"controllerRef,omitempty"`
}

// PackageMetadataSpec defines metadata about the package application
// and package contents
type PackageMetadataSpec struct {
	APIVersion string `json:"apiVersion,omitempty"`

	AppMetadataSpec `json:",inline"`
}

// AppMetadataSpec defines metadata about the package application
type AppMetadataSpec struct {
	Title         string            `json:"title,omitempty"`
	OverviewShort string            `json:"overviewShort,omitempty"`
	Overview      string            `json:"overview,omitempty"`
	Readme        string            `json:"readme,omitempty"`
	Version       string            `json:"version,omitempty"`
	Icons         []IconSpec        `json:"icons,omitempty"`
	Maintainers   []ContributorSpec `json:"maintainers,omitempty"`
	Owners        []ContributorSpec `json:"owners,omitempty"`
	Company       string            `json:"company,omitempty"`
	Category      string            `json:"category,omitempty"`
	Keywords      []string          `json:"keywords,omitempty"`
	Website       string            `json:"website,omitempty"`
	Source        string            `json:"source,omitempty"`
	License       string            `json:"license,omitempty"`

	// DependsOn is the list of CRDs that this package depends on. This data
	// drives the RBAC generation process.
	DependsOn []PackageInstallSpec `json:"dependsOn,omitempty"`

	// +kubebuilder:validation:Enum=Provider;Package;Application;Addon
	PackageType string `json:"packageType,omitempty"`

	// +kubebuilder:validation:Enum=Cluster;Namespaced
	PermissionScope string `json:"permissionScope,omitempty"`
}

// CRDList is the full list of CRDs that this package owns and depends on
type CRDList []metav1.TypeMeta

// NewCRDList creates a new CRDList with its members initialized.
func NewCRDList() CRDList {
	return []metav1.TypeMeta{}
}

// IconSpec defines the icon for a package
type IconSpec struct {
	Base64IconData string `json:"base64Data"`
	MediaType      string `json:"mediatype"`
}

// ContributorSpec defines a contributor for a package (e.g., maintainer, owner,
// etc.)
type ContributorSpec struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// ControllerSpec defines the controller that implements the logic for a
// package, which can come in different flavors.
type ControllerSpec struct {
	// ServiceAccount options allow for changes to the ServiceAccount the
	// Package Manager creates for the Package's controller
	ServiceAccount *ServiceAccountOptions `json:"serviceAccount,omitempty"`

	Deployment *ControllerDeployment `json:"deployment,omitempty"`
}

// ServiceAccountOptions augment the ServiceAccount created by the Package
// controller
type ServiceAccountOptions struct {
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ControllerDeployment defines a controller for a package that is managed by a
// Deployment.
type ControllerDeployment struct {
	Name string              `json:"name"`
	Spec apps.DeploymentSpec `json:"spec"`
}

// PermissionsSpec defines the permissions that a package will require to
// operate.
type PermissionsSpec struct {
	Rules []rbac.PolicyRule `json:"rules,omitempty"`
}
