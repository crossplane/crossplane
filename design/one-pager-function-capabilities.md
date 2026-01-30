# Function Runner Capability Advertisement

* Owner: Nic Cope (@negz)
* Reviewers: Jared Watts (@jbw976)
* Status: Accepted

## Background

Crossplane's function protocol has evolved since v1beta1. Features like required
resources, credentials, conditions, and required schemas were added in
subsequent releases. When a function uses one of these features with an older
version of Crossplane, Crossplane silently ignores the unknown fields. The
function has no way to know whether Crossplane will honor its request.

For example, a function that requests schemas via `requirements.schemas` can't
distinguish between "Crossplane fetched the schema but found nothing" and
"Crossplane doesn't understand schema requests at all".

## Proposal

Add a `Capability` enum and `capabilities` field to `RequestMeta`:

```protobuf
enum Capability {
  CAPABILITY_UNSPECIFIED = 0;
  CAPABILITY_CAPABILITIES = 1;       // v2.2
  CAPABILITY_REQUIRED_RESOURCES = 2; // v1.15
  CAPABILITY_CREDENTIALS = 3;        // v1.16
  CAPABILITY_CONDITIONS = 4;         // v1.17
  CAPABILITY_REQUIRED_SCHEMAS = 5;   // v2.2
}

message RequestMeta {
  string tag = 1;
  repeated Capability capabilities = 2;
}
```

Crossplane populates `capabilities` with all features it supports when calling
functions. Functions check for a capability before relying on the corresponding
feature, falling back gracefully when the capability is absent.

`CAPABILITY_CAPABILITIES` is the bootstrap capability. Its presence tells the
function that Crossplane advertises capabilities. If another capability is
absent, the function knows Crossplane doesn't support it - not that Crossplane
predates capability advertisement entirely.

## Usage

Functions check for a capability before relying on the corresponding feature.
The function SDKs will provide a `has_capability` helper. Usage in Python:

```python
async def RunFunction(
    self, req: fnv1.RunFunctionRequest, _: grpc.aio.ServicerContext
) -> fnv1.RunFunctionResponse:
    rsp = response.to(req)

    if request.has_capability(req, fnv1.CAPABILITY_REQUIRED_SCHEMAS):
        # Request the schema - Crossplane will populate it next iteration
        response.require_schema(rsp, "xr", "example.org/v1", "MyXR")

        schema = request.get_required_schema(req, "xr")
        if schema:
            # Use schema for validation
            pass
    else:
        # Crossplane doesn't support schemas - fall back or skip validation
        pass

    return rsp
```

Go and other languages work the same way.

## Alternatives Considered

### Repeated string

Use `repeated string capabilities` instead of an enum. This is more flexible -
Crossplane could add capabilities without a proto change. However, the
capabilities we're advertising are inherently tied to proto fields. You can't
use a new capability without updating to a proto that has the corresponding
field. Given this coupling, enum provides better type safety and documentation
with no practical downside.

### Protocol version

Add a `int32 protocol_version` field instead of listing capabilities. Functions
would need to know which version introduced which feature. This is less
self-documenting and harder to extend than explicit capability flags.

### gRPC server reflection

Use gRPC's server reflection API to let functions discover what fields exist.
This is complex - functions would need to query the reflection API and parse
protobuf descriptors to determine support. It also doesn't distinguish between
"field exists in proto" and "Crossplane actually implements this feature".

### Self-describing messages

Protobuf supports [self-describing messages][self-description] by embedding a
`FileDescriptorSet` in the message. Crossplane could include the proto schema
for `RunFunctionRequest` in each request. Functions could parse the descriptor
to discover what fields exist. This is heavyweight - descriptors add
significant message size - and like gRPC reflection doesn't distinguish between
"field exists" and "feature implemented".

[self-description]: https://protobuf.dev/programming-guides/techniques/#self-description

### Implicit detection via iteration

Functions could request a feature (e.g. schemas), then check whether Crossplane
populated the response on the next iteration. This works but requires an extra
round-trip, complicates function logic, and relies on subtle proto unknown field
preservation semantics.
