# Beta: Resource Discovery and Import

This document covers the beta resource discovery and import feature, available via the `crossplane beta import discover` CLI command.

## Overview

Resource discovery identifies external resources (for example cloud storage buckets or DNS records) that exist in an account but are not yet managed by Crossplane. The import workflow then generates Crossplane manifests that adopt those resources.

Crossplane does not call providers to list resources. Providers list their own resources and publish the results as `ExternalInventory` objects. Crossplane reads those objects.

## Workflow

### 1. Providers publish an ExternalInventory

A provider writes one `ExternalInventory` per ManagedResourceDefinition, named after the MRD. It is cluster-scoped and provider-written.

- `spec.mrdRef` names the MRD that the inventory covers.
- `spec.providerConfigRef` names the ProviderConfig the provider listed with.
- `status.scopes` holds one entry per scope of the kind. An unscoped kind has a single entry with an empty scope.
  - `items` are the external resources found in that scope. Each item has an `externalName` and a `scope`.
  - `error` describes a scope whose listing failed.
  - `nextPageToken` is set while a scope is only partially listed.

A scope is a set of `spec.forProvider` fields that identify where a resource lives, such as a DNS zone, a repository, or a project. The same external name can appear in more than one scope.

### 2. Enable the Discovery Controller

Start the Crossplane controller with the alpha discovery flag:

```bash
crossplane --enable-alpha-resource-discovery
```

### 3. Create a DiscoveryReport

Create a DiscoveryReport for the kind you want to discover. For example, to discover S3 buckets managed by the AWS provider:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: DiscoveryReport
metadata:
  name: buckets.s3.aws.crossplane.io
spec:
  mrdRef:
    name: buckets.s3.aws.crossplane.io
  providerConfig:
    name: aws-default
  interval: 1h  # Recheck every hour (optional)
```

The controller reads the ExternalInventory with the same name as the MRD and records the resources that have no matching managed resource. A resource matches by external name and by every scope field. If the inventory was listed with a different ProviderConfig, or any scope failed to list, the report is marked Unavailable rather than reporting an incomplete result.

### 4. Query the DiscoveryReport

```bash
crossplane beta import discover \
  --kind Bucket \
  --group s3.aws.crossplane.io
```

The CLI reads the DiscoveryReport with the same name as the MRD. Add `--scope key=value` to limit the import to one scope. The flag is repeatable, and a resource must match every pair given:

```bash
crossplane beta import discover \
  --kind Record \
  --group dns.example.org \
  --scope zone=example.org
```

Without `--scope`, every unmanaged resource is included.

### 5. Generate Adoption Manifests

By default the command prints YAML. Each manifest adopts one resource with management policies `Observe` and `LateInitialize`, so the adopted resource is never deleted by Crossplane. Scope fields are written to `spec.forProvider`.

To apply the manifests:

```bash
crossplane beta import discover \
  --kind Bucket \
  --group s3.aws.crossplane.io \
  --dry-run=false \
  --auto-import
```

## Current Status and Limitations

### What Works
- ExternalInventory and DiscoveryReport types and their CRDs
- Controller reads ExternalInventory, compares it with managed resources by identity, and records unmanaged resources with their scopes
- CLI lists unmanaged resources, filters them by scope, and generates adoption manifests with scope fields

### Known Limitations (Alpha)
- **No provider publishes ExternalInventory yet.** Until a provider does, the report is Unavailable.
- **Manual DiscoveryReport creation required.** The controller does not yet create DiscoveryReports for each MRD.
- **No manifest schema validation.** Generated manifests are not validated against the target CRD schema before they are applied.
- **Single ProviderConfig per report.** Multi-account discovery is not yet supported.

## Next Steps

1. Providers publish ExternalInventory for their kinds.
2. Auto-create DiscoveryReports for each MRD with a provider that publishes inventories.
3. Support multi-account discovery and batch imports.
4. Graduate the feature from alpha to beta.

## Troubleshooting

**DiscoveryReport shows Unavailable status:**
- Check the condition message. A missing ExternalInventory means no provider has published a listing for this MRD.
- A message about a ProviderConfig mismatch means the inventory was listed with a different ProviderConfig than the report names.
- A message about failed scopes means one or more scopes could not be listed. Check the `error` of each scope in the ExternalInventory: `kubectl get externalinventory <name> -o yaml`.
- Ensure the referenced MRD exists: `kubectl get mrd <mrd-name>`.

**CLI command shows no resources found:**
- Run `kubectl get discoveryreport <name> -o yaml` to inspect the status.
- Check `lastDiscoveryTime`. If it is stale (more than 10 minutes old), the controller has not rescanned.
- Use `--scope` only with keys that the kind's scope uses. A scope key that no resource carries matches nothing.

**Generated manifests are incomplete:**
- This is expected in alpha. Field mappings are minimal, so review and complete the manifests before applying.
