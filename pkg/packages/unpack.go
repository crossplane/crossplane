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

package packages

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/packages/walker"
)

const (
	installFileName         = "install.yaml"
	resourceFileNamePattern = "*resource.yaml"
	groupFileName           = "group.yaml"
	appFileName             = "app.yaml"
	behaviorFileName        = "behavior.yaml"

	// StackDefinitionNamespaceEnv is an environment variable used in the
	// StackDefinition controllers deployment to find the StackDefinition
	StackDefinitionNamespaceEnv = "SD_NAMESPACE"

	// StackDefinitionNameEnv is an environment variable used in the
	// StackDefinition controllers deployment to find the StackDefinition
	StackDefinitionNameEnv = "SD_NAME"

	// PackageImageEnv is an environment variable used by the unpack job to select
	// the stack version if there is no version provided in the application
	// metadata.
	PackageImageEnv = "STACK_IMAGE"

	// iconFileNamePattern is the pattern used when walking the stack package and looking for icon files.
	// Icon files that are for a single resource can be prefixed with the kind of the resource, e.g.,
	// mytype.icon.svg
	// Multiple icon types and therefore file extensions are supported: svg, png, jpg, gif
	iconFileNamePattern = "*icon.*"

	crdFileNamePattern      = "*crd.yaml"
	uiSchemaFileNamePattern = "*ui-schema.yaml"
	yamlSeparator           = "\n---\n"

	// Package annotation constants
	annotationPackageIcon           = "packages.crossplane.io/icon-data-uri"
	annotationPackageUISchema       = "packages.crossplane.io/ui-schema"
	annotationPackageTitle          = "packages.crossplane.io/package-title"
	annotationGroupTitle            = "packages.crossplane.io/group-title"
	annotationGroupCategory         = "packages.crossplane.io/group-category"
	annotationGroupReadme           = "packages.crossplane.io/group-readme"
	annotationGroupOverview         = "packages.crossplane.io/group-overview"
	annotationGroupOverviewShort    = "packages.crossplane.io/group-overview-short"
	annotationResourceTitle         = "packages.crossplane.io/resource-title"
	annotationResourceTitlePlural   = "packages.crossplane.io/resource-title-plural"
	annotationResourceCategory      = "packages.crossplane.io/resource-category"
	annotationResourceReadme        = "packages.crossplane.io/resource-readme"
	annotationResourceOverview      = "packages.crossplane.io/resource-overview"
	annotationResourceOverviewShort = "packages.crossplane.io/resource-overview-short"

	// LabelKubernetesManagedBy identifies the resource manager
	LabelKubernetesManagedBy = "app.kubernetes.io/managed-by"

	// LabelValuePackageManager is the Crossplane Package Manager managed-by value
	LabelValuePackageManager = "package-manager"
)

var (
	// PackageCoreRBACRules are the rules that all Package controllers receive
	PackageCoreRBACRules = []rbacv1.PolicyRule{{
		APIGroups:     []string{""},
		ResourceNames: []string{},
		Resources:     []string{"configmaps", "events", "secrets"},
		Verbs:         []string{"*"},
	}}

	// iconFileGlobalNames is the set of supported icon file names at the global level, i.e. not
	// specific to a single resource
	iconFileGlobalNames = []string{"icon.svg", "icon.png", "icon.jpg", "icon.jpeg", "icon.gif"}

	// uiSchemaFileGlobalNames is the set of supported ui schema file names at the global level.
	uiSchemaFileGlobalNames = []string{"ui-schema.yaml"}
)

// PackageResource provides the Package metadata for a CRD. This is the format for resource.yaml files.
type PackageResource struct {
	// ID refers to the CRD Kind
	ID            string `json:"id"`
	Title         string `json:"title"`
	TitlePlural   string `json:"titlePlural"`
	OverviewShort string `json:"overviewShort,omitempty"`
	Overview      string `json:"overview,omitempty"`
	Readme        string `json:"readme,omitempty"`
	Category      string `json:"category"`
}

// PackageGroup provides the Package metadata for a resource group. This is the format for group.yaml files.
type PackageGroup struct {
	Title         string `json:"title"`
	OverviewShort string `json:"overviewShort,omitempty"`
	Overview      string `json:"overview,omitempty"`
	Readme        string `json:"readme,omitempty"`
	Category      string `json:"category"`
}

// PackagePackager implentations can build a package from Package resources and emit the Yaml artifact
type PackagePackager interface {
	SetApp(v1alpha1.AppMetadataSpec)
	SetBehavior(v1alpha1.Behavior)
	SetInstall(unstructured.Unstructured) error
	SetRBAC(v1alpha1.PermissionsSpec)

	GotApp() bool
	IsNamespaced() bool
	GetDefaultTmplCtrlImage() string

	AddGroup(string, PackageGroup)
	AddResource(string, PackageResource)
	AddIcon(string, v1alpha1.IconSpec)
	AddUI(string, string)
	AddCRD(string, *apiextensions.CustomResourceDefinition)

	Yaml() (string, error)
}

// PackagePackage defines the artifact structure of Packages
// A fully processed Package can be thought of as a Package CR and
// a collection of managed CRDs. The Package CR includes its
// controller install and RBAC definitions. The managed CRDS are
// annotated by their Package resource, icon, group, and UI descriptors.
type PackagePackage struct {
	// Package is the Kubernetes API Package representation
	Package v1alpha1.Package

	// StackDefinition is the Kubernetes API StackDefintion representation
	StackDefinition v1alpha1.StackDefinition

	// CRDs map CRD files contained within a Package by their GVK
	CRDs map[string]apiextensions.CustomResourceDefinition
	// TODO(displague) CRD "Version" has been deprecated in favor of multiple "Versions"

	// CRDPaths map CRDs to the path they were found in
	// Package resources will be paired based on their path and the CRD path.
	CRDPaths map[string]string

	// Groups, Icons, Resources, and UISchemas are indexed by the filepath where they were found

	Groups    map[string]PackageGroup
	Icons     map[string]*v1alpha1.IconSpec
	Resources map[string]PackageResource
	UISchemas map[string]string

	// appSet indicates if a App has been assigned through SetApp (for use by GotApp)
	appSet bool

	// behaviorSet indicates if a Behavior has been assigned through SetBehavior (for use by GotBehavior)
	behaviorSet bool

	// baseDir is the directory that serves as the base of the package package (it should be absolute)
	baseDir string

	// defaultTmplCtrlImage is the Template Controller image to handle template packages
	defaultTmplCtrlImage string

	log logging.Logger
}

// GetDefaultTmplCtrlImage returns the default templating controller image path.
func (sp *PackagePackage) GetDefaultTmplCtrlImage() string {
	return sp.defaultTmplCtrlImage
}

// Yaml returns a multiple document YAML representation of the Package Package
// This YAML includes the Package itself and and all CRDs managed by that Package.
func (sp *PackagePackage) Yaml() (string, error) {
	builder := &strings.Builder{}
	builder.WriteString(yamlSeparator)

	// For testing, we want a predictable output order for CRDs
	orderedKeys := orderPackageCRDKeys(sp.CRDs)

	for _, k := range orderedKeys {
		crd := sp.CRDs[k]
		if err := writeYaml(builder, crd, "CRD"); err != nil {
			return "", err
		}
	}

	if sp.GotBehavior() {
		sp.Package.DeepCopyIntoStackDefinition(&sp.StackDefinition)

		// New Format, using 'behavior.yaml'
		if err := writeYaml(builder, sp.StackDefinition, "StackDefinition"); err != nil {
			return "", err
		}
	} else if sp.GotApp() {
		// Old Format, using 'app.yaml'
		if err := writeYaml(builder, sp.Package, "Package"); err != nil {
			return "", err
		}
	}

	return builder.String(), nil
}

// IsNamespaced reports if the PackagePackage is Namespaced (not Cluster Scoped)
func (sp *PackagePackage) IsNamespaced() bool {
	return sp.Package.Spec.PermissionScope == string(apiextensions.NamespaceScoped)
}

// SetApp sets the Package's App metadata
func (sp *PackagePackage) SetApp(app v1alpha1.AppMetadataSpec) {
	app.DeepCopyInto(&sp.Package.Spec.AppMetadataSpec)

	if sp.Package.Spec.AppMetadataSpec.Version == "" {
		iv := os.Getenv(PackageImageEnv)
		sp.log.Debug("No package version found in app metadata; reading version from environment instead", "versionFromEnvironment", iv)

		ref, err := reference.Parse(iv)

		if err != nil {
			sp.log.Debug("Unable to parse image reference. Ignoring image reference.", "imageReference", iv, "err", err)
		} else if tagged, ok := ref.(reference.Tagged); ok {
			sp.Package.Spec.AppMetadataSpec.Version = tagged.Tag()
		} else {
			sp.log.Debug("No tag found on image reference. Ignoring image reference.", "imageReference", iv)
		}
	}

	sp.appSet = true
}

func (sp *PackagePackage) createBehaviorController(sd v1alpha1.Behavior) appsv1.DeploymentSpec {
	controllerContainerName := "package-behavior-manager"

	behaviorDirName := strings.TrimRight(sd.Source.Path, "/")
	behaviorContentsVolumeName := "behaviors"
	behaviorMountPoint := "/behaviors"
	spec := appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{MatchLabels: map[string]string{}},
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyAlways,
				InitContainers: []corev1.Container{
					{
						Name:    "package-behavior-copy-to-manager",
						Image:   sd.Source.Image,
						Command: []string{"cp", "-R", behaviorDirName + "/.", behaviorMountPoint},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      behaviorContentsVolumeName,
								MountPath: behaviorMountPoint,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:  controllerContainerName,
						Image: sd.Engine.ControllerImage,
						// The Command field is omitted and we are relying on
						// the controller's image to explicitly define its
						// entrypoint
						//
						// the environment variables are known and applied to
						// containers at a higher level than unpack
						Args: []string{
							"--resources-dir", behaviorMountPoint,
							"--stack-definition-namespace", "$(" + StackDefinitionNamespaceEnv + ")",
							"--stack-definition-name", "$(" + StackDefinitionNameEnv + ")",
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      behaviorContentsVolumeName,
								MountPath: behaviorMountPoint,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: behaviorContentsVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
	}

	return spec
}

// SetBehavior sets the Package's Behavior
// This is primarily for defining Template Package behaviors
func (sp *PackagePackage) SetBehavior(sd v1alpha1.Behavior) {
	sd.DeepCopyInto(&sp.StackDefinition.Spec.Behavior)

	// Set Package deployment, which StackDefinition YAML will include
	spec := sp.createBehaviorController(sd)
	sp.Package.Spec.Controller.Deployment = &v1alpha1.ControllerDeployment{
		Spec: spec,
	}

	sp.behaviorSet = true
}

// SetInstall sets the Package controller's install method from a Deployment or Job
func (sp *PackagePackage) SetInstall(install unstructured.Unstructured) error {
	if install.GetKind() == "Deployment" {
		deployment := appsv1.Deployment{}
		b, err := install.MarshalJSON()
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &deployment); err != nil {
			return err
		}

		sp.Package.Spec.Controller.Deployment = &v1alpha1.ControllerDeployment{
			Spec: deployment.Spec,
		}
	}
	return nil
}

// SetRBAC sets the PackagePackage Package's permissions with using the supplied PermissionsSpec
func (sp *PackagePackage) SetRBAC(rbac v1alpha1.PermissionsSpec) {
	sp.Package.Spec.Permissions = rbac
}

// GotApp reveals if the AppMetadataSpec has been set
func (sp *PackagePackage) GotApp() bool {
	return sp.appSet
}

// GotBehavior reveals if the BehaviorSpec has been set
func (sp *PackagePackage) GotBehavior() bool {
	return sp.behaviorSet
}

// AddGroup adds a group to the PackagePackage
func (sp *PackagePackage) AddGroup(path string, sg PackageGroup) {
	sp.Groups[path] = sg
}

// AddResource adds a resource to the PackagePackage
func (sp *PackagePackage) AddResource(filepath string, sr PackageResource) {
	sp.Resources[filepath] = sr
}

// AddUI adds a resource to the PackagePackage
func (sp *PackagePackage) AddUI(filepath string, ui string) {
	sp.UISchemas[filepath] = ui
}

// AddIcon adds an icon to the PackagePackage
func (sp *PackagePackage) AddIcon(path string, icon v1alpha1.IconSpec) {
	// only store top-level icons in the package spec
	if filepath.Dir(path) == sp.baseDir {
		// TODO(displague) do we want to keep all top-level icons in the Package spec or just the preferred type?
		sp.Package.Spec.AppMetadataSpec.Icons = append(sp.Package.Spec.AppMetadataSpec.Icons, icon)
	}

	// highest priority wins per path
	iconMimePriority := map[string]int{"image/svg+xml": 4, "image/png": 3, "image/jpeg": 2, "image/gif": 1}
	if existing, found := sp.Icons[filepath.Dir(path)]; found {
		if iconMimePriority[existing.MediaType] > iconMimePriority[icon.MediaType] {
			return
		}
	}

	sp.Icons[path] = &icon
}

// AddCRD appends a CRD to the CRDs of this PackagePackage
// The CRD will be annotated before being added and the Package will track ownership of this CRD.
func (sp *PackagePackage) AddCRD(path string, crd *apiextensions.CustomResourceDefinition) {
	if crd.ObjectMeta.Labels == nil {
		crd.ObjectMeta.Labels = map[string]string{}
	}
	if crd.ObjectMeta.Annotations == nil {
		crd.ObjectMeta.Annotations = map[string]string{}
	}
	crd.ObjectMeta.Labels[LabelKubernetesManagedBy] = LabelValuePackageManager

	if sp.IsNamespaced() {
		crd.ObjectMeta.Labels[LabelScope] = NamespaceScoped
	} else {
		crd.ObjectMeta.Labels[LabelScope] = EnvironmentScoped
	}

	crdGVK := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crd.Spec.Version,
		Kind:    crd.Spec.Names.Kind,
	}

	// TODO(displague) store crd and path in a single struct
	sp.CRDs[crdGVK.String()] = *crd
	sp.CRDPaths[crdGVK.String()] = path

	crdTypeMeta := metav1.TypeMeta{
		Kind:       crdGVK.Kind,
		APIVersion: crdGVK.GroupVersion().String(),
	}

	sp.Package.Spec.CRDs = append(sp.Package.Spec.CRDs, crdTypeMeta)

}

// applyAnnotations walks each discovered CRD annotates that CRD with the nearest metadata file
func (sp *PackagePackage) applyAnnotations() {
	for gvk, crdPath := range sp.CRDPaths {
		crd := sp.CRDs[gvk]

		crd.ObjectMeta.Annotations[annotationPackageTitle] = sp.Package.Spec.AppMetadataSpec.Title

		sp.applyGroupAnnotations(crdPath, &crd)
		sp.applyIconAnnotations(crdPath, &crd)
		sp.applyResourceAnnotations(crdPath, &crd)
		sp.applyUISchemaAnnotations(crdPath, &crd)

	}
}

// generateRBAC generates a RBAC policy rule for the given kind and group.
// Note that apiGroup should not contain a version, only the group, e.g., database.crossplane.io
// RBAC policy rules are intended to be versionless.
func generateRBAC(apiKinds []string, apiGroup string, verbs []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		APIGroups:     []string{apiGroup},
		ResourceNames: []string{},
		Resources:     apiKinds,
		Verbs:         verbs,
	}
}

// applyRules adds RBAC rules to the Package for standard Package needs and to fulfill dependencies
func (sp *PackagePackage) applyRules() (v1alpha1.PermissionsSpec, error) {
	// standard rules that all Packages get
	rbac := v1alpha1.PermissionsSpec{Rules: PackageCoreRBACRules}
	allVerbs := []string{"*"}

	// owned CRD rules
	orderedKeys := orderPackageCRDKeys(sp.CRDs)
	for _, k := range orderedKeys {
		crd := sp.CRDs[k]
		kinds := []string{crd.Spec.Names.Plural}
		verbs := allVerbs
		if subs := crd.Spec.Subresources; subs != nil {
			if subs.Status != nil {
				kinds = append(kinds, crd.Spec.Names.Plural+"/status")
			}
			if subs.Scale != nil {
				kinds = append(kinds, crd.Spec.Names.Plural+"/scale")
			}
		}

		// For the package controller to set a controller owner reference on CRs
		// that it owns, in some settings (OpenShift 4.3), it is necessary to
		// give the controller access to a finalizers subresource, even if the
		// crd does not present this subresource. External controllers will look
		// for this rule. The error produced without it is: "cannot set
		// blockOwnerDeletion if an ownerReference refers to a resource you
		// can't set finalizers on"
		kinds = append(kinds, crd.Spec.Names.Plural+"/finalizers")

		rule := generateRBAC(kinds, crd.Spec.Group, verbs)
		rbac.Rules = append(rbac.Rules, rule)
	}

	// dependency based rules
	for _, dependency := range sp.Package.Spec.DependsOn {
		crd := dependency.CustomResourceDefinition
		if crd != "" {
			// versions are not allowed in RBAC PolicyRules, remove any trailing version denoted by a "/"
			// e.g., kind.group.com/v1alpha1 -> kind.group.com
			if i := strings.Index(crd, "/"); i != -1 {
				crd = crd[:i]
			}

			gk := schema.ParseGroupKind(crd)
			if gk.Kind == "" {
				return v1alpha1.PermissionsSpec{}, errors.New(fmt.Sprintf("cannot parse CustomResourceDefinition %q as Kind and Group", crd))
			}
			rule := generateRBAC([]string{gk.Kind}, gk.Group, allVerbs)
			rbac.Rules = append(rbac.Rules, rule)
		}
	}

	return rbac, nil
}

// NewPackagePackage returns a PackagePackage with maps created
func NewPackagePackage(baseDir, tmplCtrlImage string, log logging.Logger) *PackagePackage {
	// create a Package record and populate it with the relevant package contents
	sv, sk := v1alpha1.PackageGroupVersionKind.ToAPIVersionAndKind()
	sdv, sdk := v1alpha1.StackDefinitionGroupVersionKind.ToAPIVersionAndKind()

	sp := &PackagePackage{
		Package: v1alpha1.Package{
			TypeMeta: metav1.TypeMeta{APIVersion: sv, Kind: sk},
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
		},
		StackDefinition: v1alpha1.StackDefinition{
			TypeMeta:   metav1.TypeMeta{APIVersion: sdv, Kind: sdk},
			ObjectMeta: metav1.ObjectMeta{},
		},
		CRDs:                 map[string]apiextensions.CustomResourceDefinition{},
		CRDPaths:             map[string]string{},
		Groups:               map[string]PackageGroup{},
		Icons:                map[string]*v1alpha1.IconSpec{},
		Resources:            map[string]PackageResource{},
		UISchemas:            map[string]string{},
		baseDir:              baseDir,
		defaultTmplCtrlImage: tmplCtrlImage,
		log:                  log,
	}

	return sp
}

// Unpack writes to `out` using custom Step functions against a ResourceWalker
// The custom Steps process Package resource files and the output is multiple
// YAML documents.  CRDs container within the package will be annotated based
// on the other Package resource files contained within the Package.
//
// baseDir is expected to be an absolute path, i.e. have a root to the path,
// at the very least "/".
func Unpack(rw walker.ResourceWalker, out io.StringWriter, baseDir, permissionScope string, tsControllerImage string, log logging.Logger) error {
	l := log.WithValues("operation", "unpack")
	sp := NewPackagePackage(filepath.Clean(baseDir), tsControllerImage, l)

	rw.AddStep(appFileName, appStep(sp))
	rw.AddStep(behaviorFileName, behaviorStep(sp))
	rw.AddStep(groupFileName, groupStep(sp))

	rw.AddStep(resourceFileNamePattern, resourceStep(sp))
	rw.AddStep(crdFileNamePattern, crdStep(sp))
	rw.AddStep(installFileName, installStep(sp))
	rw.AddStep(iconFileNamePattern, iconStep(sp))
	rw.AddStep(uiSchemaFileNamePattern, uiStep(sp))

	if err := rw.Walk(); err != nil {
		return errors.Wrap(err, "failed to walk Package filesystem")
	}

	if !sp.GotApp() {
		return errors.New("Package does not contain an app.yaml file")
	}

	if sp.Package.Spec.PermissionScope != permissionScope {
		return errors.New(fmt.Sprintf("Package permissionScope %q is not permitted by unpack invocation parameters (expected %q)", sp.Package.Spec.PermissionScope, permissionScope))
	}

	perms, err := sp.applyRules()
	if err != nil {
		return err
	}
	sp.SetRBAC(perms)

	sp.applyAnnotations()

	yaml, err := sp.Yaml()

	if err == nil {
		_, err = out.WriteString(yaml)
	}

	return err
}

// orderPackageCRDKeys returns the map indexes in descending order
func orderPackageCRDKeys(m map[string]apiextensions.CustomResourceDefinition) []string {
	ret := make([]string, len(m))
	i := 0

	for k := range m {
		ret[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret
}

// orderPackageGroupKeys returns the map indexes in descending order (/foo/bar/baz, /foo/bar, /foo, /bar)
func orderPackageGroupKeys(m map[string]PackageGroup) []string {
	ret := make([]string, len(m))
	i := 0

	for k := range m {
		ret[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret
}

// orderPackageIconKeys returns the map indexes in descending order (/foo/bar/baz, /foo/bar, /foo, /bar)
// TODO(displague) this is identical to orderPackageGroupKeys. generics?
func orderPackageIconKeys(m map[string]*v1alpha1.IconSpec) []string {
	ret := make([]string, len(m))
	i := 0

	for k := range m {
		ret[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret
}

// orderPackageResourceKeys returns the map indexes in descending order (/foo/bar/baz, /foo/bar, /foo, /bar)
// TODO(displague) this is identical to orderPackageGroupKeys. generics?
func orderPackageResourceKeys(m map[string]PackageResource) []string {
	ret := make([]string, len(m))
	i := 0

	for k := range m {
		ret[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret
}

// orderStringKeys returns the map indexes in descending order (/foo/bar/baz, /foo/bar, /foo, /bar)
// TODO(displague) this is identical to orderPackageGroupKeys. generics?
func orderStringKeys(m map[string]string) []string {
	ret := make([]string, len(m))
	i := 0

	for k := range m {
		ret[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret
}

func (sp *PackagePackage) applyGroupAnnotations(crdPath string, crd *apiextensions.CustomResourceDefinition) {
	// A group among many CRDs applies to all CRDs
	groupPathsOrdered := orderPackageGroupKeys(sp.Groups)
	for _, groupPath := range groupPathsOrdered {
		group := sp.Groups[groupPath]
		if strings.HasPrefix(crdPath, groupPath) {
			crd.ObjectMeta.Annotations[annotationGroupTitle] = group.Title
			crd.ObjectMeta.Annotations[annotationGroupCategory] = group.Category
			crd.ObjectMeta.Annotations[annotationGroupReadme] = group.Readme
			crd.ObjectMeta.Annotations[annotationGroupOverview] = group.Overview
			crd.ObjectMeta.Annotations[annotationGroupOverviewShort] = group.OverviewShort
			break
		}
	}

}

// applyResourceAnnotations annotates resource.yaml properties to the appropriate PackagePackage CRD
// A resource.yaml must reside in the same path or higher than the CRD and must contain an id matching
// the CRD kind
func (sp *PackagePackage) applyResourceAnnotations(crdPath string, crd *apiextensions.CustomResourceDefinition) {
	// TODO(displague) which pattern should associate *resource.yaml to their matching *crd.yaml files?
	// * resource.yaml contain "id=_kind_" (or gvk)
	// * limit one-crd per path
	// * file names match their CRD: [_group_]/[_kind_.[_version_.]]{resource,crd}.yaml
	resourcePathsOrdered := orderPackageResourceKeys(sp.Resources)
	for _, resourcePath := range resourcePathsOrdered {
		dir := filepath.Dir(resourcePath)
		resource := sp.Resources[resourcePath]
		if strings.HasPrefix(crdPath, dir) && strings.EqualFold(resource.ID, crd.Spec.Names.Kind) {
			crd.ObjectMeta.Annotations[annotationResourceTitle] = resource.Title
			crd.ObjectMeta.Annotations[annotationResourceTitlePlural] = resource.TitlePlural
			crd.ObjectMeta.Annotations[annotationResourceCategory] = resource.Category
			crd.ObjectMeta.Annotations[annotationResourceReadme] = resource.Readme
			crd.ObjectMeta.Annotations[annotationResourceOverview] = resource.Overview
			crd.ObjectMeta.Annotations[annotationResourceOverviewShort] = resource.OverviewShort

			break
		}
	}
}

// applyIconAnnotations annotates icon data to the appropriate PackagePackage CRDs
// An icon among many CRDs applies to all CRDs. Only the nearest ancestor icon is applied to CRDs.
func (sp *PackagePackage) applyIconAnnotations(crdPath string, crd *apiextensions.CustomResourceDefinition) {
	iconPathsOrdered := orderPackageIconKeys(sp.Icons)
	for _, iconPath := range iconPathsOrdered {
		if isMetadataApplicableToCRD(crdPath, iconPath, iconFileGlobalNames, crd.Spec.Names.Kind) {
			// the current icon file is applicable to the given CRD, apply the icon to the CRD now
			// and then break from the loop since we do not apply more than one icon per resource
			icon := sp.Icons[iconPath]
			crd.ObjectMeta.Annotations[annotationPackageIcon] = "data:" + icon.MediaType + ";base64," + icon.Base64IconData
			break
		}
	}
}

// applyUISchemaAnnotations annotates ui-schema.yaml contents to the appropriate PackagePackage CRDs
// Existing ui-schema annotation values are preserved. All existing and matching ui-schema.yaml files
// will be concatenated as a multiple document YAML.
// A ui-schema.yaml among many CRDs applies to all neighboring and descendent CRDs,
// a _kind_.ui-schema.yaml applies to crds with a matching kind
func (sp *PackagePackage) applyUISchemaAnnotations(crdPath string, crd *apiextensions.CustomResourceDefinition) {
	uiPathsOrdered := orderStringKeys(sp.UISchemas)
	for _, uiSchemaPath := range uiPathsOrdered {
		if isMetadataApplicableToCRD(crdPath, uiSchemaPath, uiSchemaFileGlobalNames, crd.Spec.Names.Kind) {
			// the current UI schema file is applicable to the given CRD, apply its spec content to the CRD now
			schema := strings.Trim(sp.UISchemas[uiSchemaPath], "\n")

			// TODO(displague) are there concerns about the concatenation order of ui-schema.yaml and kind.ui-schema.yaml?
			if len(crd.ObjectMeta.Annotations[annotationPackageUISchema]) > 0 {
				appendedUI := fmt.Sprintf("%s\n---\n%s", crd.ObjectMeta.Annotations[annotationPackageUISchema], schema)
				crd.ObjectMeta.Annotations[annotationPackageUISchema] = appendedUI
			} else {
				crd.ObjectMeta.Annotations[annotationPackageUISchema] = schema
			}
		}
	}
}

// isMetadataApplicableToCRD determines if the given metadata file path is applicable to the given CRD.
func isMetadataApplicableToCRD(crdPath, metadataPath string, globalFileNames []string, crdKind string) bool {
	// compare the directory of the given metadata file path to the CRDs path
	metadataDir := filepath.Dir(metadataPath)
	if !strings.HasPrefix(crdPath, metadataDir) {
		// the CRD is not in the same directory (or a child directory) that the metadata file
		// path is, the metadata is not applicable to this CRD
		return false
	}

	// get the file name of the metadata file path, e.g. /a/b/icon.svg => icon.svg
	metadataBasename := filepath.Base(metadataPath)

	for _, g := range globalFileNames {
		if metadataBasename == g {
			// the metadata file exactly matches one of the allowed global file names,
			// this metadata file is applicable to the given CRD
			return true
		}
	}

	// check to see if the metadata file name starts with the kind of the given CRD, if it does
	// then we consider that a match.  e.g. mytype.icon.svg is applicable to a CRD with kind mytype
	return strings.HasPrefix(metadataBasename, strings.ToLower(crdKind)+".")
}

// writeYaml writes the supplied object as Yaml with a separator
func writeYaml(w io.Writer, o interface{}, hint string) error {
	b, err := yaml.Marshal(o)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("could not marshal %s", hint))
	}
	if _, err := w.Write(append(b, []byte(yamlSeparator)...)); err != nil {
		return errors.Wrap(err, "could not write YAML output to buffer")
	}
	return nil
}
