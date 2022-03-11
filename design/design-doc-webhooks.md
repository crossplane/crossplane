# Webhooks in Crossplane

* Owner: Muvaffak Onuş (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Accepted

## Background

In Kubernetes, resource operations go through several steps of processes when
they are sent to `etcd` by the user. These steps include various compiled-in
admission controllers.

> An admission controller is a piece of code that intercepts requests to the
> Kubernetes API server prior to persistence of the object, but after the
> request is authenticated and authorized.

Additional to the ones that are compiled-in, Kubernetes allows users to register
their own webhook servers and API Server makes requests to those webhook servers
as needed. There are three main types of webhooks:
* Mutation: Accepts a resource, makes changes and returns it to API Server.
* Admission: Accepts a resource and returns back a decision whether the
  operation on that resource should be allowed.
* Conversion: Accepts a resource and a version string as destination version,
  then it creates the resource in the schema of that given version and returns
  it back to API Server.

The main difference between usual Kubernetes controllers is that admission
controllers work before the resource makes it to persistent storage. This allows
immediate rejection of the action performed by the users and various operations
that you want to do before persisting it and making it available for controllers
to reconcile.

In Crossplane, we frequently need webhooks for various use cases at different
levels. However, there needs to be additional mechanisms to enable providers and
Crossplane to register webhooks.

## Use Cases

Let's list all the known issues that can be solved by using webhooks and with
which operation type.

* Conversion webhooks for version changes in CRDs [#1584](https://github.com/crossplane/crossplane/issues/1584)
  * Conversion operation for **all kinds**
* Validate composition base templates [#1476](https://github.com/crossplane/crossplane/issues/1476)
  * Validation in `CREATE` operation of `Composition`
* Validate schemas in XRD [#1752](https://github.com/crossplane/crossplane/issues/1752)
  * Validation in `CREATE` operation of `CompositeResourceDefinition`
* Immutable resource fields [#727](https://github.com/crossplane/crossplane/issues/727)
  * Validation in `UPDATE` operation of **every managed resource**
  * The semantics in the KEP covering this is `can-be-set-only-in-creation`
    while what we want `cannot-be-changed-once-set`. So we can't really use
    upstream impl. when it's implemented.
* Composition scheduling [#967](https://github.com/crossplane/crossplane/issues/967)
  * Mutation in `CREATE` operation of `Composition`
* Support `oneOf` semantics validation for discriminator fields [#950](https://github.com/crossplane/crossplane/issues/950)
  * Implemented in Kubernetes, coming to controller-runtime.

## Implementation Requirements

As we can see, there are multiple use cases each requiring different levels of
customizations. While we want to streamline webhook implementation by providing
abstractions, we need to allow granular customizations to cover these cases. The
following list is roughly what we can start with as configurable:

* Mutation/Validation/Conversion webhook types.
* `CREATE` and `UPDATE` operation types.
* Static kind specification.
  * Dynamic kind specification would be useful for XRD-defined kinds, though
    it's a nice-to-have rather than a goal for now.

For the use cases that work with statically defined types like `Composition`, we
can use lower level abstractions if necessary since they are implemented only
once. So, we want to optimize our abstraction for the ones that require per-kind
implementaton: immutable fields, conversion webhooks.

### Immutable Fields

This problem can be solved by generating the function that will be called by the
admission webhook pipeline. The code generator in `crossplane/crossplane-tools`
can look for specific comment marker `// +immutable` and generate the function
that does the check.

### Conversion Webhooks

The main essence of conversion webhook problem is essentially field matching and
doing transforms in-between. These operations are not very complex but they are
also hard to generalize for every kind of CRDs we have. Specifically, when a new
version of provider API that has breaking changes is released, we can't really
guess how complex the conversion process will be. That's why we want to give
code-level flexibility to owners of the CRDs while keeping the webhook mechanics
that are generic be handled automatically by our abstraction layer.

## Upstream Tooling

There are two main mechanisms that upstream provides:
* Kubebuilder automatically generates webhook registration YAMLs for the marked
  structs.
  * Useful for only mutation and admission; conversion needs a patch on the CRD.
* `controller-runtime` provides two abstraction layers.
  * Low level `mgr.GetWebhookServer().Register(path string, hook http.Handler)`.
  * High level interfaces.
    * `Defaulter()` for mutation.
    * `Validate{Create,Update,Delete}() error` for admission.
    * `Convert{To,From}(obj)` for conversion.

Kubebuilder generated YAMLs would work for us; it's designed similar to CRD
generation and fairly simple. No additional YAML is needed for conversion but
CRD needs to point to the `Service` of webhook, though it's no-op if the CRD has
only one version.

## Proposal

There are four main pieces of a webhook:
* Actual Go implementation of the logic.
  * What will go into webhook functions, like `ValidateCreate(o runtime.Object) error`
* Having a `Service` object to be used by the webhook configurations.
* Mounting TLS key and certificate to Crossplane and provider `Deployment`s.
* Injecting TLS certificates to webhook configs and CRDs.
* Making sure the webhook configurations are registered on the Kubernetes API
  Server.

Let's look at each of these pieces in detail.

### Webhook Logic

In `crossplane-runtime`, we will accept a list of functions for each type of
the webhook and make sure they are called in given order whenever a request
hits. We will build this on top of the high level abstraction of
`controller-runtime` so that it's possible to append logic in different contexts
such as generated code and manually written code.

### Exposing Webhook Server

When a controller-runtime manager starts, the webhook server is also started
automatically. So, as long as necessary implementations are there and hooked up
with the main manager object, we have the server up and running. But in order to
expose it to cluster, we need to create a `Service` resource similar to
`Deployment` of the controller.

The package manager will create an opinionated `Service` resource for the
providers it installs and this will require Crossplane to have necessary RBAC
for managing `Service` objects, too. Though it could be limited to which
namespace it's installed, again similar to `Deployment` RBAC.

### Certificate Distribution

Kubernetes API server enforces use of TLS for the communication between the API
server and the webhook server. The webhook server needs to have the TLS Key &
Certificate and the API server needs to have a certificate bundle that is signed
by that key. Normally, the controllers in the wild have to include a mechanism
to generate the certificate to use in their webhook servers and that usually
renders a bad UX for the admins since it either requires manual creation of the
certificates or having `cert-manager` installed.

We will utilize the fact that we are orchestrating the installation of provider
controllers by our package manager. Crossplane will accept the TLS `Secret` as
input to its installation. Then in every provider installation, it will mount
this `Secret` to the provider container to use as the certificate & key. Since
it also installs the necessary YAMLs that register the webhooks to API Server,
it will inject the CA Bundle before the creation of those resources, which are
`MutatingWebhookConfiguration`, `ValidatingWebhookConfiguration` and the
`spec.webhook` part of `CustomResourceDefinition`s. 

While the provider installations will be handled this way, Crossplane doesn't
orchestrate the installation of itself. But we will again utilize the fact that
we have package manager, hence necessary RBAC to create the registration YAMLs.
So, what we will do is that Crossplane will register its own webhook
configurations right after it makes sure the TLS secret is there during the
initialization phase of the process, similar to how we deploy
`CustomResourceDefinition`s of Crossplane in its init container.

Since TLS `Secret` is required, no webhook functionality will be enabled if it's
not provided.

### Mutating/Validating Webhook Registration

The following excerpt includes examples of YAMLs necessary for mutation and
admission:
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mutation-config
webhooks:
  - admissionReviewVersions:
    - v1beta1
    name: mapplication.kb.io
    clientConfig:
      caBundle: ${CA_BUNDLE}
      service:
        name: webhook-service
        namespace: default
        path: /mutate
    rules:
      - apiGroups:
          - apps
      - apiVersions:
          - v1
        resources:
          - deployments
    sideEffects: None
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: "pod-policy.example.com"
webhooks:
- name: "pod-policy.example.com"
  rules:
  - apiGroups:   [""]
    apiVersions: ["v1"]
    operations:  ["CREATE"]
    resources:   ["pods"]
    scope:       "Namespaced"
  clientConfig:
    service:
      namespace: "example-namespace"
      name: "example-service"
    caBundle: ${CA_BUNDLE}
  admissionReviewVersions: ["v1", "v1beta1"]
  sideEffects: None
  timeoutSeconds: 5
```

The generation of these YAMLs can be handled using `kubebuilder`. The following
is an example marker we'd need to add to CRD struct:
```
// +kubebuilder:webhook:path=/mutate-batch-tutorial-kubebuilder-io-v1-cronjob,mutating=true,failurePolicy=fail,groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=create;update,versions=v1,name=mcronjob.kb.io
```

We will use this marker to generate these YAMLs similar to CRD YAMLs. Similarly,
they need to be included in the package artifact so a `webhooks` folder will be
added to the existing `package` folder which will look like the following:
```
.
├── crds
├── webhookconfigurations
└── crossplane.yaml
```

For core Crossplane webhook configurations; we will create them in Go code
during the initialization phase, inject CA Bundle and create as mentioned in the
earlier section.

### Conversion Webhook Registration

This case is different than other webhook registrations as it's not a separate
resource on its own but a field in the `CustomResourceDefinition`. Upstream
controller-runtime webhook server exposes only one path `/convert` for all
conversion operations. After the request comes in, it does a type check to
decide which conversion functions to run. The patching of
`CustomResourceDefinition` is left to client side tooling, i.e. no kubebuilder
markers available.

For provider CRDs, the package manager will inject the conversion webhook
configuration to every `CustomResourceDefinition` it installs because it's safe
to have it there even if there is only one `apiVersion` defined.

The Crossplane CRDs will be patched by Crossplane itself during the
initialization phase after ensuring TLS `Secret` is there. Since Crossplane
manages the lifecycle of its own CRDs using the init container, this should be
fine for both installation and upgrade scenarios because init container will be
done before the controller comes up and asks for the new version.


## Out of Scope

### Immutable Field Webhook

Currently, we mark the fields of CRDs as `// +immutable` but it doesn't have any
implication in practice. We can implement a code generator in
`crossplane-tools` that will generate functions for each of the marked fields
and register them with the list of validating webhooks. But this is left to
after webhook support is in place. For now, the implementations will be manual.
