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
is upgraded, so you must apply them manually before upgrading. To upgrade from
the currently installed version, run:

```console
# Update to the latest CRDs.
kubectl apply -k https://github.com/crossplane/crossplane//cluster?ref=release-1.1

# Update to the latest stable Helm chart
helm --namespace crossplane-system upgrade crossplane crossplane-stable/crossplane
```