---
title: Upgrading to v1.x
toc: true
weight: 220
indent: true
---

# Upgrading to v1.x

Crossplane versions post v1.0 do not introduce any breaking changes, but may
make some backward compatible changes to the core Crossplane CRDs. Helm [does
not currently touch CRDs](https://github.com/helm/helm/issues/6581) when a chart
is upgraded, so Crossplane has moved to [managing its own
CRDs](https://github.com/crossplane/crossplane/pull/2160) as of v1.2.0. However,
for versions prior to v1.2.0, you must manually apply the appropriate CRDs
before upgrading.

## Upgrading to v1.0.x or v1.1.x

To upgrade from the currently installed version, run:

```console
# Update to the latest CRDs.
kubectl apply -k https://github.com/crossplane/crossplane//cluster?ref=<release-branch>

# Update to the latest stable Helm chart for the desired version
helm --namespace crossplane-system upgrade crossplane crossplane-stable/crossplane --version <version>
```

## Upgrading to v1.2.x and Subsequent Versions

Since `v1.2.0`, we do not include any custom resource instances in our Helm chart.
However, if there are some existing ones, Helm will delete them. In order to keep
them, run the following commands to annotate all instances with Helm's special
annotation for this case:

```console
for name in $(kubectl get locks.pkg.crossplane.io -o name); do kubectl annotate $name 'helm.sh/resource-policy=keep'; done
for name in $(kubectl get providers.pkg.crossplane.io -o name); do kubectl annotate $name 'helm.sh/resource-policy=keep'; done
for name in $(kubectl get configurations.pkg.crossplane.io -o name); do kubectl annotate $name 'helm.sh/resource-policy=keep'; done
```

After annotations are in place you can upgrade from the currently installed version
by running:

```console
# Update to the latest stable Helm chart for the desired version
helm --namespace crossplane-system upgrade crossplane crossplane-stable/crossplane --version <version>
```
