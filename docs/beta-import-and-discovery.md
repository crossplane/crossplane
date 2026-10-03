# Beta: Resource Discovery and Import

This document covers the beta resource discovery and import feature, available via the `crossplane beta import discover` CLI command.

## Overview

Resource discovery helps operators identify external resources (e.g., cloud storage buckets, databases) that exist in their cloud account but are not yet managed by Crossplane. The import workflow then generates Crossplane manifests to adopt those resources.

## Workflow

### 1. Enable the Discovery Controller

Start the Crossplane controller with the alpha discovery flag:

```bash
crossplane --enable-alpha-resource-discovery
```

This activates the discovery controller, which watches DiscoveryReport resources.

### 2. Create a DiscoveryReport

Create a DiscoveryReport CRD to scan for resources of a specific kind. For example, to discover S3 buckets managed by the AWS provider:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: DiscoveryReport
metadata:
  name: discover-s3-buckets
spec:
  mrdRef:
    name: buckets.s3.aws.crossplane.io
  providerConfig:
    name: aws-default
  interval: 1h  # Scan every hour (optional)
```

The controller will:
- Contact the provider's ExternalLister interface
- Query all external resources of this kind
- Compare with existing Crossplane managed resources
- Update the DiscoveryReport status with findings

**Current Limitation:** Provider integration for discovery is not yet implemented. The controller will mark the DiscoveryReport as Unavailable until providers add support.

### 3. Query the DiscoveryReport

Once scanning is complete, the CLI can query the results:

```bash
crossplane beta import discover \
  --mrd buckets.s3.aws.crossplane.io
```

The CLI will:
- Fetch the DiscoveryReport by MRD name
- List unmanaged external resources
- Prompt for adoption candidates

### 4. Generate Adoption Manifests

The import command can automatically generate Crossplane manifests for selected resources:

```bash
crossplane beta import discover \
  --mrd buckets.s3.aws.crossplane.io \
  --auto-import=true
```

This generates YAML with adoption policies (Observe, LateInitialize) and management policies set to allow resource takeover.

## Current Status and Limitations

### What Works
- ✅ DiscoveryReport CRD creation and schema
- ✅ Controller registration and reconciliation loop
- ✅ CLI query of DiscoveryReport findings
- ✅ Basic manifest generation framework

### Known Limitations (Alpha)
- ❌ **Provider integration not yet implemented:** The discovery controller cannot yet call provider ExternalLister methods. This requires:
  1. Providers to implement the ExternalLister interface (being added to crossplane-runtime)
  2. Discovery controller to load and instantiate providers
  3. End-to-end testing of provider integration
- ❌ **Manual DiscoveryReport creation required:** Long-term, the controller will auto-create DiscoveryReports for all MRDs with providers that support discovery. For now, operators must manually create them.
- ❌ **No manifest schema validation:** Generated manifests have basic structure but may be incomplete for complex resources.
- ❌ **Single ProviderConfig per report:** The spec requires specifying a single ProviderConfig. Multi-account discovery is not yet supported.

## Next Steps

As the feature matures:
1. **Phase 2 (In Progress):** Implement provider integration (crossplane-runtime ExternalLister)
2. **Phase 3:** Auto-create DiscoveryReports for each MRD with a provider
3. **Phase 4:** Support multi-account discovery and batch imports
4. **Phase 5:** Graduate feature from alpha to beta/stable

## Troubleshooting

**DiscoveryReport shows Unavailable status:**
- Ensure the referenced MRD exists: `kubectl get mrd <mrd-name>`
- Check controller logs: `kubectl logs -n crossplane deployment/crossplane`
- Verify the referenced ProviderConfig exists and is healthy

**CLI command shows no resources found:**
- Run `kubectl get discoveryreport <name> -o yaml` to inspect the full status
- Check `lastDiscoveryTime` — if it's stale (>10 min old), trigger a manual reconcile by patching the DiscoveryReport
- Ensure the provider implementation supports ExternalLister (not all providers do yet)

**Generated manifests are incomplete:**
- This is expected in alpha. Field mappings are minimal; operators should review and complete the manifests before applying.
- File an issue with your provider if critical fields are missing.
