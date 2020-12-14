---
title: Upgrading to v1.0
toc: true
weight: 220
indent: true
---

# Upgrading to v1.0

Crossplane v1.0 doesn't introduce any breaking changes, but it does make some
backward compatible changes to the core Crossplane CRDs. Helm [does not
currently touch CRDs](https://github.com/helm/helm/issues/6581) when a chart is
upgraded, so you must apply them manually before upgrading. To upgrade from
v0.14, run:

```console
# Update to the latest CRDs.
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.0/cluster/charts/crossplane/crds

# Update to the v1.0 Helm chart
helm --namespace crossplane-system upgrade crossplane crossplane-stable/crossplane
```