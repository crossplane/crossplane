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

// TODO(muvaf): Consider generating the validations of the types in runtime
// so that we don't forget to update here when there are changes in those
// structs.

// InfraSpecProps is OpenAPIV3Schema for spec fields that Crossplane uses for
// Infrastructure kinds.
const InfraSpecProps = `
compositionRef:
  description: A ClassReference specifies a resource class that will be
    used to dynamically provision a managed resource when the resource
    claim is created.
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
  description: A ClassSelector specifies labels that will be used to select
    a resource class for this claim. If multiple classes match the labels
    one will be chosen at random.
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
composedRefs:
  description: A ResourceReference specifies an existing managed resource,
    in any namespace, to which this resource claim should attempt to bind.
    Omit the resource reference to enable dynamic provisioning using a
    resource class; the resource reference will be automatically populated
    by Crossplane.
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
  description: WriteConnectionSecretToReference specifies the name of
    a Secret, in the same namespace as this resource claim, to which any
    connection details for this resource claim should be written. Connection
    details frequently include the endpoint, username, and password required
    to connect to the managed resource bound to this resource claim.
  properties:
    name:
      description: Name of the secret.
      type: string
  required:
  - name
  type: object
`

// InfraStatusProps is OpenAPIV3Schema for status fields that Crossplane uses for
// Infrastructure kinds.
const InfraStatusProps = `
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
