# Crossplane State Metrics

* Owner: Christopher Haar (@haarchri)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

As Crossplane adoption grows across organizations, users are building increasingly sophisticated platforms that manage thousands of cloud resources across multiple providers. This expanded usage has revealed significant observability gaps.

### Evolution of Crossplane Deployments

What started as infrastructure provisioning has evolved into:
- Full platform-as-a-service offerings
- Multi-tenant control planes serving entire organizations
- Complex composition chains with nested dependencies
- Mission-critical production workloads requiring strict SLOs

**Early Adopters:** Small teams managing dozens of resources found basic controller-runtime and crossplane-managed metrics sufficient.

**Growing Usage:** Organizations managing hundreds of resources across multiple teams began experiencing visibility gaps.

**Enterprise Scale:** Today's deployments managing thousands of resources across complex provider configurations demand sophisticated monitoring capabilities.

### The Observability Challenge

Modern platform teams face a unique challenge with Crossplane observability:

- **Scale Mismatch:** Platform teams operate control planes serving dozens of development teams, but metrics only show aggregates
- **Accountability Gap:** When resources fail, platform teams cannot quickly identify which team owns the failing resource
- **SLO Complexity:** Teams want to offer SLOs to their consumers but lack the granular metrics to measure them

**Real-world Scenario:**

A platform team manages a Crossplane control plane with 5000 AWS resources across 30 teams. The `crossplane_managed_resource_ready` metric shows that 47 S3 buckets are not ready. Without additional context, the platform team must:

1. Query the Kubernetes API to list all S3 buckets
2. Filter for those not ready
3. Check labels to identify owning teams
4. Manually notify affected teams

This process that could take 30+ minutes during an incident could be reduced to seconds with proper metrics.

### Current State of Crossplane Metrics

Crossplane provides metrics through multiple layers: https://docs.crossplane.io/latest/guides/metrics/
### The Cardinality Challenge

While Crossplane provides metrics, the managed resource metrics deliberately exclude resource names, claim names, or composite names as metric labels. This design decision prevents cardinality explosion but creates operational challenges.

Example:
```
crossplane_managed_resource_ready{
  gvk="s3.aws.upbound.io/v1beta1, Kind=Bucket",
  namespace="crossplane-system",
  pod="provider-aws-s3-56468ed6f1ee-557cb7bc69-7cggr"
} value=2
```

This tells us that 2 S3 buckets are ready, but not **which** buckets are ready.

**Why Cardinality Matters:**

In time series databases, every unique combination of metric name and labels creates a new time series. Adding resource names as labels would create:
- 1,000 S3 buckets x 7 condition types = 7,000 time series just for bucket conditions
- 50 resource types x 1,000 instances each x 7 conditions = 350,000 time series
- Adding team labels with 20 teams multiplies this further = 7,000,000 time series

With Crossplane 2.x and namespace-scoped CRDs, Managed Resources can be created in the same namespace as their parent XR, which could enable namespace-based attribution. However, this approach still faces cardinality challenges in high-scale control planes with many tenants, and legacy deployments or certain architectural patterns may still place Managed Resources at cluster scope.

### Current Ecosystem Solutions

**crossplane-contrib/x-metrics** (archived)

What it is: A community-contributed metrics exporter specifically built for Crossplane resources.

Current limitations:
- Inflexible configuration: Can only choose which resource types to monitor (like "all AWS resources" or "all Composites"), but can't select specific metrics
- All-or-nothing approach: No way to say "I only want to track the ready status" - you get everything or nothing
- Real-world impact: Teams end up with thousands of unnecessary metrics, driving up costs and making it harder to find useful data

Example configuration:
```yaml
apiVersion: metrics.crossplane.io/v1
kind: ClusterMetric
metadata:
  name: xr
spec:
  matchName: .platform.upbound.io
```

This generates metrics like:
```
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1 gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1 A metrics series for each object
aws_platform_upbound_io_XNetwork_v1alpha1{name="configuration-aws-network"} 1
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1_created gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1_created Unix creation timestamp
aws_platform_upbound_io_XNetwork_v1alpha1_created{name="configuration-aws-network"} 1.760604257e+09
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1_labels gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1_labels Labels from the kubernetes object
aws_platform_upbound_io_XNetwork_v1alpha1_labels{name="configuration-aws-network",label_crossplane_io_composite="configuration-aws-network"} 1
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1_info gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1_info A metrics series exposing parameters as labels
aws_platform_upbound_io_XNetwork_v1alpha1_info{name="configuration-aws-network"} 1
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1_ready gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1_ready A metrics series mapping the Ready status condition to a value (True=1,False=0,other=-1)
aws_platform_upbound_io_XNetwork_v1alpha1_ready{name="configuration-aws-network"} 0
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1_ready_time gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1_ready_time Unix timestamp of last ready change
aws_platform_upbound_io_XNetwork_v1alpha1_ready_time{name="configuration-aws-network"} 1.76060426e+09
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1_synced gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1_synced A metrics series mapping the Synced status condition to a value (True=1,False=0,other=-1)
aws_platform_upbound_io_XNetwork_v1alpha1_synced{name="configuration-aws-network"} 0
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1_synced_time gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1_synced_time Unix timestamp of last synced change
aws_platform_upbound_io_XNetwork_v1alpha1_synced_time{name="configuration-aws-network"} 1.760604263e+09
# TYPE aws_platform_upbound_io_XNetwork_v1alpha1_resource_count gauge
# HELP aws_platform_upbound_io_XNetwork_v1alpha1_resource_count A metrics series objects to count objects of aws_platform_upbound_io_XNetwork_v1alpha1
aws_platform_upbound_io_XNetwork_v1alpha1_resource_count 1
# TYPE x_metric_resources_count_total gauge
# HELP x_metric_resources_count_total A metric to count all resources
x_metric_resources_count_total 1
```

**kubernetes/kube-state-metrics**

What it is: The standard Kubernetes metrics exporter that can be configured to monitor custom resources.

Current limitations:
- configuration: All configuration goes in ConfigMaps; every change requires reloading the entire deployment
- No dynamic updates or API-based configuration
- Unreliable wildcard support: The wildcard filtering feature has been stuck in alpha for a long time, known to cause issues with large numbers of CRDs
- Built for standard Kubernetes resources, not hundreds of CRDs

Example Configuration:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-state-metrics-crd-config
  namespace: crossplane-system
data:
    kind: CustomResourceStateMetrics
    spec:
      resources:
        - groupVersionKind:
            group: platform.upbound.io
            version: "*"
            kind: "*"
          metricNamePrefix: crossplane
          labelsFromPath:
            managedResourceKind: [kind]
            claimKind: [metadata, ownerReferences, "[controller=true]", kind]
            cllaimName: [metadata, labels, crossplane.io/claim-name]
            claimNamespace: [metadata, labels, crossplane.io/claim-namespace]
          metrics:
            - name: "engine_version"
              help: "Engine Version"
              each:
                type: Info
                info:
                  labelsFromPath:
                    engineVersion: [spec, forProvider, engineVersion]
              commonLabels:
                custom_metric: "yes"
[...]
```

## Goals

### Preserve Simplicity

**Do NOT introduce granular metrics in Crossplane core, providers, or functions.**

Keep default metrics low-cardinality for all resource types:
- Managed Resources
- Namespaced XRs
- Cluster XRs
- Claims

Adding resource-specific labels to core metrics would force ALL Crossplane users to handle high cardinality, regardless of their scale or needs. This would increase memory usage, slow down queries, and raise operational costs for everyone. By keeping core metrics simple, we ensure Crossplane remains lightweight and performant by default.

### External Metrics System

Build a kube-state-metrics-like operator for Crossplane:
- Provide flexible configuration interface for diverse needs
- Support different isolation models (namespaces, labels, custom fields)
- Allow teams to define their own monitoring requirements

**Why this approach:**

Different organizations have vastly different requirements:
- A small team might only need basic resource counts
- An enterprise might need team attribution across 50+ teams
- A service provider might need customer-level isolation
- A platform team might need SLOs per application team

An external operator allows each organization to choose their trade-offs without impacting others.

## Proposal

We propose implementing a **State Metrics Operator for Crossplane**, a standalone controller that observes resources and exposes their state as configurable metrics.

This operator will act as the metrics translation layer between Crossplane and external monitoring systems. By moving advanced metrics handling out of Crossplane Core and into an external, configurable operator, we preserve Crossplane's simplicity while unlocking flexibility for organizations with diverse observability requirements.

### Architecture

The operator is deployed as an independent controller with its own RBAC, CRDs, and reconciliation loops.

**Core Components:**

1. **ClusterMetric Controller**
   - Watches ClusterMetric CRDs for configuration changes
   - Discovers and monitors cluster-scoped target resources (Providers, cluster XRs, etc.)
   - Extracts values and labels using JSONPath expressions
   - Registers Prometheus metrics based on ClusterMetric definitions
   - Tracks state transitions for counter metrics

2. **Metric Controller**
   - Watches Metric CRDs (namespace-scoped) for configuration changes
   - Discovers and monitors namespace-scoped target resources (XRs, Claims, etc.)
   - Same extraction and metric generation logic as ClusterMetric
   - Provides namespace-level isolation for metrics

3. **Prometheus Metrics Endpoint**
   - Exposes metrics at `/metrics` endpoint
   - Standard Prometheus scrape target
   - Serves both gauge (current state) and counter (state transitions) metrics
   - All metrics follow pattern: `crossplane_{metadata.name}` and `crossplane_{metadata.name}_total`

### API Design

#### Metric and ClusterMetric

The operator provides two CRD types for defining metrics:

- **Metric**: Namespace-scoped, for monitoring resources within a specific namespace
- **ClusterMetric**: Cluster-scoped, for monitoring cluster-wide resources

Both share the same core spec structure:

```yaml
apiVersion: metrics.crossplane.io/v1alpha1
kind: ClusterMetric
metadata:
  name: xnetwork-ready-status
spec:
  # Help text for the metric (Prometheus HELP)
  help: "XNetwork ready status (1=ready, 0=not ready)"

  # Reconciliation interval (default: 10m)
  interval: 1m

  # Target resource to monitor
  target:
    group: aws.platform.upbound.io
    kind: XNetwork        # Supports wildcard "*" for all kinds
    version: v1alpha1

  # Optional: Filter resources
  labelSelector: "team=platform"
  fieldSelector: "metadata.namespace=production"

  # Define metric values to extract (using JSONPath)
  metrics:
  - name: ready_status
    path: "status.conditions[?(@.type=='Ready')].status"

  # Define label dimensions (using JSONPath)
  labels:
  - name: resource_name
    path: "metadata.name"
  - name: region
    path: "spec.parameters.region"
  - name: team
    path: "metadata.labels['team']"
```

**Key API Features:**

1. **JSONPath Support**: Extract any field from the target resource using JSONPath expressions
2. **Wildcard Kind Monitoring**: Use `kind: "*"` to monitor all resource types in a group/version
3. **Flexible Filtering**: Label and field selectors to target specific resources
4. **Configurable Intervals**: Per-metric reconciliation intervals to balance freshness vs. load
5. **Automatic Metric Types**: Generates both gauge (current state) and counter (state transitions)

#### Example: Namespace-Scoped Metric

```yaml
apiVersion: metrics.crossplane.io/v1alpha1
kind: Metric
metadata:
  name: webapp-ready-status
  namespace: team-a
spec:
  help: "WebApp ready status per instance"
  interval: 1m
  target:
    group: platform.example.com
    kind: WebApp
    version: v1
  metrics:
  - name: ready_status
    path: "status.conditions[?(@.type=='Ready')].status"
  labels:
  - name: app_name
    path: "metadata.name"
  - name: team
    path: "metadata.labels['team']"
  labelSelector: "environment=production"
```

#### Example: Wildcard Kind Monitoring

Monitor all platform APIs with a single ClusterMetric:

```yaml
apiVersion: metrics.crossplane.io/v1alpha1
kind: ClusterMetric
metadata:
  name: team-apis-ready-count
spec:
  help: "Ready status of all platform APIs (1=ready, 0=not ready)"
  interval: 1m
  target:
    group: aws.platform.upbound.io
    kind: "*"              # Monitors ALL kinds in this group
    version: "*"           # Monitors ALL versions from this kinds
  metrics:
  - name: ready_status
    path: "status.conditions[?(@.type=='Ready')].status"
  labels:
  - name: status
    path: "status.conditions[?(@.type=='Ready')].status"
  # Automatically adds 'kind' label: XNetwork, XVPC, XDatabase, etc.
```

### Metrics Generated

For each ClusterMetric/Metric resource, the operator automatically generates **two metric types**:

#### 1. Gauge Metrics (Current State)

```promql
# Pattern: crossplane_{metadata.name}
crossplane_xnetwork_ready_status{resource_name="prod-vpc", region="us-west-2", network_id="vpc-123"} = 1

# Provider health example
crossplane_provider_health{provider_name="provider-aws", status="True"} = 1

# Resource count example
crossplane_xnetwork_resource_count{resource_name="prod-vpc", region="us-west-2"} = 15

# Wildcard kind monitoring
crossplane_team_apis_ready_count{kind="XNetwork", status="True"} = 42
crossplane_team_apis_ready_count{kind="XDatabase", status="True"} = 18
crossplane_team_apis_ready_count{kind="XVPC", status="False"} = 3
```

#### 2. Counter Metrics (State Transitions)

Counters track how many times a resource has transitioned between states:

```promql
# Pattern: crossplane_{metadata.name}_total
crossplane_xnetwork_ready_status_total{resource_name="prod-vpc", ready_status="True"} = 5
crossplane_xnetwork_ready_status_total{resource_name="prod-vpc", ready_status="False"} = 2

# Only increments when state changes (0→1 or 1→0)
```

**Note**: Counters are NOT generated for count-based metrics (like `resource_count`) since they measure quantities, not binary states.

### Example PromQL Queries

#### Basic Status Monitoring

```promql
# Count of ready XNetworks
sum(crossplane_xnetwork_ready_status == 1)

# Count of not-ready XNetworks
sum(crossplane_xnetwork_ready_status == 0)

# Alert: Specific resource is not ready
crossplane_xnetwork_ready_status{resource_name="prod-vpc"} == 0

# Alert: Any provider is unhealthy
crossplane_provider_health{status="False"} > 0
```

#### Multi-Team Attribution

```promql
# Ready count per team
sum by (team) (crossplane_xnetwork_ready_status == 1)

# Not-ready resources by team and region
sum by (team, region) (crossplane_xnetwork_ready_status == 0)

# Team-specific error rate
rate(crossplane_xnetwork_synced_status_total{team="data-eng", reason="ReconcileError"}[5m])
```

#### SLO Tracking

```promql
# SLO: 99% of XNetworks should be ready
(sum(crossplane_xnetwork_ready_status == 1) /
 count(crossplane_xnetwork_ready_status)) * 100 > 99

# Ready percentage per region
sum by (region) (crossplane_xnetwork_ready_status == 1) /
  count by (region) (crossplane_xnetwork_ready_status) * 100

# Alert: Team's SLO breach (ready < 95% for 10m)
(sum by (team) (crossplane_xnetwork_ready_status{team="platform"} == 1) /
 count by (team) (crossplane_xnetwork_ready_status{team="platform"}) * 100) < 95
```

#### Capacity Planning and Trends

```promql
# Total managed resources across all composites
sum(crossplane_xnetwork_resource_count)

# Average resources per composite by region
avg by (region) (crossplane_xnetwork_resource_count)

# Rate of resources becoming unhealthy
rate(crossplane_xnetwork_ready_status_total{ready_status="False"}[5m])
```

#### Wildcard Monitoring Queries

```promql
# Total ready resources across all platform API kinds
sum(crossplane_team_apis_ready_count{status="True"})

# Ready count per API kind
sum by (kind) (crossplane_team_apis_ready_count{status="True"})

# Which API kinds have failures
crossplane_team_apis_ready_count{status="False"} > 0

# Rate of resources becoming unhealthy per kind
rate(crossplane_team_apis_ready_count_total{status="False"}[5m])

# Ready percentage by API kind
sum by (kind) (crossplane_team_apis_ready_count{status="True"}) /
  count by (kind) (crossplane_team_apis_ready_count) * 100
```

#### Troubleshooting and Debugging

```promql
# Find networks with sync errors
crossplane_xnetwork_synced_status{reason="ReconcileError"} == 0

# Group sync failures by reason
sum by (reason) (crossplane_xnetwork_synced_status == 0)

# How many times has a resource flapped between ready/not-ready
sum(crossplane_xnetwork_ready_status_total{resource_name="prod-vpc"})

# Networks that have never been ready (counter = 0)
crossplane_xnetwork_ready_status_total{ready_status="True"} == 0
```

#### Cardinality Management

**User-Controlled Cardinality:**

- Users explicitly define which labels to include via `spec.labels[]`
- No labels are added by default (except `kind` for wildcard monitoring, `namespace` for namespace metrics)
- Users opt-in to cardinality by choosing dimensions
- labelSelector/fieldSelector reduce cardinality by filtering resources

**Example Cardinality Calculation:**

```yaml
# Low cardinality: Status-only monitoring
metrics:
- name: ready_status
  path: "status.conditions[?(@.type=='Ready')].status"

# Result: 2 time series per kind (True/False)

# High cardinality: Per-resource monitoring
metrics:
- name: ready_status
  path: "status.conditions[?(@.type=='Ready')].status"
labels:
- name: resource_name
  path: "metadata.name"
- name: team
  path: "metadata.labels['team']"
- name: region
  path: "spec.parameters.region"

# Result: N time series (one per unique resource_name+team+region combo)
```

Platform teams can choose their cardinality trade-offs based on their monitoring requirements and metrics backend capacity.

### Deployment Model

The Metrics Operator can be deployed:

**Standalone Installation:**
```bash
kubectl apply -f https://github.com/crossplane-contrib/crossplane-state-metrics/releases/latest/install.yaml
```

**Helm Chart:**
```bash
helm install crossplane-state-metrics crossplane-contrib/crossplane-state-metrics
```

## Future Considerations

1. **DataSink / ClusterDataSink Controllers:** Manage outbound connections to metrics backends (optional)

Enhance the Metrics and ClusterMetrics objects with a `dataSinkRef` field to enable pushing metrics to OTEL endpoints and other monitoring platforms:

```yaml
apiVersion: metrics.crossplane.io/v1alpha1
kind: ClusterDataSink
metadata:
  name: default
spec:
  connection:
    endpoint: http://prometheus-kube-prometheus-prometheus.monitoring:9090/api/v1/otlp/v1/metrics
    credentials: {}  # Support for authentication credentials
```

This would enable integration with enterprise monitoring systems like:
- DynaTrace
- DataDog
- New Relic
- Other SaaS monitoring platforms
- Custom OTEL collectors

2. **Remote Cluster Monitoring:** Support for collecting metrics from remote Kubernetes API endpoints

Enable monitoring resources across multiple clusters by introducing a `RemoteCluster` CRD and adding cluster references to Metrics/ClusterMetrics:

```yaml
apiVersion: metrics.crossplane.io/v1alpha1
kind: RemoteCluster
metadata:
  name: remote-prod-cluster
spec:
  # Reference to credentials for remote cluster access
  credentialsSecretRef:
    name: remote-cluster-kubeconfig
    namespace: crossplane-system
  # Optional: API server endpoint override
  endpoint: https://api.remote-cluster.example.com:6443
  # Optional: Connection health check interval
  healthCheckInterval: 30s
---
apiVersion: metrics.crossplane.io/v1alpha1
kind: ClusterMetric
metadata:
  name: remote-xnetwork-status
spec:
  # Reference to remote cluster
  remoteClusterRef:
    name: remote-prod-cluster

  help: "XNetwork ready status from remote cluster"
  target:
    group: aws.platform.upbound.io
    kind: XNetwork
    version: v1alpha1
  metrics:
  - name: ready_status
    path: "status.conditions[?(@.type=='Ready')].status"
  labels:
  - name: resource_name
    path: "metadata.name"
  - name: cluster
    value: "remote-prod"
```

This would support:
- Multi-cluster Crossplane deployments
- Hub-and-spoke architectures
- Centralized metrics collection from distributed control planes

## Prior Art

### Related Issues and Discussions

- https://docs.crossplane.io/latest/guides/metrics/
- https://grafana.com/blog/2022/10/20/how-to-manage-high-cardinality-metrics-in-prometheus-and-kubernetes/
- https://prometheus.io/docs/practices/instrumentation/#do-not-overuse-labels
- https://github.com/crossplane/crossplane/issues/4620
- https://github.com/crossplane/crossplane/issues/6238
- https://github.com/crossplane/crossplane/issues/5850
- https://github.com/crossplane/crossplane-runtime/issues/555
- https://github.com/crossplane/crossplane-runtime/issues/792
- https://github.com/crossplane/crossplane-runtime/issues/674
- https://github.com/kubernetes/kube-state-metrics
- https://github.com/crossplane-contrib/x-metrics

## Alternatives Considered

### Alternative 1: Add High-Cardinality Metrics to Core

**Approach:** Add resource names and other identifiers as labels to existing Crossplane metrics.

**Pros:**
- No additional components to deploy
- Immediate availability for all users

**Cons:**
- Forces all users to handle high cardinality
- Increased memory and CPU usage for everyone
- No flexibility for different organizational needs

**Decision:** Rejected. This would degrade performance for all users regardless of their needs.

### Alternative 2: Rely on External Tooling

**Approach:** Document how to use kube-state-metrics with Crossplane CRDs.

**Pros:**
- No development effort required
- Uses standard Kubernetes tooling

**Cons:**
- Poor user experience with hundreds of CRDs
- Complex configuration
- Not optimized for Crossplane's specific needs
- Not configurable with CRDs

**Decision:** Rejected. While this works for small deployments, it doesn't scale well for enterprise Crossplane usage.
