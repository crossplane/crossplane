# Stack Package UI configuration metadata

* Owner: Steven Rathbauer (@rathpc), Marques Johansson (@displague)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Terms

* **Stack**: A Stack is a Crossplane stack-manager managed package.  These packages may bundle one or many Crossplane applications or infrastructure provider controllers.
* **Stacks**: Stacks could refer to more than one Crossplane stack-manager packages or the Stack-Manager system as a whole.

See the [Stacks Design Doc](design-doc-stacks.md) for more details about Crossplane Stacks.

## Objective

This document aims to provide details on the necessary metadata that will drive the UI on a configuration page with respect to Crossplane Stacks and the user configurable custom-resource fields exposed with a Stack. This is meant to be a dynamic spec that will not attempt to accommodate all potential UI elements.  This spec does not intend to be as complete as [XForms](https://en.wikipedia.org/wiki/XForms) or [HTML Forms](https://www.w3.org/TR/html52/sec-forms.html), for example.

Users may choose to annotate Stack bundled CRD today with UI hints.  This proposal suggests a means for the stack manager to apply CRD annotations at install and upgrade time.  This separation provides a better developer experience preventing the need for modifying escaped, nested, quoted, YAML or JSON annotations within a CRD document while iterating over UI design.

A YAML file named `ui-schema.yaml` is proposed for inclusion in Stacks.  This will standardize the approach Stack creators use to offer UI context to their package and offer a more engaging user experience.

Tools that deliver a user-interface based on this proposal may include:

* CLI help output
* CLI auto-completion for arguments
* Advanced validation (beyond what Kubernetes' OAS3 support delivers)
* ncurses-style installers
* web-based package managers (out-of-cluster and in-cluster)
* web-based configuration panels

This spec is not concerned with a specific UI implementation but serves to provide a foundation for supporting Stack developers and users through basic form input.

## Proposed File format

The `ui-schema.yaml` content will not be dictated in this design doc.  Rather, the [formal specification](#formal-specification) is a simple guideline for the type and size of the document.  The contents are otherwise left open to interpretation by Stack tools with the expectation that these tools will promote interoperable standards.  Specifications suggested through examples and supporting text in this design document should not be considered doctrine.

## Proposed UI Elements

The following are proposed UI elements for Stack tool authors to consider supporting.

* Sections
* Title
* Subtitle
* Input: Text, Number, Password, Hidden, Readonly
* Textarea
* Select Dropdown: Single, Multiple
* Checkbox: Single, Group
* Radio Button Group

Support for all of these UI types is not mandated or required.  Other UI elements, not listed here, may also be offered.

## Stack Implementation

The optional addition of a `ui-schema.yaml` file within the package tree will be used to drive the UI.

The primary purpose of the `ui-schema.yaml` format is to allow for easy authoring of the definition of UI markup, validation, and errors for a more complete UI and UX. The `ui-schema.yaml` file will be parsed and validated as part of package validation and ultimately serialized as a `YAML` annotation on the respective CRD at install time.

A root `ui-schema.yaml` file may be used to set global UI metadata.  This may be useful for describing required fields that appear across resources and versions.  Resource specific UI metadata should take priority over global UI metadata.

### Example addition of `ui-schema.yaml` in a package tree

```text
.registry/
├── icon.png
├── app.yaml # Application metadata.
├── ui-schema.yaml # Optional UI spec for configuration metadata
├── install.yaml # Optional install metadata.
├── rbac.yaml # Optional RBAC permissions.
└── resources
      └── databases.foocompany.io # Group directory
            ├── group.yaml # Optional Group metadata
            ├── icon.png # Optional Group icon
            └── mysql # Kind directory by convention
                └── v1alpha1
                    ├── mysql.v1alpha1.crd.yaml # Required CRD
                    ├── ui-schema.yaml # Optional UI spec for configuration metadata
                    ├── icon.png # Optional resource icon
                    ├── resource.yaml # Resource level metadata.
```

### Example ui-schema.yaml

This file contains multiple sections and a variety of different input types with validation overrides, extended validation and custom error messages.

```yaml
uiSpecVersion: 0.3
uiSpec:
- title: Configuration
  description: Enter information specific to the configuration you wish to create.
  items:
  - name: dbReplicas
    controlType: singleInput
    type: integer
    path: .spec.dbReplicas
    title: DB Replicas
    description: The number of DB Replicas
    default: 1
    validation:
    - minimum: 1
    - maximum: 3
  - name: masterPassword
    controlType: singleInput
    type: password
    path: .spec.masterPassword
    title: DB Master Password
    description: The master DB password. Must be between 8-32 characters long
  - name: subdomain
    controlType: singleInput
    type: string
    path: .spec.subdomain
    title: Subdomain
    pattern: ^([A-Za-z0-9](?:(?:[-A-Za-z0-9]){0,61}[A-Za-z0-9])?){2,62}$
    description: Enter a value for your subdomain. It cannot start or end with a dash and must be between 2-62 characters long
    validation:
    - minLength: 2
    - maxLength: 62
  - name: instanceSize
    controlType: singleSelect
    path: .spec.instanceSize
    title: Instance Size
    enum:
    - 'Option-1'
    - 'Option-2'
    - 'Option-3'
    validation:
    - required: true
      customError: You must select an instance size for your configuration!
```

### CRD Annotation Example

An example injection of the `ui-schema.yaml` as a CRD annotation follows.  Keep in mind that Kubernetes annotations are limited to 256kb (less in older versions).

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: rdsinstances.database.aws.crossplane.io
  annotations:
    stacks.crossplane.io/ui-spec: |-
      ---
      uiSpecVersion: 0.3
      uiSpec:
      - title: Configuration
        description: Enter information specific to the configuration you wish to create.
        items:
        - name: dbReplicas
          controlType: singleInput
          type: integer
          path: ".spec.dbReplicas"
          title: DB Replicas
          description: The number of DB Replicas
          default: 1
          validation:
          - minimum: 1
          - maximum: 3
        - name: masterPassword
          controlType: singleInput
          type: password
          path: ".spec.masterPassword"
          title: DB Master Password
          description: The master DB password. Must be between 8-32 characters long
        - name: subdomain
          controlType: singleInput
          type: string
          path: ".spec.subdomain"
          title: Subdomain
          pattern: "^([A-Za-z0-9](?:(?:[-A-Za-z0-9]){0,61}[A-Za-z0-9])?){2,62}$"
          description: Enter a value for your subdomain. It cannot start or end with a dash
            and must be between 2-62 characters long
          validation:
          - minLength: 2
          - maxLength: 62
        - name: instanceSize
          controlType: singleSelect
          path: ".spec.instanceSize"
          title: Instance Size
          enum:
          - Option-1
          - Option-2
          - Option-3
          validation:
          - required: true
            customError: You must select an instance size for your configuration!
      ---
      uiSpecVersion: 0.3
      uiSpec:
      - title: Supplementary
        description: A supplementary UI annotation
  labels:
    controller-tools.k8s.io: "1.0"
```

## Formal Specification

The current design calls for unbiased processing of the UI YAML to annotations, regardless of content.

Because the YAML will be concatenated as a multiple document YAML in the annotation:

* the file must contain a valid YAML document
* the fully transcribed YAML must be less than 256kb

These are the only requirements.

TBD: A specification for the supported elements and their properties.  A validator and validation document pair should be offered.  There is no clear YAML validation format as there is DTD for XML documents.  Validation may be deferred to the capabilities of YAML parsing in Go via <https://github.com/ghodss/yaml>.  Unrecognized fields will be ignored while typed parameters will expect to conform to a Go type.

## Open Questions

* How should multiple document YAML annotations signify priority?
* Should dependent and co-dependent fields be supported? (Not relevant if Crossplane remains unopinionated about the file contents)
