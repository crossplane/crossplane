---
title: Upgrading to v0.14
toc: true
weight: 210
indent: true
---

# Upgrading to v0.14

Crossplane made a small handful of breaking changes in v0.14. The most broadly
impactful change was updating the `CompositeResourceDefinition` (XRD) schema to
support defining multiple versions of a composite resource (XR) at once. This
guide covers how to upgrade from v0.13 of Crossplane to v0.14.

- [Updating CompositeResourceDefinitions](#updating-compositeresourcedefinitions)
- [Updating Packages](#updating-packages)

## Updating CompositeResourceDefinitions

In v0.14 the schema of XRD was updated to support defining multiple versions of
an XR. This update requires manual update steps. To upgrade from v0.13 to v0.14
you must:

1. Ensure you have up-to-date YAML representations of all of your XRDs.
1. `helm upgrade` your Crossplane release.
1. Update and apply all of your XRDs.

Note that Crossplane will not actively reconcile your XRs between steps 2 and 3,
and you will see some errors in the events and logs, but your managed resources
(and thus infrastructure) will continue to run. Follow the below steps in order
to update your XRDs for v0.14:

1. Rename `spec.crdSpecTemplate` to `spec.versions`.
1. Move `spec.versions.group` to `spec.group`.
1. Move `spec.versions.names` to `spec.names`.
1. Rename `spec.versions.version` to `spec.versions.name`
1. Rename `spec.versions.validation` (if set) to `spec.versions.schema`.
1. Rename `spec.versions.additionalPrinterColumns[].JSONPath` (if set) to
   `spec.versions.additionalPrinterColumns[].jsonPath`.
1. Set `spec.versions.served` to `true`.
1. Set `spec.versions.referenceable` to `true`.
1. Make `spec.versions` a single element array.

For example, the below XRD:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: CompositeResourceDefinition
metadata:
  name: xpostgresqlinstances.database.example.org
spec:
  claimNames:
    kind: PostgreSQLInstance
    plural: postgresqlinstances
  connectionSecretKeys:
    - username
    - password
    - endpoint
    - port
  crdSpecTemplate:
    group: database.example.org
    version: v1alpha1
    names:
      kind: XPostgreSQLInstance
      plural: xpostgresqlinstances
    validation:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              parameters:
                type: object
                properties:
                  storageGB:
                    type: integer
                required:
                  - storageGB
            required:
              - parameters
```

Would become:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: CompositeResourceDefinition
metadata:
  name: xpostgresqlinstances.database.example.org
spec:
  group: database.example.org
  names:
    kind: XPostgreSQLInstance
    plural: xpostgresqlinstances
  claimNames:
    kind: PostgreSQLInstance
    plural: postgresqlinstances
  connectionSecretKeys:
    - username
    - password
    - endpoint
    - port
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              parameters:
                type: object
                properties:
                  storageGB:
                    type: integer
                required:
                  - storageGB
            required:
              - parameters
```

## Updating Packages

A minor breaking change was made to on-disk package types
(`meta.pkg.crossplane.io`). In v0.13, the `spec.crossplane` field was present to
specify a compatible Crossplane version range, but it was not honored by the
package manager when packages were installed. The field was refactored to
`spec.crossplane.version` meaning that packages that previously specified
`spec.crossplane` will fail to parse when building with the Crossplane CLI or
installing into a Crossplane Kubernetes cluster. If `spec.crossplane` was not
specified, packages compatible with Crossplane v0.13 will continue to be
compatible in v0.14. This is true for both `Provider` and `Configuration`
packages.

The following example shows how a `Configuration` package that specified
`spec.crossplane` can be updated to specify Crossplane version constraints that
will be honored by the package manager in v0.14:

```yaml
apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: my-configuration
spec:
  crossplane: ">=v0.13.0"
```

Would become:

```yaml
apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: my-configuration
spec:
  crossplane:
    version: ">=v0.13.0"
```

Please note that while `spec.dependsOn` is also a valid field in on-disk package
types, it is not yet honored by the package manager and will be ignored at
installation time.
