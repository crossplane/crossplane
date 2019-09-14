---
title: "Crossplane Stacks Guide"
toc: true
weight: 331
indent: true
---

# Crossplane Stacks Guide


## Table of Contents

TODO


Introduction
Concepts
Before you get started
Install and Configure Crossplane
Install support for our application into Crossplane
Create a Wordpress
Clean up
Conclusion
Next steps
References

## Introduction

Welcome to the Crossplane Stack guide! In this document, we will:

* Learn how to install an existing stack
* Interact with a stack to see how to use it
* Glimpse what is possible with a stack
* Touch a little bit on how stacks work

We will **not**:

* Learn first principles (see the [concepts
  document][crossplane-concepts] for that level of detail)
* Develop our own stack from scratch (go to [this development
  guide][stack-developer-guide] to learn how to do that)

Let's go!

## Concepts

There are a bunch of things you might want to know to fully understand what's happening in this document. This guide won't cover them, but there are other ones that do. Here are some links!

* [Crossplane concepts][crossplane-concepts]
* [Kubernetes concepts][kubernetes-concepts]

## Before you get started

This guide assumes you are using a *nix-like environment. It also assumes you have a basic working familiarity with the following:

* The terminal environment
* Setting up cloud provider accounts for the cloud provider you want to
  use
* [Kubernetes][kubernetes-docs] and [kubectl][kubectl-docs]

You will need:

* A *nix-like environment
* A cloud provider account, for the cloud provider of your choice (out
  of the supported providers)
* A locally-configured kubectl which points to a configured Kubernetes
  cluster. We will put Crossplane in this cluster, and we'll refer to it
  as the control cluster.

## Install the Crossplane CLI

To interact with stacks, we're going to use the [Crossplane
CLI][crossplane-cli], because it's more convenient. To install it, we
can use the one-line curl bash:

```
RELEASE=0.0.1
curl -sL https://raw.githubusercontent.com/crossplaneio/crossplane-cli/"${RELEASE}"/bootstrap.sh | bash
```

To use the latest release, you can use `master` as the `RELEASE` instead
of using a specific version.

## Install and configure Crossplane

Outline
Install Crossplane
Install provider stack
Configure provider stack credentials
Configure resource classes

### Install Crossplane

The recommended way of installing Crossplane is by using
[helm][helm-install]. We can grab the most stable version currently
available by using:

```
helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

For more options for installing, including how to install a more
bleeding-edge version, or how to uninstall, see the [full install
documentation][crossplane-install-docs].

### Configure support for your cloud provider

Next we'll set up support for our cloud provider of choice! See the
provider-specific guides:

* [AWS][aws-setup]
* [GCP][gcp-setup]
* [Azure][azure-setup]

Then come back here! Don't worry; we'll still be here when you're ready.

Don't see your favorite cloud provider? [Help us add
support][provider-stack-developer-guide] for it!

## Install support for our application into Crossplane

Now that we've got Crossplane set up and configured to use a cloud
provider, we're ready to add support for creating WordPresses! We'll do
this using a Crossplane Stack. For more information about stacks, see
the [full Stack documentation][stack-docs].

We can use the [Crossplane CLI][crossplane-cli] to install our stack which adds support for
Wordpress:

```
kubectl crossplane stack install 'crossplane/sample-stack-wordpress:latest' 'sample-stack-wordpress'
```

This pulls the stack package from a registry to install it into
Crossplane. For more details about how to use the CLI, see the
[documentation for the CLI][crossplane-cli-docs]. For more details about how stacks work behind
the scenes, see the documentation about the [stack
manager][stack-manager-docs] and the [stack
format][stack-format-docs].

## Create a Wordpress

Now that Crossplane supports Wordpress creation, we can ask Crossplane
to spin up a Wordpress for us. We can do this by creating a Kubernetes
resource that our Wordpress stack will recognize:

```
cat > my-wordpress.yaml <<EOF
apiVersion: wordpress.samples.stacks.crossplane.io/v1alpha1
kind: WordpressInstance
metadata:
  name: my-wordpressinstance
EOF

kubectl apply -f my-wordpress.yaml
```

If the control cluster doesn't recognize the Wordpress instance type, it
could be because the stack is still being installed. Wait a few seconds,
and try creating the Wordpress instance again.

### Wait

The Wordpress can take a while to spin up, because behind the scenes
Crossplane is creating all of its dependendencies, which is a database
and Kubernetes cluster. To check the status, we can look at the
resources that Crossplane is creating for us:

```
# The claim for the database
kubectl get mysqlinstance
# The claim for the Kubernetes cluster
kubectl get kubernetescluster

# The workload definition
kubectl get kubernetesapplication
# The things created on the Kubernetes cluster as part of the workload
kubectl get kubernetesapplicationresource
```

For more information about how Crossplane manages databases and
Kubernetes clusters for us, see the more complete documentation about
[claims][claims-docs], [resource classes][resource-classes-docs], and
[workloads][workloads-docs].

### Use

Once everything has been created, the ip address for the Wordpress
instance will show up in the [Crossplane
KubernetesApplicationResource][kubernetesapplicationresource-docs]
which represents the workload's service. Here's a way to watch for the
ip:

```
kubectl get kubernetesapplicationresource -n wordpresses -o custom-columns='NAME:.metadata.name,NAMESPACE:.spec.template.metadata.namespace,KIND:.spec.template.kind,SERVICE-EXTERNAL-IP:.status.remote.loadBalancer.ingress[0].ip' --watch
```

The ip will show up on the one which has a `Service` kind.

If you navigate to the ip, you should see the Wordpress first-time
start-up screen in your browser.

If you see it, things are working!

## Clean up

When we want to get rid of everything, we can delete the Wordpress
instance and let Crossplane and Kubernetes clean up the rest. To read
more about how cleanup works, see the documentation on reclaim policies
in Crossplane and garbage collection in Kubernetes.

To delete the Wordpress instance:

```
kubectl delete wordpressinstance my-wordpressinstance
```

We can also remove the stack, using the Crossplane CLI:

```
kubectl crossplane stack uninstall sample-stack-wordpress
```

Removing the stack removes any Wordpress instances that were created.

The cloud provider stack can also be removed using the `kubectl
crossplane stack uninstall` command. Use `kubectl crossplane stack list`
to see what's installed.

## Conclusion

We're done! :tada:

In this guide, we:

* Set up Crossplane on a control cluster
* Installed functionality for a cloud provider
* Extended Crossplane to manage Wordpress workloads for us
* Created a Wordpress workload
* Got some initial exposure to some of the tools and concepts of
  Crossplane, Crossplane Stacks, and the Crossplane CLI

## Next steps

Crossplane can do a lot.

Now that we've gone through how to use a Crossplane Stack, you may want
to learn more about which stacks are available, or about how to write
your own stack.

To learn more about which stacks are available, check out the [stack registry][stack-registry].

To learn more about how to write your own stack, see the [stack developer
guide][stack-developer-guide].

## References

*   [The Crossplane Concepts guide][crossplane-concepts]
*   [The Stacks Concepts guide][stack-concepts]
*   [Crossplane Install Guide][crossplane-install-docs]
*   [The Crossplane CLI][crossplane-cli]
*   [Stacks Quick Start][stack-quick-start]
*   [Stack Registry][stack-registry]
*   [Stacks Developer Guide][stack-developer-guide]
*   [Provider Stack Developer Guide][provider-stack-developer-guide]
*   [AWS documentation][aws-docs]
*   [GCP documentation][gcp-docs]
*   [Azure documentation][azure-docs]
*   [Kubernetes documentation][kubernetes-docs]

## FAQ
These should probably be covered in the concepts document, but I am
adding them here for tracking purposes.

* How does Crossplane use the cloud provider? What's the model in general
for asking for a resource?
* How do Stacks work behind the scenes?
* What if I want to use an existing cluster, instead of spinning up a
new one?
* What if I want Crossplane to run in a control cluster which is also
running the rest of my Kubernetes stuff?
* Are there any known issues?

## TODO
For tracking purposes; not for inclusion in the document

* Remove things that assume you have the Crossplane repo checked out
* Figure out how to move the provider-specific stuff into different docs
  or have it be a selectable section
* Interactive docs?
  - Could it be a terminal program tutorial?
* Check to see whether provider stacks can be cleaned up completely by
  just uninstalling them
* Add a TOC

<!-- Named links -->
[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli
[crossplane-cli-docs]: https://github.com/crossplaneio/crossplane-cli/blob/master/README.md
[crossplane-concepts]: TODO
[crossplane-install-docs]: TODO

[kubernetesapplicationresource-docs]: TODO
[claims-docs]: TODO
[resource-classes-docs]: TODO
[workloads-docs]: TODO

[kubernetes-concepts]: https://kubernetes.io/docs/concepts/
[kubernetes-docs]: https://kubernetes.io/docs/home/
[kubectl-docs]: https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands

[helm-install]: https://github.com/helm/helm#install

[aws-docs]: https://docs.aws.amazon.com/
[gcp-docs]: https://cloud.google.com/docs/
[azure-docs]: https://docs.microsoft.com/azure/

[aws-setup]: TODO
[gcp-setup]: stacks-guide-gcp.html
[azure-setup]: TODO

[stack-docs]: TODO
[stack-quick-start]: TODO
[stack-concepts]: TODO
[stack-manager-docs]: TODO
[stack-format-docs]: TODO
[stack-registry]: TODO
[stack-developer-guide]: TODO
[provider-stack-developer-guide]: TODO
