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

package manager

import (
	"context"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

// crdTemplates is an ordered list of resources to be applied for a given GVK
// type crdTemplates []string

// template is a tuple of the name of a template and the template body itself
type template struct {
	name, template string
}

// templates are an ordered list of template
type templates []template

// TODO(displague) can we just use templates as a map[string]string ?

type templateResource = *unstructured.Unstructured

// type templateResources []templateResource

type templateResourceMap map[string]templateResource
type templateResourceErrors map[string]error

type templateSyncer struct {
	client    client.Client
	templates templates
}

// GetTemplates returns the resources described by a template stack
// for a particular GVK
// TODO(displague) move this to a Stack method
func getTemplates(_ v1alpha1.Stack, _ schema.GroupVersionKind) templates {
	return templates{}
}

// Sync fetches and applies updates to a set of templates and returns the
// updated templates and any errors encountered handling each template
func (ts templateSyncer) Sync(ctx context.Context) (templateResourceMap, templateResourceErrors) {
	current := ts.FetchResources(ctx, ts.templates)
	rendered, errs := ts.templates.Render(current)
	for key, err := range errs {
		if err != nil {
			// TODO(dispolague) should UpdateResources take a hint that
			// no changes are required for a resource? or should TSM always
			// update resources
			rendered[key] = current[key]
		}
	}
	updateErrs := ts.UpdateResources(ctx, rendered)
	for key, err := range errs {
		if err == nil && updateErrs[key] != nil {
			errs[key] = updateErrs[key]
		}
	}
	// TODO(displague) delete?
	return rendered, errs

}

// FetchResources fetches the resources from the Kubernetes API and returns
// a copy with their current state
// TODO(displague) also return errors per resource?
func (ts templateSyncer) FetchResources(ctx context.Context, templates templates) templateResourceMap {
	resources := templateResourceMap{}
	for _, template := range templates {
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(template.template), obj); err != nil {
			// TODO(displague) log it and continue, or is this fatal?
			// we need a more fault tollerant approach to fetching the name and
			// kind of the resource.
			continue
		}

		nsn := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}

		if err := ts.client.Get(ctx, nsn, obj); err != nil {
			// TODO(displague) see previous todo. Need a better structure to map
			// these errors with their resources. include it in
			// templateResourcemap?

			continue
		}
		resources[template.name] = obj
	}
	return resources
}

func (ts templateSyncer) UpdateResources(ctx context.Context, resources templateResourceMap) templateResourceErrors {
	errs := templateResourceErrors{}
	for key, obj := range resources {
		errs[key] = ts.client.Patch(ctx, obj, client.MergeFrom(obj))
	}
	return errs
}

// DeleteResources removes any resources that should not exist
// TODO(displague) do we need to know the owner? do we look for empty templates?
func (ts templateSyncer) DeleteResources() templateResourceErrors {
	/*
		We want to allow a template body to be emptied so a resource could be
		deleted:
		```yaml
		{{ if .somevar.ready }}
		apiVersion: ...
		kind: ...
		  spec:
		  ...
		{{ end }}
		````
		but if that is the case, then how do we get the name of object that we need
		to delete. We could collect ALL of the resources that are owned by this cr
		(Foo) and delete any that are not accounted for but we still need to fetch
		the correct type of resources .. and we don't know the "kind" if that
		template is empty..
		we need to know the kind to fetch the resources so there may need to be
		additional rules about what can or cannot be templated, or perhaps the
		delete/omit status has to be toggled in some other way
	*/
	return templateResourceErrors{}
}

// Render mutates resources using the supplied resources
func (templates) Render(templateResourceMap) (templateResourceMap, templateResourceErrors) {
	return templateResourceMap{}, templateResourceErrors{}
}
