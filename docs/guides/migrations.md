---
title: Migrations
toc: true
weight: 200
indent: true
---

# Migrations

Crossplane is built on CustomResourceDefinition (CRD) objects that expose an API
with a certain schema. As the capabilities of the controller reconciling a CRD
changes, so does its schema, which is what the users and their systems interact
with. You will find that each schema is enumerated by a [version](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/).

Kubernetes provides a way to make sure users of CRDs are able to migrate
to the new versions without breaking their existing systems in a way that give
them enough time to make the changes to adopt the new version. The versions in
this context have certain implications about the API stability guarantees that
the owner of the CRD promises. Crossplane community follows those guarantees.
For more details about what those guarantees are, see [API Versioning](https://kubernetes.io/docs/reference/using-api/#api-versioning).

In this document, the migration steps for the most common cases Crossplane users
encounter will be covered.

## Managed Resources

There are mainly three classes of migration scenarios:

* Changes that don't require version increase.
* Version increased with conversion support.
* Version increased with no conversion support.

### Change of Schema Without Version Increase

This class of changes include cases where the developer changed the schema in a
way that is fully compatible with the old schema. For example, addition of a new
optional field falls under this class.

You don't need to take any action for these cases.

### Version Increased With Conversion Support

In this case, a new schema is introduced as a new API version and the developer
provided a way for Kubernetes API server to perform conversions between the old
and the new version. Keep in mind that not all incompatible changes require
developer to provide conversion support, see [API Versioning](https://kubernetes.io/docs/reference/using-api/#api-versioning) to see in what cases
they need to provide it.

In this case, your systems are expected to continue to work after the upgrade
without any immediate changes. In most cases, developers would deprecate the old
version and warn you about its removal schedule in the release notes. The steps
described below need to be taken before that removal date.

The important thing to keep in mind here is that Kubernetes API server is able to
serve multiple versions equally well but the object stored in its persistence
layer has a single shape. The point of the steps below is that we'll have the
same stored version of our objects but saved using the new version so that we
don't need to worry about what version is served at the moment.

#### Direct Managed Resource Usage

If you use managed resources directly without Composition, the only action you
need to take is to change the YAML files applied to the cluster, making sure
you use the new `apiVersion` and change the rest of the file to adhere to the
new schema.

It's essentially reapplying the new YAML.

#### Composition

If you use managed resources as part of your `Composition`s, then the Crossplane
is what interacts with Kubernetes API, so what it will apply needs to be changed
to the new version. In addition, Crossplane composite reconciler tracks its
resource via reference and that includes `apiVersion` which you'll need to update.
The tricky part about the latter is that the `apiVersion` in that reference should
not be different than what you have in your `Composition` template. So, we'll
need to shut down Crossplane to perform this operation.

Another reason for shutting down Crossplane is to make sure to update `Configuration`
packages that may be used to deploy the `Composition`s so that changes we make
to `Composition`s are not overridden by Crossplane trying to make sure the
`Composition`s are up-to-date with the `Configuration`s installed in the cluster.

> You may not need to update references after https://github.com/crossplane/crossplane/issues/2905
> is resolved.

1. Set `spec.replicas` of the `Deployment` object of Crossplane to `0` and wait
  until there is no Crossplane `Pod`.
  ```console
  kubectl -n crossplane-system scale deployment/crossplane --replicas=0
  ```
3. Edit all your `Configuration` packages, push their package image and use the
   new version of those packages.
4. Edit the base object in every `Composition` to match the schema of the new
  version.
  ```console
  # This can help you list all the Compositions that use a certain CRD.
  kubectl get composition -o json | \
  jq .items | \
  jq -c 'map(select( (.spec.resources[0].base.apiVersion | contains("ec2.aws.crossplane.io/v1beta1")) and (.spec.resources[0].base.kind | contains("VPC")) ))' | \
  jq '.[].metadata.name'
  ```
4. Edit the `spec.resourceRefs` of every composite object that uses affected `Composition`.
  ```console
  # This can help you list all the composite resources that has a resource with
  # certain CRD.
  kubectl get composite -o json | \
  jq .items | \
  jq -c 'map(select( (.spec.resourceRefs[0].apiVersion | contains("ec2.aws.crossplane.io/v1beta1")) and (.spec.resourceRefs[0].kind | contains("VPC")) ))' | \
  jq '"\(.[].kind)/\(.[].metadata.name)"'
  ```
5. Run the commands above and make sure you get empty result with the old API
  versions.
6. Set `spec.replicas` of the `Deployment` object of Crossplane to `1`.
  ```console
  kubectl -n crossplane-system scale deployment/crossplane --replicas=1
  ```

### Version Increased With No Conversion Support

There are two main cases where we see no conversion support:
* In cases where the change involves deploying another CRD, like developers deciding
  to use another API group or kind, there is no way to provide conversion support
  because Kubernetes does not support this case.
* The developer does not have to provide a migration path according to the
  stability guarantees implied by the versions. In that case, you should see big
  warning in the release notes.

#### API Group or Kind Rename

Kubernetes does not allow changing the API group or kind of the CRD, so the developers
introduce new CRDs to the cluster when they need to use new values for those. What
we need to do is similar to the case where there is conversion support but instead
of changing only the version, we'll change the whole `apiVersion` and `kind` fields
along with other fields whose schema might have changed.

##### Direct Managed Resource Usage

If you use managed resources directly without `Composition`, the only action you
need to take after you upgrade the provider is to change the YAML files applied
to the cluster, making sure you use the new `apiVersion`, `kind` and change the
rest of the file to adhere to the new schema.

One thing to check is that whether the provider is still spinning up the controller
of the old CRD. If it doesn't, it means that nothing is reconciling the changes
you make to the resource, so you can safely force delete the instance with the
following command after changing to the correct identifiers:
```console
# The following deletes a whole API group called "identity.aws.crossplane.io" from
# the resources that have "aws" category.

# Remove finalizer.
kubectl get aws -o name \
  | grep 'identity.aws.crossplane.io' \
  | xargs -I {} kubectl patch {} -p '{"metadata":{"finalizers": []}}' --type=merge
# Delete all resources in identity group
kubectl get aws -o name \
  | grep 'identity.aws.crossplane.io' \
  | xargs kubectl delete
```

If the controller of the old CRD is started in the new version of the provider,
then you'll need to make sure to set `spec.deletionPolicy` to `Orphan` and then
delete gracefully.

##### Composition

If you use managed resources as part of your `Composition`s, then the Crossplane
is what interacts with Kubernetes API, so what it will apply needs to be changed
to the new version. In addition, Crossplane composite reconciler tracks its
resource via reference and that includes `apiVersion` which you'll need to update.
The tricky part about the latter is that the `apiVersion` in that reference should
not be different than what you have in your `Composition` template. So, we'll
need to shut down Crossplane to perform this operation.

Another reason for shutting down Crossplane is to make sure to update `Configuration`
packages that may be used to deploy the `Composition`s so that changes we make
to `Composition`s are not overridden by Crossplane trying to make sure the
`Composition`s are up-to-date with the `Configuration`s installed in the cluster.

> You may not need to update references after https://github.com/crossplane/crossplane/issues/2905
> is resolved.

1. Upgrade to the new provider version.
2. Set `spec.replicas` of the `Deployment` object of Crossplane to `0` and wait
   until there is no Crossplane `Pod`.
  ```console
  kubectl -n crossplane-system scale deployment/crossplane --replicas=0
  ```
3. Edit all your `Configuration` packages, push their package image and use the
   new version of those packages.
4. Edit the base object in every `Composition` to match the schema of the new
   version.
  ```console
  # This can help you list all the Compositions that use a certain CRD.
  kubectl get composition -o json | \
  jq .items | \
  jq -c 'map(select( (.spec.resources[0].base.apiVersion | contains("ec2.aws.crossplane.io/v1beta1")) and (.spec.resources[0].base.kind | contains("VPC")) ))' | \
  jq '.[].metadata.name'
  ```
4. Edit the `spec.resourceRefs` of every composite object that uses affected `Composition`.
  ```console
  # This can help you list all the composite resources that has a resource with
  # certain CRD.
  kubectl get composite -o json | \
  jq .items | \
  jq -c 'map(select( (.spec.resourceRefs[0].apiVersion | contains("ec2.aws.crossplane.io/v1beta1")) and (.spec.resourceRefs[0].kind | contains("VPC")) ))' | \
  jq '"\(.[].kind)/\(.[].metadata.name)"'
  ```
5. Run the commands above and make sure you get empty result with the old API
   versions.
6. Back up the existing managed resource instances to your file system TODO!!!! DON'T MERGE!!
7. Set `spec.replicas` of the `Deployment` object of Crossplane to `1`.
  ```console
  kubectl -n crossplane-system scale deployment/crossplane --replicas=1
  ```

#### Version Removal With No Conversion

In this case, the upgrade process of the provider will not be able to succeed if
you have existing instances using the old API version. What we need to do is
essentially to remove the representation of the resource from cluster and then
after the upgrade, import it back. The key point is to make sure to set
`spec.deletionPolicy` to `Orphan` so that when we delete the old resource, the
actual service instance in the external system is not deleted.

#### Direct Managed Resource Usage

Follow the instructions here if you are using managed resources directly without
`Composition`.

1. Set `spec.deletionPolicy` of affected managed resource instances to `Orphan`.
2. Keep note of the value of `crossplane.io/external-name` of the resource.
3. Delete the affected instances.
4. Proceed with upgrading the provider, i.e. patching `spec.package` field of
  `Provider` object.
5. Use external name annotation to [import](https://crossplane.io/docs/master/concepts/managed-resources.html#importing-existing-resources)
  the resource back to the cluster.

#### Composition

To be filled.