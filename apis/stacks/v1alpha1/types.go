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

// TODO: how do we pretty print conditioned status items? There may be multiple of them, and they
// can have varying status (e.g., True, False, Unknown)

// +kubebuilder:object:root=true

// A StackInstall requests a stack be installed to Crossplane.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditionedStatus.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SOURCE",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="PACKAGE",type="string",JSONPath=".spec.package"
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".spec.crd"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type StackInstall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackInstallSpec   `json:"spec,omitempty"`
	Status StackInstallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StackInstallList contains a list of StackInstall.
type StackInstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StackInstall `json:"items"`
}

// StackInstallSpec specifies details about a request to install a stack to
// Crossplane.
type StackInstallSpec struct {
	// Source is the domain name for the stack registry hosting the stack being requested,
	// e.g., registry.crossplane.io
	Source string `json:"source,omitempty"`

	// Package is the name of the stack package that is being requested, e.g., myapp.
	// Either Package or CustomResourceDefinition can be specified.
	Package string `json:"package,omitempty"`

	// CustomResourceDefinition is the full name of a CRD that is owned by the stack being
	// requested. This can be a convenient way of installing a stack when the desired
	// CRD is known, but the package name that contains it is not known.
	// Either Package or CustomResourceDefinition can be specified.
	CustomResourceDefinition string `json:"crd,omitempty"`
}

// StackInstallStatus represents the observed state of a StackInstall.
type StackInstallStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`

	InstallJob  *corev1.ObjectReference `json:"installJob,omitempty"`
	StackRecord *corev1.ObjectReference `json:"stackRecord,omitempty"`
}

// Image returns the Package prefixed with a source (if available).
// If the package format is not understood it is returned unmodified to be
// handled by Kubernetes.
func (spec StackInstallSpec) Image() string {
	image, err := spec.ImageWithSource(spec.Package)
	if err != nil {
		return spec.Package
	}
	return image
}

// ImageWithSource applies a source to a container image URI only if the image
// does not appear to contain a source. Source is some combination of scheme,
// host, port, and prefix where host is required.
func (spec StackInstallSpec) ImageWithSource(image string) (string, error) {
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
	// want to apply the StackInstall source.
	domain := reference.Domain(named)
	if strings.Index(image, domain) != 0 {
		return strings.Trim(spec.Source, "/") + "/" + image, nil
	}

	// image contained a source
	return image, nil
}

// GetPackage gets the Spec of the StackInstall
func (si *StackInstall) GetPackage() string {
	return si.Spec.Package
}

// GetPackage gets the Spec of the ClusterStackInstall
func (si *ClusterStackInstall) GetPackage() string {
	return si.Spec.Package
}

// SetSource sets the Source of the StackInstall Spec
func (si *StackInstall) SetSource(src string) {
	si.Spec.Source = src
}

// SetSource sets the Source of the ClusterStackInstall Spec
func (si *ClusterStackInstall) SetSource(src string) {
	si.Spec.Source = src
}

// ImageWithSource modifies the supplied image with the source of the StackInstall
func (si *StackInstall) ImageWithSource(img string) (string, error) {
	return si.Spec.ImageWithSource(img)
}

// ImageWithSource modifies the supplied image with the source of the ClusterStackInstall
func (si *ClusterStackInstall) ImageWithSource(img string) (string, error) {
	return si.Spec.ImageWithSource(img)
}

// PermissionScope gets the required app.yaml permissionScope value ("Namespaced") for StackInstall
func (si *StackInstall) PermissionScope() string { return string(apiextensions.NamespaceScoped) }

// PermissionScope gets the required app.yaml permissionScope value ("Cluster") for ClusterStackInstall
func (si *ClusterStackInstall) PermissionScope() string { return string(apiextensions.ClusterScoped) }

// SetConditions sets the StackInstall's Status conditions
func (si *StackInstall) SetConditions(c ...runtimev1alpha1.Condition) {
	si.Status.SetConditions(c...)
}

// SetConditions sets the ClusterStackInstall's Status conditions
func (si *ClusterStackInstall) SetConditions(c ...runtimev1alpha1.Condition) {
	si.Status.SetConditions(c...)
}

// InstallJob gets the ClusterStackInstall's Status InstallJob
func (si *ClusterStackInstall) InstallJob() *corev1.ObjectReference {
	return si.Status.InstallJob
}

// InstallJob gets the StackInstall's Status InstallJob
func (si *StackInstall) InstallJob() *corev1.ObjectReference {
	return si.Status.InstallJob
}

// SetInstallJob sets the ClusterStackInstall's Status InstallJob
func (si *ClusterStackInstall) SetInstallJob(job *corev1.ObjectReference) {
	si.Status.InstallJob = job
}

// SetInstallJob sets the StackInstall's Status InstallJob
func (si *StackInstall) SetInstallJob(job *corev1.ObjectReference) {
	si.Status.InstallJob = job
}

// StackRecord gets the ClusterStackInstall's Status StackRecord
func (si *ClusterStackInstall) StackRecord() *corev1.ObjectReference {
	return si.Status.StackRecord
}

// SetStackRecord sets the ClusterStackInstall's Status StackRecord
func (si *ClusterStackInstall) SetStackRecord(job *corev1.ObjectReference) {
	si.Status.StackRecord = job
}

// SetStackRecord sets the StackInstall's Status StackRecord
func (si *StackInstall) SetStackRecord(job *corev1.ObjectReference) {
	si.Status.StackRecord = job
}

// StackRecord gets the StackInstall's Status StackRecord
func (si *StackInstall) StackRecord() *corev1.ObjectReference {
	return si.Status.StackRecord
}

// GroupVersionKind gets the GroupVersionKind of the StackInstall
func (si *StackInstall) GroupVersionKind() schema.GroupVersionKind {
	return StackInstallGroupVersionKind
}

// GroupVersionKind gets the GroupVersionKind of the ClusterStackInstall
func (si *ClusterStackInstall) GroupVersionKind() schema.GroupVersionKind {
	return ClusterStackInstallGroupVersionKind
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:generate=false

// StackInstaller provides a common interface for StackInstall and ClusterStackInstall to share controller and reconciler logic
type StackInstaller interface {
	metav1.Object
	runtime.Object

	GetPackage() string
	SetSource(string)
	ImageWithSource(string) (string, error)
	PermissionScope() string
	SetConditions(c ...runtimev1alpha1.Condition)
	InstallJob() *corev1.ObjectReference
	SetInstallJob(*corev1.ObjectReference)
	StackRecord() *corev1.ObjectReference
	SetStackRecord(*corev1.ObjectReference)
	GroupVersionKind() schema.GroupVersionKind
}

// +kubebuilder:object:root=true

// ClusterStackInstall is the CRD type for a request to add a stack to Crossplane.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditionedStatus.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SOURCE",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="PACKAGE",type="string",JSONPath=".spec.package"
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".spec.crd"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterStackInstall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackInstallSpec   `json:"spec,omitempty"`
	Status StackInstallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterStackInstallList contains a list of StackInstall
type ClusterStackInstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterStackInstall `json:"items"`
}

// +kubebuilder:object:root=true

// A Stack that has been added to Crossplane.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditionedStatus.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Stack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackSpec   `json:"spec,omitempty"`
	Status StackStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StackList contains a list of Stack.
type StackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Stack `json:"items"`
}

// StackSpec specifies the desired state of a Stack.
type StackSpec struct {
	AppMetadataSpec `json:",inline"`
	CRDs            CRDList         `json:"customresourcedefinitions,omitempty"`
	Controller      ControllerSpec  `json:"controller,omitempty"`
	Permissions     PermissionsSpec `json:"permissions,omitempty"`
}

// StackStatus represents the observed state of a Stack.
type StackStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`
	ControllerRef                     *corev1.ObjectReference `json:"controllerRef,omitempty"`
}

// PackageMetadataSpec defines metadata about the stack application
// and package contents
type PackageMetadataSpec struct {
	APIVersion string `json:"apiVersion,omitempty"`

	AppMetadataSpec `json:",inline"`
}

// AppMetadataSpec defines metadata about the stack application
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

	// DependsOn is the list of CRDs that this stack depends on. This data
	// drives the RBAC generation process.
	DependsOn []StackInstallSpec `json:"dependsOn,omitempty"`

	// +kubebuilder:validation:Enum=Provider;Stack;Application
	PackageType string `json:"packageType,omitempty"`

	// +kubebuilder:validation:Enum=Cluster;Namespaced
	PermissionScope string `json:"permissionScope,omitempty"`
}

// CRDList is the full list of CRDs that this stack owns and depends on
type CRDList []metav1.TypeMeta

// NewCRDList creates a new CRDList with its members initialized.
func NewCRDList() CRDList {
	return []metav1.TypeMeta{}
}

// IconSpec defines the icon for a stack
type IconSpec struct {
	Base64IconData string `json:"base64Data"`
	MediaType      string `json:"mediatype"`
}

// ContributorSpec defines a contributor for a stack (e.g., maintainer, owner, etc.)
type ContributorSpec struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// ControllerSpec defines the controller that implements the logic for a stack, which can come
// in different flavors. A golang code (controller-runtime) controller with a managing Deployment
// is all that is supported currently, but more types will come in the future (e.g., templates,
// functions/hooks, templates, a new DSL, etc.
type ControllerSpec struct {
	Deployment *ControllerDeployment `json:"deployment,omitempty"`
}

// ControllerDeployment defines a controller for a stack that is managed by a Deployment.
type ControllerDeployment struct {
	Name string              `json:"name"`
	Spec apps.DeploymentSpec `json:"spec"`
}

// PermissionsSpec defines the permissions that a stack will require to operate.
type PermissionsSpec struct {
	Rules []rbac.PolicyRule `json:"rules,omitempty"`
}
