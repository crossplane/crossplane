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

package ccrd

// TODO(negz): Add descriptions to schema fields.

// DefinedInfrastructureSpecProps is a partial OpenAPIV3Schema for the spec
// fields that Crossplane expects to be present for all composite infrastructure
// resources.
const DefinedInfrastructureSpecProps = `
compositionRef:
  properties:
    name:
      type: string
  required:
  - name
  type: object
compositionSelector:
  properties:
    matchLabels:
      additionalProperties:
        type: string
      type: object
  required:
  - matchLabels
  type: object
resourceRefs:
  items:
    type: object
    properties:
      apiVersion:
        type: string
      kind:
        type: string
      name:
        type: string
      uid:
        type: string
    required:
    - apiVersion
    - kind
    - name
  type: array
requirementRef:
  properties:
    name:
      type: string
    namespace:
      type: string
  required:
  - name
  - namespace
  type: object
writeConnectionSecretToRef:
  properties:
    name:
      description: Name of the secret.
      type: string
    namespace:
      description: Name of the secret.
      type: string
  required:
  - name
  - namespace
  type: object
`

// DefinedInfrastructureStatusProps is a partial OpenAPIV3Schema for the
// status fields that Crossplane expects to be present for all composite
// infrastructure resources.
const DefinedInfrastructureStatusProps = `
bindingPhase:
  enum:
  - Unbindable
  - Unbound
  - Bound
  - Released
  type: string
conditions:
  description: Conditions of the resource.
  items:
    properties:
      lastTransitionTime:
        format: date-time
        type: string
      message:
        type: string
      reason:
        type: string
      status:
        type: string
      type:
        type: string
    required:
    - lastTransitionTime
    - reason
    - status
    - type
    type: object
  type: array
`

// PublishedInfrastructureSpecProps is a partial OpenAPIV3Schema for the spec
// fields that Crossplane expects to be present for all published infrastructure
// resources.
const PublishedInfrastructureSpecProps = `
compositionRef:
  properties:
    name:
      type: string
  required:
  - name
  type: object
compositionSelector:
  properties:
    matchLabels:
      additionalProperties:
        type: string
      type: object
  required:
  - matchLabels
  type: object
resourceRef:
  properties:
    name:
      type: string
  required:
  - name
  type: object
writeConnectionSecretToRef:
  properties:
    name:
      type: string
  required:
  - name
  type: object
`

// PublishedInfrastructureStatusProps is a partial OpenAPIV3Schema for the
// status fields that Crossplane expects to be present for all composite
// infrastructure resources.
const PublishedInfrastructureStatusProps = `
bindingPhase:
  enum:
  - Unbindable
  - Unbound
  - Bound
  - Released
  type: string
conditions:
  items:
    properties:
      lastTransitionTime:
        format: date-time
        type: string
      message:
        type: string
      reason:
        type: string
      status:
        type: string
      type:
        type: string
    required:
    - lastTransitionTime
    - reason
    - status
    - type
    type: object
  type: array
`
