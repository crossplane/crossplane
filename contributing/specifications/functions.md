# Composition Functions Specification

A Composition Function is a kind of Crossplane extension. A Function is called
to instruct Crossplane how a Composite Resource (XR) should be composed of other
resources. This document specifies how a Function MUST behave. Refer to the
Composition Functions [design document] for broader context on the feature.

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED",  "MAY", and "OPTIONAL" in this document are
to be interpreted as described in [RFC 2119].

## Serving Requests from Crossplane

A Function MUST implement a gRPC `FunctionRunnerService` server. A Function
SHOULD implement the latest available version of this service - e.g. `v1beta1`.
The authoritative definition of this service can be found at the following URL.

https://github.com/crossplane/crossplane/tree/master/apis/apiextensions/fn/proto

A Function MUST copy the tag field from a RunFunctionRequest's RequestMeta
message to the ResponseMeta tag field of the corresponding RunFunctionResponse.
A Function MUST NOT modify the tag field. A Function MUST NOT inspect or
otherwise depend on the tag's value.

A Function SHOULD specify the ttl field of each RunFunctionResponse. The value
of the ttl field MUST reflect how long Crossplane may use a cached version of
the response for an identical RunFunctionRequest.

A Function MUST pass through any desired state for which it does not have an
opinion from the RunFunctionRequest desired field to the corresponding
RunFunctionResponse desired field. A Function MAY mutate any desired state for
which it has an opinion, including adding to, updating, or deleting from the
desired state. For example a Function may:

* Add a new composed resource to the desired state.
* Add a new field to an existing composed resource in the desired state.
* Remove a composed resource from the desired state.
* etc...

A Function MUST do so _intentionally_. Put otherwise, remember that Functions
are run in a pipeline. A Function MUST propagate the
desired state passed to it by previous Functions.

A Function MUST NOT specify desired composite resource spec or metadata fields.
A Function MAY only specify desired composite resource status fields.

A Function MUST NOT specify desired composed resource status fields. A Function
MAY only specify non-status desired composed resource fields, for example fields
under the top-level metadata, spec, or data fields.

A Function SHOULD NOT specify a metadata.name for desired composed resources.
Crossplane will generate an appropriate name. A Function SHOULD specify the
crossplane.io/external-name annotation for its desired composed resources in
order to influence the external name of those resources.

A Function SHOULD avoid conflicting with existing observed or desired composed
resources, for example by attempting to add an entry to the desired composed
resources array with a name that already exists in either that array or the
observed composed resources array.

A Function MUST only return a Fatal result if it intends to terminate the entire
Function pipeline, including preventing any Functions that would normally run
after it from running.

A Function MUST NOT return a Warning or Normal result every time it is called. A
Function SHOULD return Warning or Normal results to indicate transitions - e.g.
when entering a warning or normal state.

## Configuration

A Function MUST support the following command-line flags:

* `--debug` (Default: `false`) - Enable debug logging.
* `--insecure` (Default: `false`) - Disable gRPC transport security.
* `--tls-certs-dir` - A directory containing mTLS server certs (tls.key and
  tls.crt), and a CA used to verify clients (ca.crt).

A Function MUST support reading the `--tls-certs-dir` flag from the
`TLS_SERVER_CERTS_DIR` environment variable. A Function MUST serve insecurely if
both `--tls-certs-dir` and `--insecure` are specified.

A Function MUST listen for gRPC requests on TCP port 9443, regardless of whether
they are using mTLS transport security or have transport security disabled.

A Function MUST enable gRPC transport security unless the `--insecure` flag
is explicitly specified.

## Packaging

A Function MUST be packaged according to the [xpkg Specification]. A Function
SHOULD make use of the `io.crossplane.xpkg: base` OCI annotation to use a single
OCI image to deliver both its package metadata (`package.yaml`) and Function
binary.

A Function MUST have a name beginning with `function-`. This name MUST appear in
its package metadata, and SHOULD be used elsewhere, for example its GitHub
repository.

## Runtime Environment

A Function MUST NOT assume it is deployed in any particular way, for example
that it is running as a Kubernetes Pod in the same cluster as Crossplane.

A Function MUST NOT assume it has network access. A Function SHOULD fail
gracefully if it needs but does not have network access, for example by
returning a Fatal result.

A Function SHOULD use the latest version of the SDK for its language, for
example https://github.com/crossplane/function-sdk-go.

[design document]: ../../design/design-doc-composition-functions.md
[RFC 2119]: https://www.ietf.org/rfc/rfc2119.txt
[xpkg Specification]: xpkg.md