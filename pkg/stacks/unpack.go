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

package stacks

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/stacks/walker"
)

const (
	installFileName         = "install.yaml"
	resourceFileNamePattern = "*resource.yaml"
	groupFileName           = "group.yaml"
	appFileName             = "app.yaml"
	permissionsFileName     = "rbac.yaml"
	iconFileNamePattern     = "icon.*"
	crdFileNamePattern      = "*crd.yaml"
	uiSpecFileNamePattern   = "*ui-schema.yaml"
	uiSpecFileName          = "ui-schema.yaml" // global ui schema filename
	yamlSeparator           = "\n---\n"

	// Stack annotation constants
	annotationStackIcon           = "stacks.crossplane.io/icon"
	annotationStackUISpec         = "stacks.crossplane.io/ui-spec"
	annotationStackTitle          = "stacks.crossplane.io/stack-title"
	annotationGroupTitle          = "stacks.crossplane.io/group-title"
	annotationGroupCategory       = "stacks.crossplane.io/group-category"
	annotationGroupDescription    = "stacks.crossplane.io/group-description"
	annotationResourceTitle       = "stacks.crossplane.io/resource-title"
	annotationResourceTitlePlural = "stacks.crossplane.io/resource-title-plural"
	annotationResourceCategory    = "stacks.crossplane.io/resource-category"
	annotationResourceDescription = "stacks.crossplane.io/resource-description"
	annotationKubernetesManagedBy = "app.kubernetes.io/managed-by"
)

var (
	log = logging.Logger.WithName("stacks")
)

// StackResource provides the Stack metadata for a CRD. This is the format for resource.yaml files.
type StackResource struct {
	// ID refers to the CRD Kind
	ID          string `json:"id"`
	Title       string `json:"title"`
	TitlePlural string `json:"title-plural"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// StackGroup provides the Stack metadata for a resource group. This is the format for group.yaml files.
type StackGroup struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// StackPackager implentations can build a stack from Stack resources and emit the Yaml artifact
type StackPackager interface {
	SetApp(v1alpha1.AppMetadataSpec)
	SetInstall(unstructured.Unstructured) error
	SetRBAC(v1alpha1.PermissionsSpec)

	GotApp() bool

	AddGroup(string, StackGroup)
	AddResource(string, StackResource)
	AddIcon(string, v1alpha1.IconSpec)
	AddUI(string, string)
	AddCRD(string, *apiextensions.CustomResourceDefinition)

	Yaml() (string, error)
}

// StackPackage defines the artifact structure of Stacks
// A fully processed Stack can be thought of as a Stack CR and
// a collection of managed CRDs.  The Stack CR includes its
// controller install and RBAC definitions. The managed CRDS are
// annotated by their Stack resource, icon, group, and UI descriptors.
type StackPackage struct {
	// Stack is the Kubernetes API Stack representation
	Stack v1alpha1.Stack

	// CRDs map CRD files contained within a Stack by their GVK
	CRDs map[string]apiextensions.CustomResourceDefinition
	// TODO(displague) CRD "Version" has been deprecated in favor of multiple "Versions"

	// CRDPaths map CRDs to the path they were found in
	// Stack resources will be paired based on their path and the CRD path.
	CRDPaths map[string]string

	// Groups, Icons, Resources, and UISpecs are indexed by the filepath where they were found

	Groups    map[string]StackGroup
	Icons     map[string]*v1alpha1.IconSpec
	Resources map[string]StackResource
	UISpecs   map[string]string

	// appSet indicates if a App has been assigned through SetApp (for use by GotApp)
	appSet bool
}

// Yaml returns a multiple document YAML representation of the Stack Package
// This YAML includes the Stack itself and and all CRDs managed by that Stack.
func (sp *StackPackage) Yaml() (string, error) {
	var builder strings.Builder

	builder.WriteString(yamlSeparator)

	// For testing, we want a predictable output order for CRDs
	orderedKeys := orderStackCRDKeys(sp.CRDs)

	for _, k := range orderedKeys {
		crd := sp.CRDs[k]
		b, err := yaml.Marshal(crd)
		if err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("could not marshal CRD (%s)", crd.GroupVersionKind()))
		}
		builder.Write(b)
		builder.WriteString(yamlSeparator)
	}

	b, err := yaml.Marshal(sp.Stack)
	if err != nil {
		return "", errors.Wrap(err, "could not marshal Stack")
	}

	if _, err := builder.Write(b); err != nil {
		if err != nil {
			return "", errors.Wrap(err, "could not write YAML output to buffer")
		}
	}

	return builder.String(), nil
}

// SetApp sets the Stack's App metadata
func (sp *StackPackage) SetApp(app v1alpha1.AppMetadataSpec) {
	sp.Stack.Spec.AppMetadataSpec = app
	sp.appSet = true
}

// SetInstall sets the Stack controller's install method from a Deployment or Job
func (sp *StackPackage) SetInstall(install unstructured.Unstructured) error {
	switch install.GetKind() {
	case "Deployment":
		deployment := appsv1.Deployment{}
		b, err := install.MarshalJSON()
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &deployment); err != nil {
			return err
		}

		sp.Stack.Spec.Controller.Deployment = &v1alpha1.ControllerDeployment{
			Name: install.GetName(),
			Spec: deployment.Spec,
		}
	case "Job":
		job := batchv1.Job{}
		b, err := install.MarshalJSON()
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &job); err != nil {
			return err
		}

		sp.Stack.Spec.Controller.Job = &v1alpha1.ControllerJob{
			Name: install.GetName(),
			Spec: job.Spec,
		}
	}
	return nil
}

// SetRBAC sets the StackPackage Stack's permissions with using the supplied PermissionsSpec
func (sp *StackPackage) SetRBAC(rbac v1alpha1.PermissionsSpec) {
	sp.Stack.Spec.Permissions = rbac
}

// GotApp reveals if the AppMetadataSpec has been set
func (sp *StackPackage) GotApp() bool {
	return sp.appSet
}

// AddGroup adds a group to the StackPackage
func (sp *StackPackage) AddGroup(path string, sg StackGroup) {
	sp.Groups[path] = sg
}

// AddResource adds a resource to the StackPackage
func (sp *StackPackage) AddResource(filepath string, sr StackResource) {
	sp.Resources[filepath] = sr
}

// AddUI adds a resource to the StackPackage
func (sp *StackPackage) AddUI(filepath string, ui string) {
	sp.UISpecs[filepath] = ui
}

// AddIcon adds an icon to the StackPackage
func (sp *StackPackage) AddIcon(path string, icon v1alpha1.IconSpec) {
	// TODO(displague) do we want to keep all icons in the Stack spec or just the preferred type?
	sp.Stack.Spec.AppMetadataSpec.Icons = append(sp.Stack.Spec.AppMetadataSpec.Icons, icon)

	// highest priority wins per path
	iconMimePriority := map[string]int{"image/svg+xml": 4, "image/png": 3, "image/jpeg": 2, "image/gif": 1}
	if existing, found := sp.Icons[filepath.Dir(path)]; found {
		if iconMimePriority[existing.MediaType] > iconMimePriority[icon.MediaType] {
			return
		}
	}

	sp.Icons[filepath.Dir(path)] = &icon
}

// AddCRD appends a CRD to the CRDs of this StackPackage
// The CRD will be annotated before being added and the Stack will track ownership of this CRD.
func (sp *StackPackage) AddCRD(path string, crd *apiextensions.CustomResourceDefinition) {
	if crd.ObjectMeta.Annotations == nil {
		crd.ObjectMeta.Annotations = map[string]string{}
	}
	crd.ObjectMeta.Annotations[annotationKubernetesManagedBy] = "stack-manager"

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

	sp.Stack.Spec.CRDs.Owned = append(sp.Stack.Spec.CRDs.Owned, crdTypeMeta)

}

// applyAnnotations walks each discovered CRD annotates that CRD with the nearest metadata file
func (sp *StackPackage) applyAnnotations() {
	for gvk, crdPath := range sp.CRDPaths {
		crd := sp.CRDs[gvk]

		crd.ObjectMeta.Annotations[annotationStackTitle] = sp.Stack.Spec.AppMetadataSpec.Title

		sp.applyGroupAnnotations(crdPath, &crd)
		sp.applyIconAnnotations(crdPath, &crd)
		sp.applyResourceAnnotations(crdPath, &crd)
		sp.applyUISpecAnnotations(crdPath, &crd)

	}
}

// NewStackPackage returns a StackPackage with maps created
func NewStackPackage() *StackPackage {
	// create a Stack record and populate it with the relevant package contents
	v, k := v1alpha1.StackGroupVersionKind.ToAPIVersionAndKind()

	sp := &StackPackage{
		Stack: v1alpha1.Stack{
			TypeMeta: metav1.TypeMeta{APIVersion: v, Kind: k},
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
		},
		CRDs:      map[string]apiextensions.CustomResourceDefinition{},
		CRDPaths:  map[string]string{},
		Groups:    map[string]StackGroup{},
		Icons:     map[string]*v1alpha1.IconSpec{},
		Resources: map[string]StackResource{},
		UISpecs:   map[string]string{},
	}

	return sp
}

// Unpack writes to `out` using custom Step functions against a ResourceWalker
// The custom Steps process Stack resource files and the output is multiple
// YAML documents.  CRDs container within the stack will be annotated based
// on the other Stack resource files contained within the Stack.
func Unpack(rw walker.ResourceWalker, out io.StringWriter, permissionScope string) error {
	log.V(logging.Debug).Info("Unpacking stack")

	sp := NewStackPackage()

	rw.AddStep(appFileName, appStep(sp))

	rw.AddStep(groupFileName, groupStep(sp))

	rw.AddStep(resourceFileNamePattern, resourceStep(sp))
	rw.AddStep(crdFileNamePattern, crdStep(sp))
	rw.AddStep(permissionsFileName, rbacStep(sp))
	rw.AddStep(installFileName, installStep(sp))
	rw.AddStep(iconFileNamePattern, iconStep(sp))
	rw.AddStep(uiSpecFileNamePattern, uiStep(sp))

	if err := rw.Walk(); err != nil {
		return errors.Wrap(err, "failed to walk Stack filesystem")
	}

	if !sp.GotApp() {
		return errors.New("Stack does not contain an app.yaml file")
	}

	if sp.Stack.Spec.AppMetadataSpec.PermissionScope != permissionScope {
		return errors.New(fmt.Sprintf("Stack permissionScope %q is not permitted by unpack invocation parameters", permissionScope))
	}

	sp.applyAnnotations()

	yaml, err := sp.Yaml()

	if err == nil {
		_, err = out.WriteString(yaml)
	}

	return err
}

// orderStackCRDKeys returns the map indexes in descending order
func orderStackCRDKeys(m map[string]apiextensions.CustomResourceDefinition) []string {
	ret := make([]string, len(m))
	i := 0

	for k := range m {
		ret[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret
}

// orderStackGroupKeys returns the map indexes in descending order (/foo/bar/baz, /foo/bar, /foo, /bar)
func orderStackGroupKeys(m map[string]StackGroup) []string {
	ret := make([]string, len(m))
	i := 0

	for k := range m {
		ret[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret
}

// orderStackIconKeys returns the map indexes in descending order (/foo/bar/baz, /foo/bar, /foo, /bar)
// TODO(displague) this is identical to orderStackGroupKeys. generics?
func orderStackIconKeys(m map[string]*v1alpha1.IconSpec) []string {
	ret := make([]string, len(m))
	i := 0

	for k := range m {
		ret[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret
}

// orderStackResourceKeys returns the map indexes in descending order (/foo/bar/baz, /foo/bar, /foo, /bar)
// TODO(displague) this is identical to orderStackGroupKeys. generics?
func orderStackResourceKeys(m map[string]StackResource) []string {
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
// TODO(displague) this is identical to orderStackGroupKeys. generics?
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

func (sp *StackPackage) applyGroupAnnotations(crdPath string, crd *apiextensions.CustomResourceDefinition) {
	// A group among many CRDs applies to all CRDs
	groupPathsOrdered := orderStackGroupKeys(sp.Groups)
	for _, groupPath := range groupPathsOrdered {
		group := sp.Groups[groupPath]
		if strings.HasPrefix(crdPath, groupPath) {
			crd.ObjectMeta.Annotations[annotationGroupTitle] = group.Title
			crd.ObjectMeta.Annotations[annotationGroupCategory] = group.Category
			crd.ObjectMeta.Annotations[annotationGroupDescription] = group.Description
			break
		}
	}

}

// applyResourceAnnotations annotates resource.yaml properties to the appropriate StackPackage CRD
// A resource.yaml must reside in the same path or higher than the CRD and must contain an id matching
// the CRD kind
func (sp *StackPackage) applyResourceAnnotations(crdPath string, crd *apiextensions.CustomResourceDefinition) {
	// TODO(displague) which pattern should associate *resource.yaml to their matching *crd.yaml files?
	// * resource.yaml contain "id=_kind_" (or gvk)
	// * limit one-crd per path
	// * file names match their CRD: [_group_]/[_kind_.[_version_.]]{resource,crd}.yaml
	resourcePathsOrdered := orderStackResourceKeys(sp.Resources)
	for _, resourcePath := range resourcePathsOrdered {
		dir := filepath.Dir(resourcePath)
		resource := sp.Resources[resourcePath]
		if strings.HasPrefix(crdPath, dir) && strings.EqualFold(resource.ID, crd.Spec.Names.Kind) {
			crd.ObjectMeta.Annotations[annotationResourceTitle] = resource.Title
			crd.ObjectMeta.Annotations[annotationResourceTitlePlural] = resource.TitlePlural
			crd.ObjectMeta.Annotations[annotationResourceCategory] = resource.Category
			crd.ObjectMeta.Annotations[annotationResourceDescription] = resource.Description

			break
		}
	}
}

// applyIconAnnotations annotates icon data to the appropriate StackPackage CRDs
// An icon among many CRDs applies to all CRDs. Only the nearest ancestor icon is applied to CRDs.
func (sp *StackPackage) applyIconAnnotations(crdPath string, crd *apiextensions.CustomResourceDefinition) {
	iconPathsOrdered := orderStackIconKeys(sp.Icons)
	for _, iconPath := range iconPathsOrdered {
		icon := sp.Icons[iconPath]
		if strings.HasPrefix(crdPath, iconPath) {
			crd.ObjectMeta.Annotations[annotationStackIcon] = "data:" + icon.MediaType + ";base64," + icon.Base64IconData
			break
		}
	}

}

// applyUISpecAnnotations annotates ui-schema.yaml contents to the appropriate StackPackage CRDs
// Existing ui-schema annotation values are preserved. All existing and matching ui-schema.yaml files
// will be concatenated as a multiple document YAML.
// A ui-schema.yaml among many CRDs applies to all neighboring and descendent CRDs,
// a _kind_.ui-schema.yaml applies to crds with a matching kind
func (sp *StackPackage) applyUISpecAnnotations(crdPath string, crd *apiextensions.CustomResourceDefinition) {
	uiPathsOrdered := orderStringKeys(sp.UISpecs)
	for _, uiSpecPath := range uiPathsOrdered {
		spec := strings.Trim(sp.UISpecs[uiSpecPath], "\n")
		dir := filepath.Dir(uiSpecPath)
		basename := filepath.Base(uiSpecPath)
		if strings.HasPrefix(crdPath, dir) &&
			(basename == uiSpecFileName || strings.HasPrefix(basename, strings.ToLower(crd.Spec.Names.Kind)+".")) {
			// TODO(displague) are there concerns about the concatenation order of ui-schema.yaml and kind.ui-schema.yaml?
			if len(crd.ObjectMeta.Annotations[annotationStackUISpec]) > 0 {
				appendedUI := fmt.Sprintf("%s\n---\n%s", crd.ObjectMeta.Annotations[annotationStackUISpec], spec)
				crd.ObjectMeta.Annotations[annotationStackUISpec] = appendedUI
			} else {
				crd.ObjectMeta.Annotations[annotationStackUISpec] = spec
			}
		}
	}

}
