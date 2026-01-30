# Required Schemas for Composition Functions

* Owner: Nic Cope (@negz)
* Reviewers: Jared Watts (@jbw976)
* Status: Accepted

## Background

Composition functions run as gRPC servers. They receive state via
`RunFunctionRequest` and return desired state via `RunFunctionResponse`. Per the
[function specification][1], functions cannot assume network access - they can't
call the Kubernetes API directly.

Functions that implement schema-aware DSLs need to know what fields exist on a
resource kind, and what type they are. For example, a function might warn when a
user references a field that doesn't exist, uses a string where an integer is
expected, or omits a required field.

A function could use required resources to fetch CRDs and extract their schemas.
This approach has two problems:

* CRD schemas are incomplete. They lack common fields like `metadata.name` and
  `metadata.labels` that are injected by the API server.
* Built-in types have no CRDs. There's no CRD to fetch for Pods, Deployments,
  or other core Kubernetes types.

The API server's OpenAPI endpoint solves both problems. It serves complete
schemas for all types - built-in and custom - including metadata fields.

## Prior Art: Required Resources

Crossplane already supports functions requesting arbitrary Kubernetes resources
via the "required resources" pattern, introduced in the [Extra Resources design
doc][2]:

1. Function returns `requirements.resources` specifying what resources it needs
2. Crossplane fetches those resources
3. Crossplane calls the function again with `required_resources` populated
4. This repeats until requirements stabilize

Required schemas extends this pattern to support schema requests.

## Proposal

Extend the function protocol to support schema requirements:

```protobuf
message Requirements {
  map<string, ResourceSelector> resources = 2;
  map<string, SchemaSelector> schemas = 3;  // NEW
}

message SchemaSelector {
  string api_version = 1;  // e.g., "example.org/v1"
  string kind = 2;         // e.g., "MyResource"
}

message Schema {
  optional google.protobuf.Struct openapi_v3 = 1;
}

message RunFunctionRequest {
  // ... existing fields ...
  map<string, Schema> required_schemas = 9;  // NEW
}
```

Functions request schemas by adding entries to `requirements.schemas`. The map
key uniquely identifies each request - the function uses the same key to look up
the schema in `required_schemas` on subsequent iterations.

If Crossplane can't find a schema (e.g., the GVK doesn't exist), it sets the map
key to an empty `Schema` message. This lets the function distinguish "Crossplane
tried but found nothing" from "Crossplane hasn't processed my request yet".

## Usage

The function SDKs provide helpers. Usage in Python:

```python
from crossplane.function import response, request
from crossplane.function.proto.v1 import run_function_pb2 as fnv1

import openapi_schema_validator as oapi

def compose(req: fnv1.RunFunctionRequest, rsp: fnv1.RunFunctionResponse):
    # Request a schema for Deployment.
    response.require_schema(rsp, "deployment", "apps/v1", "Deployment")

    # Check if we received the schema yet.
    schema = request.get_required_schema(req, "deployment")
    if schema:
        # Validate a desired Deployment against the schema.
        oapi.validate(desired_deployment, schema)
```

## Implementation

Crossplane fetches schemas from the Kubernetes API server's OpenAPI v3 endpoint.
The API server exposes schemas at `/openapi/v3`, with one document per
group-version containing schemas for all kinds in that GV.

The implementation:

1. Maps `apiVersion` to an OpenAPI path (e.g., `apps/v1` → `apis/apps/v1`)
2. Fetches the GV's OpenAPI document
3. Searches `components.schemas` for the requested kind using
   `x-kubernetes-group-version-kind` annotations
4. Returns the matching schema as a `google.protobuf.Struct`

Crossplane caches OpenAPI documents in memory and invalidates the cache when
CRDs change. This keeps schema fetching fast without serving stale data when
providers install new CRDs.

## Alternatives Considered

### Embedded Schemas

Crossplane also supports embedding schemas in function packages at build time.
The [Developer Experience Tooling design][3] describes how the tooling generates
language bindings (e.g., Python classes, Go structs) from CRDs and XRDs. This
provides IDE autocomplete, type safety, and build-time validation.

Embedded schemas are useful when function logic is baked in at build time - the
function author knows what types they're working with. Required schemas are
useful when function logic is provided dynamically at runtime, for example via
Composition input like function-kcl or function-python. These functions are
generic runtimes that can't know at build time what types users will reference.


| Approach | When to use |
|----------|-------------|
| Embedded | Function logic is baked in at build time |
| Required | Function logic is provided dynamically at runtime |


[1]: https://github.com/crossplane/crossplane/blob/main/contributing/specifications/functions.md
[2]: design-doc-composition-functions-extra-resources.md
[3]: https://github.com/crossplane/crossplane/pull/6909
