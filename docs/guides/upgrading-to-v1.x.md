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

To upgrade from the currently installed version, run:

```console
# Update to the latest stable Helm chart for the desired version
helm --namespace crossplane-system upgrade crossplane crossplane-stable/crossplane --version <version>
```
