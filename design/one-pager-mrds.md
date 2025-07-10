# Managed Resource Definitions

* Owner: Nic Cope (@negz)
* Reviewer: Philippe Scorsolini (@phisco)
* Status: Accepted

## Background

Crossplane managed resources (MRs) suffer from two long-standing issues.

MRs can automatically write sensitive connection details like addresses,
usernames, and passwords to a Kubernetes Secret. We call these connection
details. Today it's very hard to discover what connection details an MR
supports. The only way to do so is to create an MR and see what it writes to its
connection secret. This is tracked in [issue 1143][1] - Crossplane's third most
upvoted issue.

Crossplane bundles related MRs together into a provider - like provider-github
or provider-aws-ec2. Today some of these providers install up to 100 MRs. With
v2 this'll double, because v2 introduces namespaced MRs but keeps the existing
cluster scoped MRs for backward compatibility.

Each MR is powered by a CustomResourceDefinition (CRD) and a controller. Each
CRD has a performance penalty on the Kubernetes API server - mostly memory
usage. Each CRD also creates one or more API endpoints that clients like kubectl
may need to walk.

It's not possible to install only some of a provider's MRs. It's all or nothing.
The ability to install only the MRs you intend to use is tracked in [issue
2869][2] and [issue 4192][3] - collectively Crossplane's most upvoted issue of
all time. A few years ago we broke up the largest providers into [families of
smaller providers][4]. This helped alleviate the issue, but some large
Crossplane deployments still work around it - for example by building their own
providers that have only the MRs they need.

## Proposal

I propose we introduce a new type - ManagedResourceDefinition (MRD).

An MRD would be a lightweight abstraction on a CRD:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: ManagedResourceDefinition
metadata:
  name: nopresources.nop.crossplane.io
spec:
  group: nop.crossplane.io
  names:
    categories:
    - nop
    kind: NopResource
    listKind: NopResourceList
    plural: nopresources
    singular: nopresource
  scope: Cluster
  versions:
    name: v1alpha1
    schema:
      openAPIV3Schema:
        # Omitted for brevity
    served: true
    storage: true
  connectionDetails:
  - name: password
    description: Definitely real password for this resource that does nothing
  state: Active
```

An MRD would be schematically identical to a CRD, but have two additional
`spec` fields:

* `spec.connectionDetails` - An array of connection detail keys and descriptions
* `spec.state` - Toggles whether the underlying CRD is created or not

I propose we update all providers to deliver MRDs - not CRDs - as their package
payload.

I propose all MRDs be inactive (`spec.state: Inactive`) by default, and that we
add another new type to automatically activate matching MRDs. This new type
would be called a ManagedResourceActivationPolicy (MRAP).

Here's an example MRAP:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: ManagedResourceActivationPolicy
metadata:
  name: aws
spec:
  activate:
  - instances.rds.m.aws.crossplane.io
  - *.ec2.m.aws.crossplane.io
```

An MRAP would specify an array of MRD names to activate. The array supports
wildcard prefixes (like `*.aws.crossplane.io`) but not full regular expressions.

A controller would watch MRAPs. Whenever any MRAP changes the controller would:

1. List all MRAPs
1. Compute the set of MRs that should be active
1. Activate the relevant MRDs

Crossplane would create a default MRAP at install time that activated `*` - i.e.
all MRs. A CLI flag (and Helm chart value) would allow you to opt out of
installing this default MRAP.

Crossplane would support packaging MRAPs in Configuration packages. This
addresses my concern that MR CRD filtering breaks Crossplane's package
dependency model (see [the provider family design][4] for context). Using MRAPs
a Configuration package can:

1. Depend on a provider (e.g. provider-aws-ec2)
1. Include an MRAP to specify specifically what MRs it needs

This proposal requires updates to all providers. We use [controller-runtime][5]
to build providers. controller-runtime assumes you'll add all controllers to a
`Manager`, which will refuse to start if any of its controllers are missing
CRDs:

```
error: Cannot start controller manager: no matches for kind "NopResource" in version "nop.crossplane.io/v1alpha1"
```

I propose we add a utility to crossplane-runtime that allows a provider to watch
its MR CRDs and only start a controller when the CRD is created. I'd prefer
providers to watch CRDs (not MRDs) because watching MRDs would introduce a
dependency on a type installed by Crossplane (MRD). No such dependency exists
today.

I propose providers only support dynamically starting MR controllers - not
stopping them. Once an MR controller is started it wouldn't be possible to stop
it. This is because stopping a controller requires stopping its informers, which
is quite hard using controller-runtime. We'd need to migrate all providers to
use something like the [controller engine][7] Crossplane uses to dynamically
start and stop XR controllers. This would be a significant architectural change
to providers. This implies an MRD's `spec.state` will only be allowed to
transition from `Inactive -> Active`, not the other way.

We could consider supporting deactivating an active MR (at significant
implementation cost) if there was sufficient community demand.

Crossplane will need to know which providers support 'late activation' of MRDs
and which don't. I propose providers include a hint in their package metadata:

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws-ec2
spec:
  capabilities:
  - late-mr-activation
```

If Crossplane sees a provider with the `late-mr-activation` capability it'll
create its MRDs with `spec.state: Inactive`. Its CRDs won't be created until an
MRAP toggles its MRDs state to `Active`.

If Crossplane sees a provider without the `late-mr-activation` capability it'll
create its MRDs with `spec.state: Active`. This'll cause Crossplane's MRD
controller to immediately create the necessary CRDs.

## Migration Plan

This proposal should be relatively simple to implement in Crossplane core, but
it requires significant changes for providers.

Today providers use [controller-tools][6] to generate YAML CRD manifests from Go
types. These CRDs are then baked into a Crossplane OCI package using the
`crossplane` CLI.

For a provider to package MRDs instead of CRDs it'd need a tool that can
generate MRD YAML manifests from Go structs. This could be done 'directly' or by
generating a CRD and post-processing it to create an MRD - e.g. changing the
type and adding the connection details.

Older versions of Crossplane wouldn't understand MRDs, so a provider that
switches from CRDs to MRDs wouldn't work on older versions of Crossplane.

To alleviate these migration issues I propose that Crossplane automatically
translate MR CRDs to MRDs at package install time. Providers would be expected
to eventually switch to packaging MRDs directly, but this would happen only
after all supported versions of Crossplane had GA support for MRDs.

## Alternatives Considered

I considered the following alternatives before landing on this proposal.

### Use CRD Annotations for Connection Details

Connection details (not CRD filtering) were the original motivation for the MRD
design. I wasn't convinced MRDs were worth the churn purely as a place to
document connection details. The alternative to MRDs for this use case was to
annotate CRDs, e.g.:

```yaml
metadata:
  annotations:
    connection.crossplane.io/username: The username for this MR
    connection.crossplane.io/address: The IP address for this MR
```

This'd likely be a good enough solution for connection details, but it's a worse
UX compared to MRD due to the lack of schema.

Given MRDs are also useful for CRD filtering, I discarded this alternative.

### Use `spec.enabledAPIs` to Filter CRDs

When we first started discussing filtering CRDs, adding `spec.enabledAPIs` to
the Provider package was a popular idea. For example:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-tf-azure
spec:
  enabledAPIs:
  - Provider.*
  - virtual.*
  - lb
  - resource.azure.tf.crossplane.io/v1alpha1.ResourceGroup
  package: ulucinar/provider-tf-azure-arm64:build-83b9bfc0
```

This would be passed down to the ProviderRevision and ultimately to the running
provider binary by configuring an `--enabled-apis` flag.

This seems like a good UX on the surface, but dependencies make it challenging.
We'd need to update the package dependency model to be aware of what APIs a
dependent Configuration needed - not just what providers it needed. If multiple
Configurations depended on the same provider Crossplane would need to compute
the set of APIs (MRs) that should be enabled.

I believe MRD and MRAP provides a simpler way to filter what CRDs are installed
while still supporting package dependencies.

### Use a Man-in-the-Middle Proxy to 'Black Hole' Provider CRD Watches

In this alternative we'd run a man-in-the-middle (MITM) proxy between providers
and the API server. The proxy would be MRD or CRD aware. If a provider started a
watch for a type that didn't really exist, the proxy would act as if the type
did exist. It's serve a Kubernetes style list/watch REST endpoint that simply
pretended the type existed but that there were no instances of it.

This'd remove the need to update providers at all. The proxy would 'swap out'
provider watches for a real one when an MRD was enabled. Another benefit of the
proxy approach is that it could be enabled and disabled globally, e.g. to easily
roll back the feature if needed.

One downside of the proxy approach is that providers would still run potentially
hundreds of controller goroutines and watches for types that didn't exist.
This'd incur a non-zero compute and I/O penalty, though we haven't measured to
know whether it'd be meaningful.

Another downside is the risk of putting a proxy between a controller and the API
server. The proxy would need to act exactly like the API server or it could
introduce subtle bugs. If the proxy crashes or fails in any way, the provider
would be unable to function.

Ultimately we think this is a compelling idea, but more complex overall
relative to updating providers to late-start their controllers.

[1]: https://github.com/crossplane/crossplane/issues/1143
[2]: https://github.com/crossplane/crossplane/issues/2869
[3]: https://github.com/crossplane/crossplane/issues/4192
[4]: https://github.com/crossplane/crossplane/blob/v1.20.0/design/design-doc-smaller-providers.md
[5]: https://github.com/kubernetes-sigs/controller-tools
[6]: https://github.com/kubernetes-sigs/controller-tools
[7]: https://pkg.go.dev/github.com/crossplane/crossplane@v1.20.0/internal/engine
