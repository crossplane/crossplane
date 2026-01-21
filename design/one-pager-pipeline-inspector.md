# Function Pipeline Inspector

* Owner: [Philippe Scorsolini] (@phisco)
* Reviewers: Crossplane Maintainers
* Status: Draft, revision 0.1

## Background

Crossplane Compositions are defined as pipelines—a sequence of Functions called one after
another, where each function receives the previous step's output combined with the observed
state, and can modify the desired state, emit events, or return an error. If no failures occur,
Crossplane applies the resulting desired state to both the composite resource and all composed
resources.

While this pipeline-based model is powerful and flexible, it presents significant challenges for
debugging and observability. When something goes wrong—or behaves unexpectedly—platform
engineers have limited visibility into what actually happened during pipeline execution.

### Current State

Today, users rely on a combination of tools to debug composition pipelines:

- **`crossplane render`**: Enables local rendering and testing of compositions before
  deployment. Useful during development but cannot capture real-world behavior with actual
  observed state from a live cluster.

- **`crossplane beta trace`**: Traces resource relationships to understand the hierarchy of
  composite and composed resources. Helps with understanding "what exists" but not "how it got
  there."

- **Function outputs (logs, events, conditions)**: Functions can emit logs, report events on the
  composite resource, and set conditions. While these provide some visibility into function
  behavior, correlating this information across multiple functions in a pipeline, understanding
  the data flow between them, and reconstructing the full sequence of events is manual and
  error-prone.

### Pain Points

1. **Debugging failures**: When a pipeline fails, users cannot easily determine which function failed or what input caused the failure.

2. **Understanding state evolution**: Users cannot see how the desired state transforms as it passes through each function in the pipeline.

3. **Inspecting function inputs and outputs**: There is no way to see the actual `RunFunctionRequest` and `RunFunctionResponse` data that flows through the pipeline during a live reconciliation.

4. **Tracing composed resources**: Users struggle to understand why a specific composed resource was or wasn't created, modified, or deleted.

## Goals

- Provide a mechanism to capture `RunFunctionRequest` and `RunFunctionResponse` data for pipeline reconciliations.
- Enable the capture of data for all reconciles, including cache hits.
- Expose captured data via a configurable interface that downstream consumers can implement.
- Provide a minimal reference implementation that logs pipeline data to stdout.
- Allow consumers to correlate steps in a same pipeline.

### Non-Goals

- Providing storage, querying, or visualization of pipeline data (left to downstream implementations).
- Replacing local development workflows (`crossplane render` remains the tool for local testing).
- Providing an audit log or compliance record.
- CLI access to pipeline data.

## Proposal

This design introduces a hook in the Crossplane core that emits `RunFunctionRequest` and `RunFunctionResponse` data for each function invocation. This follows the pattern established by [Change Logs](https://github.com/crossplane/crossplane/blob/main/design/one-pager-change-logs.md): minimal upstream hooks with a reference implementation, enabling downstream commercial or community implementations to build richer functionality.

### Architecture Overview

```mermaid
flowchart TB
    subgraph cluster["Kubernetes Cluster"]
        subgraph xp_pod["Crossplane Pod"]
            subgraph xp_core["Crossplane Core"]
                wrapper["FunctionRunner Wrapper<br/>(emits req/res)"]
                cache["Cached FunctionRunner"]
                wrapper --> cache
            end
            subgraph sidecar["Pipeline Inspector Sidecar"]
                sidecar_impl["Reference implementation:<br/>logs to stdout as JSON"]
            end
            wrapper -->|"gRPC<br/>(Unix Socket)"| sidecar_impl
        end

        subgraph functions["Function Pods"]
            fn1["function-patch-and-transform"]
            fn2["function-go-templating"]
            fn3["..."]
        end

        cache -->|"gRPC<br/>(mTLS)"| functions
    end
```

### FunctionRunner Wrapper

A new `FunctionRunner` wrapper that emits pipeline execution data after each function call. This follows the existing pattern in Crossplane where `FunctionRunner` implementations can be composed (e.g., the caching `FunctionRunner` wraps the base `PackagedFunctionRunner`).

The implementation is organized into the following packages:

- `internal/xfn/inspected/`: Contains the `Runner` wrapper, `SocketPipelineInspector`, and metrics
- `internal/controller/apiextensions/composite/step/`: Contains `Metadata` struct and `BuildStepMeta()` function

```go
// A PipelineInspector inspects function pipeline execution data.
type PipelineInspector interface {
    // EmitRequest emits the given request and metadata.
    EmitRequest(ctx context.Context, req *fnv1.RunFunctionRequest, meta *step.Metadata) error

    // EmitResponse emits the given response, error, and metadata.
    EmitResponse(ctx context.Context, rsp *fnv1.RunFunctionResponse, err error, meta *step.Metadata) error
}

// A Runner wraps another FunctionRunner, emitting request and response data to
// a PipelineInspector for debugging and observability.
type Runner struct {
    wrapped   FunctionRunner
    inspector PipelineInspector
    metrics   Metrics
    log       logging.Logger
}

func (r *Runner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
    // Extract metadata from context and request
    meta, err := step.BuildMetadata(ctx, name, req)
    if err != nil {
        r.log.Info("failed to extract step metadata, skipping inspection", "function", name, "error", err)
        // Skip inspection if we can't build metadata, but proceed with function execution.
        return r.wrapped.RunFunction(ctx, name, req)
    }

    // Emit request before execution
    if err := r.inspector.EmitRequest(ctx, req, meta); err != nil {
        r.metrics.ErrorOnRequest(name)
        r.log.Info("failed to inspect request for function", "function", name, "error", err)
    }

    // Run the wrapped function
    rsp, err := r.wrapped.RunFunction(ctx, name, req)

    // Emit response after execution
    if err := r.inspector.EmitResponse(ctx, rsp, err, meta); err != nil {
        r.metrics.ErrorOnResponse(name)
        r.log.Info("failed to inspect response for function", "function", name, "error", err)
    }

    return rsp, err
}

```

#### Context Injection by FunctionComposer

The `FunctionComposer` is responsible for injecting the step metadata into the context before calling each function. This happens in `composition_functions.go`:

```go
// Before the pipeline loop starts, generate a trace_id for the entire reconciliation
traceID := uuid.NewString()

// For each step in the pipeline...
for stepIndex, fn := range req.Revision.Spec.Pipeline {
    // ...

    // Inject step metadata into context
    stepCtx := step.ContextWithStepMeta(ctx, traceID, stepIndex, compositionName, 0)

    // Call the function with the enriched context
    rsp, err := c.pipeline.RunFunction(stepCtx, fn.FunctionRef.Name, fnreq)
}
```

The `Runner` wrapper then extracts this metadata using `step.BuildMetadata()`:

```go
// BuildMetadata builds Metadata from the given context and function request.
func BuildMetadata(ctx context.Context, functionName string, req *fnv1.RunFunctionRequest) (*Metadata, error) {
    meta := Metadata{
        FunctionName: functionName,
        Timestamp:    time.Now(),
    }

    // Extract trace_id, step index, composition name, and iteration from context
    // (injected by FunctionComposer via ContextWithStepMeta)
    if v, ok := ctx.Value(ContextKeyTraceID).(string); ok {
        meta.TraceID = v
    } else {
        return nil, fmt.Errorf("could not extract trace ID from context")
    }
    if v, ok := ctx.Value(ContextKeyStepIndex).(int); ok {
        meta.StepIndex = int32(v)
    } else {
        return nil, fmt.Errorf("could not extract step index from context")
    }
    if v, ok := ctx.Value(ContextKeyCompositionName).(string); ok {
        meta.CompositionName = v
    } else {
        return nil, fmt.Errorf("could not extract composition name from context")
    }
    if v, ok := ctx.Value(ContextKeyIteration).(int32); ok {
        meta.Iteration = v
    }

    // Generate a unique span_id for this function invocation
    meta.SpanID = uuid.NewString()

    // Extract XR metadata from the request
    xr := req.GetObserved().GetComposite().GetResource()
    if xr != nil {
        meta.CompositeResourceAPIVersion = getStringField(xr, "apiVersion")
        meta.CompositeResourceKind = getStringField(xr, "kind")
        if metadata := getStructField(xr, "metadata"); metadata != nil {
            meta.CompositeResourceName = getStringField(metadata, "name")
            meta.CompositeResourceNamespace = getStringField(metadata, "namespace")
            meta.CompositeResourceUID = getStringField(metadata, "uid")
        }
    }

    return &meta, nil
}
```

The `FetchingFunctionRunner` (in `internal/xfn/required_resources.go`) updates the iteration counter when a function requests additional resources and needs to be re-run:

```go
// FetchingFunctionRunner re-runs functions that request additional resources
for i := int32(0); i <= MaxRequirementsIterations; i++ {
    // Update the iteration counter in the context for downstream components.
    iterCtx := step.ContextWithStepIteration(ctx, i)

    rsp, err := c.wrapped.RunFunction(iterCtx, name, req)
    // ...
}
```

This design separates concerns:
- **FunctionComposer** generates the `trace_id` and injects initial context values (`trace_id`, `step_index`, `composition_name`, `iteration=0`)
- **FetchingFunctionRunner** updates the `iteration` counter when re-running functions that request additional resources
- **Runner** extracts all context values along with request data to build complete metadata for emission

### gRPC Interface

For the sidecar implementations, we define a gRPC service:

```protobuf
syntax = "proto3";

package crossplane.pipeline.v1alpha1;

import "google/protobuf/timestamp.proto";

// PipelineInspectorService receives pipeline execution data from Crossplane.
service PipelineInspectorService {
    // EmitRequest receives the function request before execution.
    // Errors do not affect pipeline execution.
    rpc EmitRequest(EmitRequestRequest) returns (EmitRequestResponse);

    // EmitResponse receives the function response after execution.
    // Errors do not affect pipeline execution.
    rpc EmitResponse(EmitResponseRequest) returns (EmitResponseResponse);
}

message EmitRequestRequest {
    // The original function request as JSON bytes (with credentials stripped).
    // This allows consumers to parse the request without needing the proto schema.
    bytes request = 1;

    // Metadata for correlation and identification.
    StepMeta meta = 2;
}

message EmitRequestResponse {
    // Empty - fire and forget.
}

message EmitResponseRequest {
    // The function response as JSON bytes (nil if function call failed).
    // This allows consumers to parse the response without needing the proto schema.
    bytes response = 1;

    // Error message if the function call failed.
    string error = 2;

    // Metadata for correlation and identification.
    // Must match the meta from the corresponding EmitRequest.
    StepMeta meta = 3;
}

message EmitResponseResponse {
    // Empty - fire and forget.
}

message StepMeta {
    // UUID identifying the entire pipeline execution (all steps in one reconciliation).
    string trace_id = 1;

    // UUID identifying this specific function invocation.
    string span_id = 2;

    // Zero-based index of this step in the function pipeline.
    int32 step_index = 3;

    // Per-step counter incremented when a function requests additional resources and
    // needs to be re-run, starting from 0.
    int32 iteration = 4;

    string function_name = 5;
    string composition_name = 6;
    string composite_resource_uid = 7;
    string composite_resource_name = 8;
    string composite_resource_namespace = 9;
    string composite_resource_api_version = 10;
    string composite_resource_kind = 11;
    google.protobuf.Timestamp timestamp = 12;
}
```

By using JSON bytes for the request and response payloads, consumers can parse the data without needing the function proto schema. This decouples the sidecar implementation from the Crossplane proto definitions entirely, making it simpler to build and maintain. Each message still stays within the default 4MB gRPC limit by splitting request and response into separate RPC calls. The `StepMeta` is included in both calls to allow the sidecar to correlate them using `trace_id`, `span_id`, `step_index`, and `composite_resource_uid`.

### Security: Credential Stripping

The `RunFunctionRequest` includes `credentials` and connection details fields that may contain
sensitive data. **These fields must be cleared before emission.**

Additionally, if any composed resource (observed or desired) is a Secret, its `data` field must
also be cleared before emission. The `context` field is passed through as-is, since functions
already have access to it and it typically contains non-sensitive configuration data.

```go
func (e *SocketPipelineInspector) EmitRequest(ctx context.Context, req *fnv1.RunFunctionRequest, meta StepMeta) {
    // Strip sensitive data
    sanitizedReq := proto.Clone(req).(*fnv1.RunFunctionRequest)
    sanitizedReq.Credentials = nil

    // Sanitize observed resources
    sanitizedReq.GetObserved().GetComposite().GetResource().ConnectionDetails = nil
    for _, cr := range sanitizedReq.GetObserved().GetResources() {
        r := cr.GetResource()
        r.ConnectionDetails = nil
        // if it's a Secret, drop data too
        // ...
    }

    // Sanitize desired resources
    sanitizedReq.GetDesired().GetComposite().GetResource().ConnectionDetails = nil
    for _, cr := range sanitizedReq.GetDesired().GetResources() {
        r := cr.GetResource()
        r.ConnectionDetails = nil
        // if it's a Secret, drop data too
        // ...
    }

    // Serialize the request to JSON bytes
    reqBytes, err := protojson.Marshal(sanitizedReq)
    if err != nil {
        e.log.Debug("Failed to marshal pipeline request", "error", err, "function", meta.FunctionName)
        return
    }

    ctx, cancel := context.WithTimeout(ctx, e.timeout)
    defer cancel()

    _, err = e.client.EmitRequest(ctx, &pipelinev1alpha1.EmitRequestRequest{
        Request: reqBytes,
        Meta:    toProtoMeta(meta),
    })
    if err != nil {
        e.log.Debug("Failed to emit pipeline request", "error", err, "function", meta.FunctionName)
    }
}
```

### Fail-Open Behavior

Since this feature is for debugging rather than security auditing or compliance, the system must **fail-open**: if the
sidecar or emitter is unavailable, pipeline execution continues and data is simply not captured. This ensures the
debugging feature cannot negatively impact production workloads.

The default emit timeout is 100ms. If the sidecar doesn't respond within this time, the emit fails and pipeline execution continues.

To allow monitoring the health of the pipeline inspector, the following Prometheus metric is exposed:

- `function_run_function_pipeline_inspector_errors_total`: Counter for errors encountered emitting request/response data, with labels `function_name` and `type` (request/response).

### Configuration

The feature is enabled via the `--enable-pipeline-inspector` flag and configured with a socket path:

```yaml
# Crossplane deployment args
args:
  - --enable-pipeline-inspector
  - --pipeline-inspector-socket=/var/run/pipeline-inspector/socket  # default value
```

The socket path can also be configured via the `PIPELINE_INSPECTOR_SOCKET` environment variable.

When the feature flag is not set, the `Runner` wrapper is not instantiated and there is zero overhead.

See [Helm Chart Changes](#helm-chart-changes) for the recommended configuration approach.

### Reference Implementation: Sidecar

A minimal reference sidecar implementation will be provided in a separate repository (following the pattern of [crossplane/changelogs-sidecar](https://github.com/crossplane/changelogs-sidecar)). The reference implementation:

1. Implements the `PipelineInspectorService` gRPC interface
2. Listens on a Unix domain socket
3. Logs received data to stdout as JSON
4. Accepts `--max-recv-msg-size` flag for configuring gRPC message size limits

This reference implementation is intentionally minimal. It demonstrates the interface and can be used for basic
debugging, but more sophisticated implementations (with storage, querying, visualization) are left to downstream
consumers.

### Correlation

Pipeline Inspector provides metadata for correlating function invocations:

- **`trace_id`**: A UUID identifying the entire pipeline execution. All function invocations within a single reconciliation share the same `trace_id`. Generated by `FunctionComposer` at the start of each reconciliation.

- **`span_id`**: A UUID generated for each function invocation. This uniquely identifies each step in the pipeline.

- **`step_index`**: The zero-based index of this step in the function pipeline. Indicates the order of execution.

- **`iteration`**: Per-step counter incremented when a function requests additional resources and
  needs to be re-run, starting from 0.

- **`composite_resource_uid`**: The UID of the composite resource being reconciled. It can be
  used to group all function calls for the same resource.

**Naming Convention**: The `trace_id` and `span_id` naming
follows [OpenTelemetry (OTEL) conventions](https://opentelemetry.io/docs/concepts/signals/traces/). This intentional
alignment enables a future migration path: when Crossplane introduces OTEL tracing instrumentation, these fields can be
replaced with proper OTEL trace and span IDs, allowing seamless integration with distributed tracing backends (Jaeger,
Tempo, etc.) while maintaining backward compatibility for consumers already using these fields for correlation.

These fields allow downstream consumers to:
- Reconstruct the full pipeline execution sequence
- Correlate requests with their corresponding responses
- Group all steps within a single reconciliation using `trace_id`
- Track retries of the same step

## Data Volume Considerations

Each `RunFunctionRequest` and `RunFunctionResponse` can individually be up to 4MB (the default `MaxRecvMessageSize`). By splitting request and response into separate RPC calls, each message stays within the default gRPC limit. Optimizations at the storage layer are the responsibility of downstream implementations.

### gRPC Message Size Limits

By default, each `EmitRequest` and `EmitResponse` call stays within the 4MB gRPC limit. However, users who have increased the `--max-recv-msg-size` on their functions may have larger payloads.

**Sidecar configuration**: The reference sidecar (and any downstream implementations) should accept a `--max-recv-msg-size` flag to handle larger payloads if needed.

### Helm Chart Changes

The upstream Crossplane Helm chart has been extended to support sidecar containers via [PR #7007](https://github.com/crossplane/crossplane/pull/7007), which adds a `sidecarsCrossplane` value. This feature enables injecting additional containers into the Crossplane deployment pod.

Example `values.yaml` configuration for Pipeline Inspector:

```yaml
# Enable the pipeline inspector feature flag
args:
  - --enable-pipeline-inspector
  - --pipeline-inspector-socket=/var/run/pipeline-inspector/socket

# Inject the pipeline inspector sidecar
sidecarsCrossplane:
  - name: pipeline-inspector
    image: xpkg.crossplane.io/crossplane/pipeline-inspector-sidecar:v0.1.0
    args:
      - --socket=/var/run/pipeline-inspector/socket
      - --max-recv-msg-size=8388608  # 8MB
    volumeMounts:
      - name: pipeline-inspector-socket
        mountPath: /var/run/pipeline-inspector
    resources:
      requests:
        cpu: 10m
        memory: 64Mi
      limits:
        cpu: 100m
        memory: 128Mi

# Add the shared volume for Unix socket communication
extraVolumes:
  - name: pipeline-inspector-socket
    emptyDir: {}

extraVolumeMountsCrossplane:
  - name: pipeline-inspector-socket
    mountPath: /var/run/pipeline-inspector
```

This approach uses the generic sidecar injection mechanism rather than a feature-specific configuration block, keeping the Helm chart simple and reusable for other sidecar use cases.

## Alternatives Considered

### OpenTelemetry (OTEL) for Data Transport

We considered using OTEL instrumentation to emit spans containing the full pipeline data (request/response payloads).

**Pros:**
- Industry-standard observability format
- Existing ecosystem and tooling

**Cons:**
- Span attributes have size limits (Jaeger: 65KB, Tempo: 2KB per attribute) far below the 4MB per request/response we need
- Would require emitting as correlated logs, significantly increasing log volume for all users

**Conclusion**: OTEL is not suitable for transporting the full pipeline data. Instead, we generate our own `trace_id` and `span_id` UUIDs and use `iteration` counters for correlation. The naming follows OTEL conventions to enable future integration when Crossplane adds OTEL tracing instrumentation (see [Correlation](#correlation)).

### Proxy-Based Approaches

Multiple proxy-based approaches were evaluated (HTTP proxy, service rewriting, function sidecar injection). All failed due to a fundamental limitation: **Crossplane caches function responses**. When a cached response is used, no function call occurs and the proxy sees nothing.

The wrapper approach addresses this by capturing data before the cache is consulted.

### Modifying Functions Directly

Instrumenting functions themselves (as demonstrated by [Apple at KubeCon NA 2025][apple-kubecon-na-2025]) was considered but rejected because:
- We do not control function implementations
- Would require changes to every function
- Does not address cache behavior

## Implementation Plan

### Phase 1: Core Hook (This Proposal)

1. Define the `PipelineInspector` interface
2. Implement the `InspectingFunctionRunner` wrapper
3. Implement the `SocketPipelineInspector` gRPC client
4. Define the gRPC protobuf interface (with `trace_id`, `span_id`, and `iteration` in `StepMeta`)
5. Add feature flag and configuration
6. Implement a `NopPipelineInspector` for when the feature is disabled

### Phase 2: Reference Sidecar (Separate Repository)

1. Create `crossplane/pipeline-inspector-sidecar` repository
2. Implement minimal gRPC server with configurable `--max-recv-msg-size`
3. Log received data as JSON to stdout
4. Publish container images

### Phase 3: Documentation

1. Document the feature flag and Helm values configuration
2. Document the gRPC interface for downstream implementers

[apple-kubecon-na-2025]: https://youtu.be/g70y40Qk7bs?si=MpAwmKrDPo_mAvL0
