/*
Copyright 2020 The Crossplane Authors.

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
	"github.com/pkg/errors"

	"github.com/ghodss/yaml"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

// InfraCompositeSpecProps is OpenAPIV3Schema for spec fields that Crossplane uses for
// Infrastructure kinds.
const InfraCompositeSpecProps = `
compositionRef:
  description: A composition reference specifies a composition that will be
    used to configure the composed resources.
  properties:
    apiVersion:
      description: API version of the referent.
      type: string
    fieldPath:
      description: 'If referring to a piece of an object instead of an
        entire object, this string should contain a valid JSON/Go field
        access statement, such as desiredState.manifest.containers[2].
        For example, if the object reference is to a container within
        a pod, this would take on a value like: "spec.containers{name}"
        (where "name" refers to the name of the container that triggered
        the event) or if no container name is specified "spec.containers[2]"
        (container with index 2 in this pod). This syntax is chosen only
        to have some well-defined way of referencing a part of an object.
        TODO: this design is not final and this field is subject to change
        in the future.'
      type: string
    kind:
      description: 'Kind of the referent. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
      type: string
    name:
      description: 'Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names'
      type: string
    namespace:
      description: 'Namespace of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/'
      type: string
    resourceVersion:
      description: 'Specific resourceVersion to which this reference is
        made, if any. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency'
      type: string
    uid:
      description: 'UID of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids'
      type: string
  type: object
compositionSelector:
  description: A CompositionSelector specifies labels that will be used to select
    a composition for this composite to be configured. If multiple compositions
    match the labels one will be chosen at random.
  properties:
    matchLabels:
      additionalProperties:
        type: string
      description: matchLabels is a map of {key,value} pairs. A single
        {key,value} in the matchLabels map is equivalent to an element
        of matchExpressions, whose key field is "key", the operator is
        "In", and the values array contains only "value". The requirements
        are ANDed.
      type: object
  type: object
resourceRefs:
  description: The list of the composed resources that are provisioned for this
    composite resource.
  items:
    type: object
    properties:
      apiVersion:
        description: API version of the referent.
        type: string
      fieldPath:
        description: 'If referring to a piece of an object instead of an
          entire object, this string should contain a valid JSON/Go field
          access statement, such as desiredState.manifest.containers[2].
          For example, if the object reference is to a container within
          a pod, this would take on a value like: "spec.containers{name}"
          (where "name" refers to the name of the container that triggered
          the event) or if no container name is specified "spec.containers[2]"
          (container with index 2 in this pod). This syntax is chosen only
          to have some well-defined way of referencing a part of an object.
          TODO: this design is not final and this field is subject to change
          in the future.'
        type: string
      kind:
        description: 'Kind of the referent. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
        type: string
      name:
        description: 'Name of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names'
        type: string
      namespace:
        description: 'Namespace of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/'
        type: string
      resourceVersion:
        description: 'Specific resourceVersion to which this reference is
          made, if any. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency'
        type: string
      uid:
        description: 'UID of the referent. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids'
        type: string
    required:
    - apiVersion
    - kind
    - name
  type: array
writeConnectionSecretToRef:
  description: WriteConnectionSecretsToNamespace specifies the namespace
    in which the connection secret of the composite resource will be created.
  properties:
    name:
      description: Name of the secret.
      type: string
  required:
  - name
  type: object
`

// InfraCompositeStatusProps is OpenAPIV3Schema for status fields that Crossplane uses for
// Infrastructure kinds.
const InfraCompositeStatusProps = `
bindingPhase:
  description: Phase represents the binding phase of a managed resource
    or claim. Unbindable resources cannot be bound, typically because
    they are currently unavailable, or still being created. Unbound resource
    are available for binding, and Bound resources have successfully bound
    to another resource.
  enum:
  - Unbindable
  - Unbound
  - Bound
  - Released
  type: string
conditions:
  description: Conditions of the resource.
  items:
    description: A Condition that may apply to a resource.
    properties:
      lastTransitionTime:
        description: LastTransitionTime is the last time this condition
          transitioned from one status to another.
        format: date-time
        type: string
      message:
        description: A Message containing details about this condition's
          last transition from one status to another, if any.
        type: string
      reason:
        description: A Reason for this condition's last transition from
          one status to another.
        type: string
      status:
        description: Status of this condition; is it currently True, False,
          or Unknown?
        type: string
      type:
        description: Type of this condition. At most one of each condition
          type may apply to a resource at any point in time.
        type: string
    required:
    - lastTransitionTime
    - reason
    - status
    - type
    type: object
  type: array
`

const (
	errConvertCRDTemplate = "cannot convert given crd spec template into actual crd spec"
)

// NOTE(muvaf): We use v1beta1.CustomResourceDefinition for backward compatibility
// with clusters pre-1.16

// TODO(muvaf): Every field on top level spec could be a DefinitionOption that is
// reused, although it is known that only two different kinds will be generated.

// BaseCRD returns a base template for generating a CRD.
func BaseCRD(opts ...func(*v1beta1.CustomResourceDefinition) error) (*v1beta1.CustomResourceDefinition, error) {
	falseVal := false
	// TODO(muvaf): Add proper descriptions.
	crd := &v1beta1.CustomResourceDefinition{
		Spec: v1beta1.CustomResourceDefinitionSpec{
			PreserveUnknownFields: &falseVal,
			Subresources: &v1beta1.CustomResourceSubresources{
				Status: &v1beta1.CustomResourceSubresourceStatus{},
			},
			Validation: &v1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &v1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]v1beta1.JSONSchemaProps{
						"apiVersion": {
							Type: "string",
						},
						"kind": {
							Type: "string",
						},
						"metadata": {
							// NOTE(muvaf): api-server takes care of validating
							// metadata.
							Type: "object",
						},
						"spec": {
							Type:       "object",
							Properties: map[string]v1beta1.JSONSchemaProps{},
						},
						"status": {
							Type:       "object",
							Properties: map[string]v1beta1.JSONSchemaProps{},
						},
					},
				},
			},
		},
	}
	for _, f := range opts {
		if err := f(crd); err != nil {
			return nil, err
		}
	}
	return crd, nil
}

// InfraValidation returns a CRDOption that adds infrastructure related fields
// to the base CRD.
func InfraValidation() func(*v1beta1.CustomResourceDefinition) error {
	return func(crd *v1beta1.CustomResourceDefinition) error {
		crd.Spec.Scope = v1beta1.ClusterScoped
		spec := &map[string]v1beta1.JSONSchemaProps{}
		if err := yaml.Unmarshal([]byte(InfraCompositeSpecProps), spec); err != nil {
			return errors.Wrap(err, "constant string could not be parsed")
		}
		for k, v := range *spec {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties[k] = v
		}
		status := &map[string]v1beta1.JSONSchemaProps{}
		if err := yaml.Unmarshal([]byte(InfraCompositeStatusProps), status); err != nil {
			return errors.Wrap(err, "constant string could not be parsed")
		}
		for k, v := range *status {
			crd.Spec.Validation.OpenAPIV3Schema.Properties["status"].Properties[k] = v
		}
		return nil
	}
}

func getSpecProps(template v1beta1.CustomResourceDefinitionSpec) map[string]v1beta1.JSONSchemaProps {
	switch {
	case template.Validation == nil:
		return nil
	case template.Validation.OpenAPIV3Schema == nil:
		return nil
	case len(template.Validation.OpenAPIV3Schema.Properties) == 0:
		return nil
	case len(template.Validation.OpenAPIV3Schema.Properties["spec"].Properties) == 0:
		return nil
	}
	return template.Validation.OpenAPIV3Schema.Properties["spec"].Properties
}
