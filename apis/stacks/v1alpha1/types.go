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
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

const (
	defaultRegistryPort   = 5000
	defaultRegistryScheme = "https"
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

// StackImage defines the URI properties relevant to Registry Images,
// specifically Stack images
type StackImage struct {
	Scheme string
	Host   string
	Port   int
	Prefix string
	Path   string
	Tag    string
}

// String builds a string URI from a StackImage source and path
func (s *StackImage) String() string {
	b := ""

	// If Host is not set, we ignore Path and Protocol
	if s.Host != "" {
		// omit the default scheme
		if s.Scheme != "" && s.Scheme != defaultRegistryScheme {
			b += s.Scheme + "://"
		}
		b += s.Host
		// omit the default port
		if s.Port != 0 && s.Port != defaultRegistryPort {
			b += ":" + strconv.Itoa(s.Port)
		}

		if prefix := strings.Trim(s.Prefix, "/"); prefix != "" {
			b += "/" + prefix
		}
		b += "/"
	}
	b += s.Path
	if s.Tag != "" {
		b += ":" + s.Tag
	}

	return b
}

// FromSourcePackage populates a StackImage by parsing StackInstaller
// Source and Package
func (s *StackImage) FromSourcePackage(src, pkg string) error {
	if src != "" {
		if !strings.Contains(src, "://") {
			src = defaultRegistryScheme + "://" + src
		}
		u, err := url.Parse(src)
		if err != nil {
			return errors.Wrap(err, "failed to parse source")
		}

		s.Scheme = u.Scheme
		hostPort := strings.SplitN(u.Host, ":", 2)
		s.Host = hostPort[0]
		if len(hostPort) == 2 {
			s.Port, err = strconv.Atoi(hostPort[1])
			if err != nil {
				return errors.Wrap(err, "failed to parse source port")
			}
		}
		s.Prefix = u.Path
	}

	imageWithTag := strings.SplitN(pkg, ":", 2)
	s.Path = imageWithTag[0]
	if len(imageWithTag) == 2 {
		s.Tag = imageWithTag[1]
	}
	return nil
}

// Image returns the fully qualified image name for the StackInstallSpec.
// based on the fully qualified image name format of hostname[:port]/username/reponame[:tag]
func (spec StackInstallSpec) Image() (*StackImage, error) {
	img := &StackImage{}
	if err := img.FromSourcePackage(spec.Source, spec.Package); err != nil {
		return nil, err
	}
	return img, nil
}

// ImageStr calls Image and immediately converts it to string
func (spec StackInstallSpec) ImageStr() (string, error) {
	img, err := spec.Image()
	if err != nil {
		return "", err
	}
	return img.String(), nil
}

// Image gets the Spec.Image of the StackInstall
func (si *StackInstall) Image() (string, error) {
	return si.Spec.ImageStr()
}

// Image gets the Spec.Image of the ClusterStackInstall
func (si *ClusterStackInstall) Image() (string, error) {
	return si.Spec.ImageStr()
}

// GetSpec gets the Spec of the StackInstall
func (si *StackInstall) GetSpec() StackInstallSpec {
	return si.Spec
}

// GetSpec gets the Spec of the ClusterStackInstall
func (si *ClusterStackInstall) GetSpec() StackInstallSpec {
	return si.Spec
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

	Image() (string, error)
	GetSpec() StackInstallSpec
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

	// DependsOn is the list of CRDs that this stack depends on. This data drives the
	// dependency resolution process.
	DependsOn []StackInstallSpec `json:"dependsOn,omitempty"`

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
