# Composition Functions - Extra Resources

* Owner: Philippe Scorsolini (@phisco)
* Reviewers: Nic Cope (@negz)
* Status: Accepted

## Background

Composition Functions extend Crossplane to support a new way of configuring
how to reconcile a Composite Resource (XR). Each Function is a gRPC server.
Crossplane sends state to a Function via a `RunFunctionRequest` RPC. The
Function is intended to return the desired state via a `RunFunctionResponse`
RPC.

Currently, a Composition author who needs to access information from a resource
external to their Composition has the following options:
- [**BETA**] Create an `ObserveOnly` resource from their Composition and
  reading the required information from its `status`.
- [**ALPHA**] Create an `EnvironmentConfig`, either manually or from another
  Composition, with the required information and reference it from their
  Composition via `spec.environment.environmentConfigs`.
- Use MR-level referencing, if supported for the source and target of the
  information, being implemented by the provider.

Each of these strategies has its drawbacks:
- The creation of `ObserveOnly` resources could result in a significant number
  of resources being created.
- The use of `EnvironmentConfigs` requires the Composition author to create and
  maintain a separate resource, which can be cumbersome and error-prone.
  However, it has to be said that there is value in the clean separation of concerns
  between the `EnvironmentConfig`'s producer and consumer(s).
- Using MR-level referencing is not always possible, as it requires explicit
  provider support, and hardly ever crosses a single provider's (family) boundaries.

As per the Composition Functions' [specification]:
```text
A Function MUST NOT assume it is deployed in any particular way, for example
that it is running as a Kubernetes Pod in the same cluster as Crossplane.

A Function MUST NOT assume it has network access. A Function SHOULD fail
gracefully if it needs but does not have network access, for example by
returning a Fatal result.
```

A clear consequence of the above assumptions is that a Function should not
assume access to the Kubernetes API. Which means that letting Functions fetch
external resources on their own is not an option, so Crossplane needs to provide
them.

With this proposal we aim to provide Functions a way to request additional
existing Crossplane resources.

## Proposal

We propose to allow Functions to request extra resources through the
`RunFunctionResponse`.

The `RunFunctionResponse` and `RunFunctionRequest` protobuf messages have to be
updated as follows:
```protobuf
syntax = "proto3";

message RunFunctionResponse {
  // Existing fields omitted for brevity

  // Requirements that must be satisfied for this Function to run successfully.
  Requirements requirements = 5;
}

message Requirements {
  // Extra resources that this Function requires.
  // The map key uniquely identifies the group of resources.
  map<string, ResourceSelector> extra_resources = 1;
}

message ResourceRef {
  string api_version = 1;
  string kind = 2;
  string name = 3;
}

message ResourceSelector {
  string api_version = 1;
  string kind = 2;

  // here we would actually want to use a oneof, but due to the need of wrapping
  // messages, the syntax would become a bit cumbersome, so we'll see how to handle
  // that at implementation time.
  optional string match_name = 3;
  map<string, string> match_labels = 4;
}

message RunFunctionRequest {
  // Existing fields omitted for brevity

  // Note that extra resources is a map to Resources, plural.
  // The map key corresponds to the key in a RunFunctionResponse's
  // extra_resources field. If a Function requests extra resources that
  // don't exist Crossplane sets the map key to an empty Resources
  // message to indicate that it attempted to satisfy the request.
  map<string, Resources> extra_resources = 3;
}

// Resources just exists because you can't have map<string, repeated google.protobuf.Struct>.
message Resources {
  repeated google.protobuf.Struct items = 1;
}
```

And the logic according to which Crossplane will run the pipeline would be as
follows:
```golang
// The following code should not be considered as a final implementation, but
// rather as a pseudo-code to show the intended logic.

const (
  // maxIterations is the maximum number of times a Function should be called,
  // limiting the number of times it can request for extra resources, capped for
  // safety. We might allow Composition authors customize this value in the future.
    maxIterations = 3
)


// Compose resources using the Functions pipeline.
func (c *FunctionComposer) Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error) {
	// Existing code omitted for brevity...

	// Run any Composition Functions in the pipeline.
	for _, fn := range req.Revision.Spec.Pipeline {
		// used to store the resources fetched at the previous iteration 
		var extraResources map[string]v1beta1.Resources
		// used to store the requirements returned at the previous iteration 
		var requirements map[string]v1beta1.ResourceSelector
		
		for i := int64(0); i < maxIterations; i++ {
			req := &v1beta1.RunFunctionRequest{Observed: o, Desired: d, Context: fctx, extraResources: extraResources}

			// Run the Composition Function and get the response.
			rsp, err := c.pipeline.RunFunction(ctx, fn.FunctionRef.Name, req)
			if err != nil {
				return CompositionResult{}, errors.Wrapf(err, /* ... */)
			}

			// Pass the desired state and context returned by this Function to the next iteration.
			d = rsp.GetDesired()
			fctx = rsp.GetContext()
			
			rs := rsp.GetRequirements()
			if rs.Equal(requirements) {
				// the requirements stabilized, the function is done
				break
			}

			// if we reached the maximum number of iterations we need to return an error
			if i == maxIterations - 1 {
				return CompositionResult{}, errors.Wrapf(err, /* ... */)
			}
			
			// store the requirements for the next iteration 
			requirements = rs

			extraResourcesRequirements := rs.GetExtraResources()

			// Cleanup the extra resources from the previous iteration to store the new ones
			extraResources = make(map[string]v1beta1.Resources)
			
			// Fetch the requested resources and add them to the desired state.
			for name, selector := range extraResourcesRequirements {
				// Fetch the requested resources and add them to the desired state.
				resources, err := fetchResources(ctx, c.client, selector)
				if err != nil {
					return CompositionResult{}, errors.Wrapf(err, /* ... */)
				}
				
				// resources would be nil in case of not found resources
				extraResources[name] = resources
			}

		}
		
		// Existing code omitted for brevity...
	}
	
	// Existing code omitted for brevity...
}

// fetchResources fetches resources that match the given selector.
func fetchResources(ctx context.Context, client client.Reader, selector *v1beta1.ResourceSelector) ([]*unstructured.Unstructured, error) {
	// Fetch resources by name or label selector...
	// return nil on not found error
}

```

With the above proposal what currently can be implemented as follows:
```yaml
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: nop.sqlinstances.example.org
spec:
  compositeTypeRef:
    apiVersion: example.org/v1alpha1
    kind: XSQLInstance
  environment:
    environmentConfigs:
      - ref:
          name: example-environment-1
      - type: Selector
        selector:
          mode: Multiple
          sortByFieldPath: data.priority
          matchLabels:
            - type: FromCompositeFieldPath
              key: stage
              valueFromFieldPath: metadata.labels[stage]
  resources:
    - name: nop
      base:
        apiVersion: nop.crossplane.io/v1alpha1
        kind: NopResource
        spec:
          forProvider:
            conditionAfter:
              - conditionType: Ready
                conditionStatus: "False"
                time: 0s
              - conditionType: Ready
                conditionStatus: "True"
                time: 1s
      patches:
        - type: FromEnvironmentFieldPath
          fromFieldPath: complex.c.f
          toFieldPath: metadata.annotations[valueFromEnv]
```

Could be rewritten as follows, assuming we implemented a Function (e.g.
`function-environment-configs` or a more generic `function-extra-resources`)
that can request `EnvironmentConfigs` as extra resources, merge them according
to its logic and set the result as in the `Context`, at the key
`function-patch-and-transform` conventionally expects it,
`apiextensions.crossplane.io/environment`:
```yaml
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: nop.sqlinstances.example.org
spec:
  compositeTypeRef:
    apiVersion: example.org/v1alpha1
    kind: XSQLInstance
  mode: Pipeline
  pipeline:
  - name: envconfigs
    functionRef:
      name: function-environment-configs
    input:
      apiVersion: extra.fn.crossplane.io/v1beta1
      kind: ExtraResourceSelectors
      extraResourceSelectors:
        environmentConfigs:
        - ref:
            name: example-environment-1
        - type: Selector
          selector:
            mode: Multiple
            sortByFieldPath: data.priority
            matchLabels:
              - type: FromCompositeFieldPath
                key: stage
                valueFromFieldPath: metadata.labels[stage]
  - name: patch-and-transform
    functionRef:
      name: function-patch-and-transform
    input:
      apiVersion: pt.fn.crossplane.io/v1beta1
      kind: Resources
      resources:
        - name: nop
          base:
            apiVersion: nop.crossplane.io/v1alpha1
            kind: NopResource
            spec:
              forProvider:
                conditionAfter:
                  - conditionType: Ready
                    conditionStatus: "False"
                    time: 0s
                  - conditionType: Ready
                    conditionStatus: "True"
                    time: 1s
          patches:
            - type: FromEnvironmentFieldPath
              fromFieldPath: complex.c.f
              toFieldPath: metadata.annotations[valueFromEnv]
```

Or, we could think of adding such capabilities directly to
`function-patch-and-transform` directly, which however would make it a little
harder to debug given that the whole logic would be in a single Function:
```yaml
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: nop.sqlinstances.example.org
spec:
  compositeTypeRef:
    apiVersion: example.org/v1alpha1
    kind: XSQLInstance
  mode: Pipeline
  pipeline:
  - name: patch-and-transform
    functionRef:
      name: function-patch-and-transform
    input:
      apiVersion: pt.fn.crossplane.io/v1beta1
      kind: Resources
      environment:
        environmentConfigs:
        - ref:
            name: example-environment-1
        - type: Selector
          selector:
            mode: Multiple
            sortByFieldPath: data.priority
            matchLabels:
              - type: FromCompositeFieldPath
                key: stage
                valueFromFieldPath: metadata.labels[stage]
      resources:
        - name: nop
          base:
            apiVersion: nop.crossplane.io/v1alpha1
            kind: NopResource
            spec:
              forProvider:
                conditionAfter:
                  - conditionType: Ready
                    conditionStatus: "False"
                    time: 0s
                  - conditionType: Ready
                    conditionStatus: "True"
                    time: 1s
          patches:
            - type: FromEnvironmentFieldPath
              fromFieldPath: complex.c.f
              toFieldPath: metadata.annotations[valueFromEnv]
```

### Future Work

We left to decide and possibly develop in the future the following
functionalities:
- allowing Composition authors to customize the maximum number of iterations.
- allowing Composition authors to specify what resources Functions can request.
- allowing Functions to specify what resources should be watched.
- caching requirements between reconciliation loops.

See the dedicated section below for a more detailed discussion and the current
thinking around these topics.

## Options Considered

We established the feature design space by addressing these questions:

- **Q0**: Should Functions be able to request additional resources?
- **Q1**: How should Functions request additional resources? 
- **Q2**: Which steps in the pipeline should have access to the requested resources?
- **Q3**: Should Functions be able to request additional resources incrementally?
- **Q4**: How should Functions specify what resources they want?

### TL;DR: Decisions taken

Here is a summary of the choices made, read the following sections for the
rationale behind them:

- **Q0**: Should Functions be able to request additional resources?
  - **Option 1**: Yes, Composition authors should rely on Functions to request
	additional resources.
- **Q1**: How should Functions request additional resources?
  - **Option 1**: Functions request additional resources through the
	`RunFunctionResponse`.
- **Q2**: Which steps in the pipeline should have access to the requested resources?
  - **Option 1**: Only the Function that requested the resources should have
	access to them.
- **Q3**: Should Functions be able to request additional resources incrementally?
  - **Option 1**: Functions can request additional resources incrementally.
  - **Q3.1**: What should Crossplane send over at the `N`-th iteration?
	- **Option 2 (Q3.1)**: Only the resources requested at the previous iteration, `N-1`.
- **Q4**: How should Functions specify what resources they want?
  - **Option 3**: both by name and via label selector.
  - **Q4.1**: How should Functions signal they are done requesting resources?
    - **Option 2 (Q4.1)**: by returning the same list of extra resource requests as the
      previous iteration.
- **Q5**: What about Resolve and Resolution Policies?
  - Decided not to allow configuring it, defaulting to `resolve` policy `Always` and
    `resolution` policy left to Functions to implement.


### Q0: Should Functions be able to request additional resources?

Clearly this is a yes or no question, but we need to do a step back. What we
actually care about is to have a way to allow **Composition authors** to specify
additional resources that Functions should have access to. So, the options we
have are actually:
- **Option 1**: Yes, Composition authors should rely on Functions to request
  additional resources.
- **Option 2**: No, Composition authors should not need Functions at all to
  request additional resources, it should be part of the Composition API.

**Option 2** would feel more "native", everything would be handled by
Crossplane, and we could design an API according to our standards. Ignoring
native patch-and-transform Compositions for now, which would present their own
set of issues around handling multiple results, due to the limited
expressiveness of the downstream language (P&T). Even if we decided to make it
only available for Functions based compositions, the API would need to be pretty
expressive to allow Composition authors to specify what resources they want,
e.g. by name, by label selector, based on fields from the Composite Resource, or
even another Composed resource some day... The API would need to be at least as
expressive, and therefore complex, as what we have today for
`EnvironmentConfigs` at `spec.environment.environmentConfigs`, or even as that
of [Kyverno][kyverno-context].

**Option 1** is clearly more flexible, as it would allow Function authors to
expose whatever API to Composition authors through inputs, or just decide it
according to whatever logic they want. However, this will obviously require at
least one more round-trip to get the requested resources from Crossplane to the
Function.

So, it feels like a more sensible choice to only consider the more flexible
**Option 1**, allowing Function authors to expose whatever API they want to let
Composition authors specify what resources they want, without needing to commit
to any specific overly complex API. At least not immediately. We can always
revisit this decision at a later stage if a perfect or "good enough" API arose
from Functions and make it a native part of the Composition API.
 
### Q1: How should Functions request additional resources?

We considered the following options:
- **Option 1**: Functions request additional resources through the
  `RunFunctionResponse`.
- **Option 2**: Functions request additional by calling back Crossplane through
  a new RPC.

We discarded **Option 2** for the following reasons:
- it would require Crossplane to be a gRPC server too, which would be a
  significant change to the current architecture, requiring Crossplane to be
  reachable and addressable from Functions. This would, for example, make
  `crossplane beta render` much more complex.
- to be able to handle it securely we would need to implement some way for
  Crossplane to check the "origin" of the request to validate that the requests
  are authorized according to the policy defined in the Composition. To do so, we
  would need to implement some sort of complex short-lived token mechanism.

Therefore, **Option 1** is the only option we took into consideration.

### Q2: Which steps in the pipeline should have access to the requested resources?

We considered the following options:
- **Option 1**: Only the Function that requested the resources should have
  access to them.
- **Option 2**: Only the next step in the pipeline should have access to the
  requested resources.
- **Option 3**: All the steps in the pipeline should have access to the
  requested resources.
- **Option 4**: Both the Function that requested the resources and the next
  step in the pipeline should have access to the requested resources.

We discarded **Option 3** because of the following reasons:
- sending the requested resources to all the steps in the pipeline would
  significantly increase network usage.
- sending the requested resources to all the steps in the pipeline would
  significantly increase the security risk as potentially sensitive information
  could be leaked to steps that don't need it.
- how would we handle multiple steps in the pipeline requesting resources? Would
  Crossplane need to keep accumulating all the requested resources and continue
  sending them to all the steps in the pipeline? Would be complex to define the
  expected behavior.

**Option 1**, **Option 2** and **Option 4** are all valid, we'll evaluate them
in conjunction with the answer to **Q3**.

### Q3: Should Functions be able to request additional resources incrementally?

We considered the following options:
- **Option 1**: Functions can request additional resources incrementally.
- **Option 2**: Functions can request additional resources only once.

**Option 1** is more flexible, as it allows Functions to request additional
resources based on the results of previous requests. However, it also makes the
implementation more complex. We would want to limit the number of
iterations to a reasonable number by default and allow the user to override this
limit if needed. This would allow for example a Function to request a resource,
get the reference to another resource from it, and then request that resource
too, and so on. However, implementing it would require us to answer the
following questions:
- **Q3.1**: What should Crossplane send over at the `N`-th iteration?
   - **Option 1 (Q3.1)**: All the resources requested at all the previous
	 iterations, `[0..N-1]`.
   - **Option 2 (Q3.1)**: Only the resources requested at the previous iteration, `N-1`.

**Option 2 (Q3.1)** is more efficient with respect to both network and memory
usage. However, given that Functions should be considered stateless, it could
make the logic of the Functions much more complex if they needed to infer the
current state from the returned resources.

**Option 1 (Q3.1)** would allow Functions to be completely stateless and much simpler
in their logic, as they would always receive all the resources they requested at
all the previous iterations. However, it would require Crossplane to keep track
of all the resources requested at all the previous iterations, which would make
the implementation a much more complex and more expensive both in terms of
memory and network.

However, **Option 2 (Q3.1)** would still allow Functions to implement the same
behavior as **Option 1 (Q3.1)** by requesting all the resources requested at the
previous iteration, plus the new ones, at each iteration.

For this reason we decided to only consider **Option 2 (Q3.1)**.

**Option 2 (Q3)** is definitely simpler to implement but also less flexible, and
it would be equivalent to **Option 2 (Q3.1)** with a limit of `1` iteration,
therefore we decided to discard it, and only consider **Option 1 (Q3)**.

The cap on the number of iterations could be specified in the `PipelineStep`
struct as follows:
```golang
// A PipelineStep in a Composition Function pipeline.
type PipelineStep struct {
	// Other fields omitted for brevity ...

	// MaxIterations is the maximum number of iterations this step should
	// execute. Set it to -1 to execute it indefinitely.
	// +kubebuilder:default=1
	// +optional
	MaxIterations *int64 `json:"maxIterations,omitempty"`
}
```

> [!NOTE]
> ** WE DECIDED NOT TO EXPOSE THIS FIELD FOR NOW ** and only set a hardcoded
> maximum number of iterations just for safety, e.g. `10`. We can always revisit
> this decision at a later stage if needed.

We can now go back to **Q2** and evaluate the options we left open, in
conjunction with the decision we've now taken. 

**Option 2 (Q3.1)** imply that the Function itself already has access to the
requested resources, and therefore already implies at least **Option 1 (Q2)**.

**Option 2 (Q2)** doesn't apply anymore, as we already decided the Function
itself will have access to the resources it requested.

We decided to go with **Option 1 (Q2)** and discard **Option 4 (Q2)** for the
following reasons:
- Crossplane will decide that the Function has finished requesting extra
  resources at the `<N>`-th iteration once the Function returns an empty list of
  extra resource requests, so what Crossplane would need to send to the next step
  in the pipeline is not obvious. All the accumulated resources requested by the
  Function before? the resources requested at the `<N-1>`-th iteration?
- Sending the resources to the next step in the pipeline would increase the
  security risk without any obvious benefit.
- Functions can already pass additional information to the next step in the
  pipeline through the `Context` in the `RunFunctionResponse`, and it would be
  better not to introduce additional ways to do so, without a really good reason.

### Q4: How should Functions specify what resources they want?

This question is orthogonal to the previous ones, as the answer to it
would be the same regardless of the answers to the previous questions.

We considered the following options:
- **Option 1**: by name.
- **Option 2**: via label selector.
- **Option 3**: both by name and via label selector.

Given that one of the thought exercises we tried to do while designing this
feature, was to allow reimplementing the same functionality today at
`spec.environment.environmentConfigs` level, we decided to only consider
**Option 3**.

Taking into account this decision and the one for **Q3**, the
`RunFunctionResponse` and `RunFunctionRequest` `protobuf` messages could look as
follows:

```protobuf
syntax = "proto3";

message RunFunctionResponse {
  // Existing fields omitted for brevity

  // Requirements that must be satisfied for this Function to run successfully.
  Requirements requirements = 5;
}

message Requirements {
  // Extra resources that this Function requires.
  // The map key uniquely identifies the group of resources.
  map<string, ResourceSelector> extra_resources = 1;
}

message ResourceSelector {
  string api_version = 1;
  string kind = 2;

  // Here we would want to use a oneof, but due to the need of wrapping
  // messages, the syntax would become a bit cumbersome, so we'll see how to handle
  // that at implementation time.
  optional string match_name = 3;
  map<string, string> match_labels = 4;
}

message RunFunctionRequest {
  // Existing fields omitted for brevity

  // Note that extra resources is a map to Resources, plural.
  // The map key corresponds to the key in a RunFunctionResponse's
  // extra_resources field. If a Function requests extra resources that
  // don't exist Crossplane sets the map key to an empty Resources
  // message to indicate that it attempted to satisfy the request.
  map<string, Resources> extra_resources = 3;
}

// Resources just exists because you can't have map<string, repeated Resource>.
message Resources {
  repeated Resource resources = 1;
}
```

We considered moving the `extra_resources` field from the `RunFunctionResponse`
to the `State` so that it could be set in the `observed` `State` of the
`RunFunctionRequest`, however, we decided not to do so for the following
reasons:
- currently we don't have anything else we need to modify in the observed state
  between Functions, keeping it so would make the implementation simpler to
  reason about. 
- that would be awkward as we would need to ignore that field in the `desired`
  `State` for both `RunFunctionRequest` and `RunFunctionResponse`.
Another option would have been to split the observed and desired state into two
separate messages, however, this would be a breaking change for the existing
API, we could reconsider this choice for `v1beta2`, or `v1`.

Another related question we need to answer is **Q4.1**: How should Functions
signal they are done requesting resources?

We considered the following options:
- **Option 1**: by returning an empty list of extra resource requests.
- **Option 2**: by returning the same list of extra resource requests as the
  previous iteration.

We decided to go with **Option 2**, as it felt more symmetrical w.r.t. the way
Functions already request desired state and avoided the need for Functions to
implement additional logic to decide when to stop requesting resources. This
also allows to define "stability" for requirements which might be useful down
the line if we want to implement caching of requirements between reconciliation
loops.

### Q5: What about Resolve and Resolution Policies?

Currently, the following policies can be configured for both MR-level references
and `EnvironmentConfigs`:
```golang
// Policy represents the Resolve and Resolution policies of Reference instance.
type Policy struct {
  // Resolve specifies when this reference should be resolved. The default
  // is 'IfNotPresent', which will attempt to resolve the reference only when
  // the corresponding field is not present. Use 'Always' to resolve the
  // reference on every reconcile.
  // +optional
  // +kubebuilder:validation:Enum=Always;IfNotPresent
  Resolve *ResolvePolicy `json:"resolve,omitempty"`

  // Resolution specifies whether resolution of this reference is required.
  // The default is 'Required', which means the reconcile will fail if the
  // reference cannot be resolved. 'Optional' means this reference will be
  // a no-op if it cannot be resolved.
  // +optional
  // +kubebuilder:default=Required
  // +kubebuilder:validation:Enum=Required;Optional
  Resolution *ResolutionPolicy `json:"resolution,omitempty"`
}
```

Originally, references were only resolved once, which is equivalent to the
current default, `IfNotPresent`, and resolution was `Required` only. Other
options were added [later][resolve-resolution-policies] to allow more flexibility,
see [this discussion][resolve-resolution-policy-discussion] for more details.

Both for EnvironmentConfigs and MR-level references, the resolve policy
`IfNotPresent` is implemented by storing a reference, by name, in the `spec`,
of the XR and MR respectively. This way on the next reconcile loop, Crossplane
won't actually try to resolve the reference again, but rather only rely on the
stored references. This is trivial in case of references by name, the request is
directly added to the references. With label selectors though, this means that
the requested resources are not actually fetched according to the labels
specified at each reconciliation loop, instead, only the set of resources
found on the first reconciliation loop are going to be always used with this
policy.

We might consider applying these policies to extra resources requested by
Functions as well, but the precise meaning in this context is not easy to
define, especially for the `resolve` policy.

The naive implementation, blindly getting all the resources requested by
Functions every time, would implement the behavior expected for `resolve: Always`.

Defining the `resolve` policy `IfNotPresent` is easy for statically defined
references, which is the case for `EnvironmentConfigs` and MR-level references,
still the actual behavior results unexpected at
[times][resolve-resolution-policy-discussion], while for extra resources
requested by Functions, it would be even more surprising.

For example, let's assume a Composition with a single step defined, on the first
reconciliation loop: Crossplane calls the Function with no extra resources,
get a response back, requesting an `EnvironmentConfig` named `foo` and all
`EnvironmentConfigs` with label `env` set to `bar`. Crossplane then fetches
the requested resources and send them back to the Function, which returns a
response with no additional resources requested signaling that it has finished
requesting extra resources.

Crossplane would need to "save" somewhere references to the resources requested
during the first reconciliation loop, and only fetch and return those on
subsequent ones, but this would be surprising for users expecting "fresh" results
every time, and it would be even more surprising if the Function requested
additional resources on subsequent iterations, as we would have to decide whether
to return the resources requested at the first iteration only, or add the new
requested resources to the list of "frozen" choices.

Due to the complexity of such a mechanism, we decided not to implement it for
now and to only consider the `resolve` policy `Always` for extra resources
requested by Functions. Function authors could still implement `IfNotPresent` themselves
by writing to the Composite Resource `status` if really needed.

The `resolution` policy instead can be easily demanded to Functions, as we said above,
in case of an extra resource not found, Crossplane would just add an empty
`Resources` message to the `extra_resources` map, signaling that it attempted to
satisfy the constraint, then, a Function could easily implement the `resolution`
policy `Required` or `Optional` by respectively returning a fatal error or just
ignoring the missing resource.

##  Potential Future Improvements
We decided to leave the following questions open for future work:
- **Q6**: How can Composition authors specify what resources a Function can
  request?
- **Q7**: How can we support "Realtime Compositions"?

### TL;DR: Possible decisions
- **Q6**: How can Composition authors specify what resources Functions can
  request? ** WE DECIDED NOT TO IMPLEMENT THIS **, see
  [comment][decision-no-policy].
  - **Q6.1**: When should the policy be evaluated?
    - **Option 1 (Q6.1)**: Before gathering the requested extra resources.
  - **Q6.2**: What should be the policy's input?
    - **Option 1 (Q6.2)**: The request only.
- **Q7**: How can we support "Realtime Compositions"? ** WE DECIDED NOT TO
  IMPLEMENT THIS **, see [comment][decision-no-realtime].
  - **Q7.1**: How should Functions specify what resources should be watched?
    - **Option 2 (Q7.1)**: Let Functions specify which resources should be watched.
  - **Q7.2**:  How should Functions specify what resources should be watched?
    - **Option 2 (Q7.2)**: Functions request to watch specific resources separately
      and only by name.
  - **Q7.3**: Where should Crossplane store the references to the extra resources
    requested by Functions?
    - **Option 3 (Q7.3)**: In the XR's `status`, in a new dedicated field.
  - **Q7.4**: What resources should Functions be able to watch?
    - **Option 2 (Q7.4)**: Only resources requested in previous iterations.

### Q6: How can Composition authors specify what resources Functions can request?

> [!NOTE]
> ** WE DECIDED NOT TO IMPLEMENT THIS **: we'll allow all resources by default
> and we'll revisit this decision at a later stage if needed, see
> [here][decision-no-policy] for more details. We'll add a flag to allow
> disabling this functionality > altogether if really needed by Cluster
> operators.

To answer this question, we need to answer the following questions first,
**Q6.1**: **WHEN** should the policy be evaluated?
- **Option 1 (Q6.1)**: Before gathering the requested extra resources.
- **Option 2 (Q6.1)**: After having gathered the requested extra resources.

**Option 1 (Q6.1)** would be more efficient, as we would not request any
resource in case of an invalid request. However, it would be less flexible and
more coarse-grained, as the policy would only have to take into account the
information provided in the request, so as per the decision taken above for
**Q4**, only names and labels. While with **Option 2 (Q6.1)** the policy would
have the whole resources available and could therefore be much more expressive.

We decided to only consider **Option 1 (Q6.1)**, as we deemed unnecessary to
have the extra flexibility of **Option 2 (Q6.1)**, and we could always add that
later, if needed, but there would still be value in having a simpler API to
cover the majority of the use cases.

So, decided that the policy should be evaluated before actually gathering the
requested extra we can answer **Q6.2**: What should be the policy's input?
- **Option 1 (Q6.2)**: The request only.
- **Option 2 (Q6.2)**: Also the observed state.

**Option 1 (Q6.2)** would require a less expressive policy language that could
exactly mirror the extra resources requests language, e.g. only names and
labels, and therefore would be extremely simple to validate against the request.

Note that while Functions could still implement label selectors based on the
current observed state, e.g. a label having a key or value based on a field in
the Composite Resource, a policy language having as input only the request
without any other input would have to be coarser-grained.

**Option 2 (Q6.2)** would allow for a much more expressive policy language, as
it would have access to the current observed state, and as long as it "rendered"
to something easily comparable to the requests language, it would be easy to
implement too, e.g. JSONPath expressions, Crossplane-flavored fieldpaths or
regexes.

Kubernetes users are already used to RBAC policies being evaluated only against
the request. So, we deemed sufficient, at least as a first implementation, to
choose **Option 1 (Q6.2)**. We can always revisit this decision at a later stage
if needed, but a coarser-grained and simpler policy language would still be
useful.

Note: we could reconsider **Option 2 (Q6.1)** at a later stage and add a more
expressive policy language to be evaluated after having gathered the requested
extra resources, for example using a full-fledged policy language such as
[rego](https://www.openpolicyagent.org/docs/latest/policy-language/), or a
similar syntax as the one used by [Kyverno](https://kyverno.io/), or a list of
[Common Expression Language](https://github.com/google/cel-go) (CEL)
expressions, which would allow the Composition author to specify much more
complex policies.

Given the above choices, the answer to **Q6** is already decided. The policy
language will have to mirror the request language. To make this a bit more
flexible, we could allow specifying regexes for the names and labels.

An example Composition could look like the following:
```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  mode: Pipeline
  pipeline:
    - step: get-extra-resources
      functionRef:
        name: function-extra-resources
      allowedExtraResources:
          # all Something.example.test.com/v1 resources
        - apiVersion: example.test.com/v1
          kind: Something
          # both name and matchLabels to nil

          # all SomethingElse.example.test.com/v1 resources named according to the provided regex
        - apiVersion: example.test.com/v1
          kind: SomethingElse
          name: "something-else-.*"

          # all EnvironmentConfigs with label environment=dev, prod or qa
        - apiVersion: apiextensions.crossplane.io/v1alpha1
          kind: EnvironmentConfig
          matchLabels:
            environment: "(dev|prod|qa)" # "*" would match any label value

      input:
        apiVersion: extra.fn.crossplane.io/v1beta1
        kind: ExtraResourceSelectors
        # a list of resources by name or label selector
```

By default, the policy should be to deny all requests, so the Composition author
would have to explicitly allow the requests they want to allow.

The PipelineStep struct would look like the following:
```golang
// A PipelineStep in a Composition Function pipeline.
type PipelineStep struct {
	// Step name. Must be unique within its Pipeline.
	Step string `json:"step"`

	// FunctionRef is a reference to the Composition Function this step should
	// execute.
	FunctionRef FunctionReference `json:"functionRef"`

	// Input is an optional, arbitrary Kubernetes resource (i.e. a resource
	// with an apiVersion and kind) that will be passed to the Composition
	// Function as the 'input' of its RunFunctionRequest.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	Input *runtime.RawExtension `json:"input,omitempty"`

	// AllowedExtraResources is a list of resources by name or label selector
	// that this pipeline step is allowed to request.
	// +optional
	AllowedExtraResources []ResourceSelector `json:"allowedExtraResources,omitempty"`
}

// ResourceSelector is a selector for resources.
// Both name and matchLabels are optional, if both are nil, all resources of the
// specified kind and apiVersion can be requested.
type ResourceSelector struct {
	APIVersion string `json:"apiVersion"`
	Kind string `json:"kind"`
	
	// MatchName is a regex that the name of the resource must match.
	// Only one of MatchName or MatchLabels can be specified.
	// +optional
	MatchName *string `json:"matchName,omitempty"`
	
	// MatchLabels is a map of labels that the resource must match.
	// Values are regular expressions.
	// Only one of MatchName or MatchLabels can be specified.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}
```

The above policy could be easily validated against the request by Crossplane
by going through in `O(n^2)` with a naive implementation.

### Q7: How can we support "Realtime Compositions"?

> [!NOTE]
> ** WE DECIDED NOT TO IMPLEMENT THIS **: "Realtime Compositions" are an Alpha
> feature and we'll revisit this decision at a later stage. See
> [here][decision-no-realtime] for more details.

With [Realtime Compositions][pr-realtime-compositions] enabled, a reconciliation
loop for the right XR is triggered for any change to any of the Composed
Resources.

It would make sense to be able to do the same for extra resources requested by
Functions too.

This might sound like this would require a similar solution to the question
above of `resolve` and `resolution` policies, as it will require storing
resolved references somewhere as well, however, the difference is that in this
case, we would need that information to only be able to route the events to the
right XR, therefore a flat list of references would be enough, as we would not
need to keep track of the Functions that requested them and the exact request
that brought to it being fetched.

We have considered the following options for how Crossplane should which
resources should be watched (**Q7.1**):
- **Option 1 (Q7.1)**: Automatically watch all the requested resources.
- **Option 2 (Q7.1)**: Let Functions specify which resources should be watched.

**Option 1 (Q7.1)** would be the simplest to implement, but it would also be the
most inefficient, as it would require Crossplane to potentially track a large
number of resources that are not actually needed by the Composition, resulting
in a large number of reconciliation loops being triggered for no reason.

**Option 2 (Q7.1)** would be more complex to implement, as we would need to
extend the RPC API to allow Functions to specify which resources should be
watched, however it would still allow to implement the same behavior as **Option
1**, if needed.

So, we decided to only consider **Option 2 (Q7.1)**,

But first, we'll need to answer another question, **Q7.2**: How should
Functions specify what resources should be watched?

We considered the following options for **Q7.2**:
- **Option 1 (Q7.2)**: Functions request to watch resources when requesting
  them, only query results are watched.
- **Option 2 (Q7.2)**: Functions request to watch specific resources separately
  and only by name.
- **Option 3 (Q7.2)**: same as **Option 1 (Q7.2)**, but also routing future
  matches if applicable, not existing at the time of the request.

**Option 3 (Q7.2)** would be the most powerful, however, it would also be really
complex to implement. The current implementation of `Realtime Compositions`
relies on two indexes mapping Composed Resources to the XR, they are referenced
by. Implementing this would require something much more complex, assuming it's
even possible, hence we discarded this option.

**Option 2 (Q7.2)** would be the simplest to implement, as it would only require
to extend the `RunFunctionResponse` with a list of references of resources to
watch. Crossplane could then keep track of all the requests, until the Function
doesn't return any more extra resources requests, collect the set of resources
to watch throughout the whole pipeline, and only at last, properly set them in
the XR.

**Option 1 (Q7.2)** would be more or less equivalent to **Option 2 (Q7.2)** as
both would require the same number of round-trips between Crossplane and the
Function, the former with a slightly smaller payload though, as Functions
wouldn't need to send back anything more than just a field specifying whether
the results should be watched or not. However, in the long run, **Option 1
(Q7.2)** being less fine-grained, could lead to more reconciliation loops and
therefore more wasted resources

So, we decided to only consider **Option 2 (Q7.2)**.

As we said currently, Composed Resources are tracked in the XR at
`spec.resourceRefs`, and similarly, `EnvironmentConfigs` are tracked at
`spec.environmentConfigRefs`. And the existing indexes for Realtime Composition
only handle Composed Resources and do so by leveraging the list of refs at
`spec.resourceRefs`.

It's debatable whether these references should be in the XRs' `spec` or
`status`, as according to the [Kubernetes API Conventions], as it's not so
common for a controller to modify the `spec` of its own resource to store state
information, but rather the `status`. However, we don't want here to discuss
whether the existing fields should be moved to the `status`, but we need to take
this into consideration to decide where to store references to the extra
resources requested by Functions.

So, **Q7.3**: Where should Crossplane store the references to the extra resources
requested by Functions?
- **Option 1 (Q7.3)**: In the XR's `spec`, but in the existing `resourceRefs`
  field.
- **Option 2 (Q7.3)**: In the XR's `spec`, in a new dedicated field.
- **Option 3 (Q7.3)**: In the XR's `status`, in a new dedicated field.

**Option 1 (Q7.3)** would allow to reuse the existing indexes, however the whole
codebase relies on the fact that the `resourceRefs` field only contains
references to Composed Resources, therefore we would need to allow
differentiating the two types of refs, and it would be really hard to be sure
that we are not breaking any existing functionality. So, we discarded **Option 1
(Q7.3)**.

The implementation for **Option 2 (Q7.3)** and **Option 3 (Q7.3)** would be
almost identical, the only difference would be more a matter of taste. In **Q6**
we decided that, due to the lack of clarity around their semantics, we
won't implement `resolve` policies for extra resources requested by Functions,
so, it would make little sense to have these too in the `spec`, so we decided to
only consider **Option 3 (Q7.3)**, which would also be the option more in line
with the [Kubernetes API Conventions].

We could store the references in a new dedicated field in the `status`, e.g.
`watchedResourceRefs`:
```golang
// CompositeResourceStatusProps is a partial OpenAPIV3Schema for the status
// fields that Crossplane expects to be present for all defined or published
// infrastructure resources.
func CompositeResourceStatusProps() map[string]extv1.JSONSchemaProps {
	return map[string]extv1.JSONSchemaProps{
		// Other fields omitted for brevity ...
		
		"watchedResourceRefs": {
			Type:     "object",
			Required: []string{"apiVersion", "kind", "name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"apiVersion": {Type: "string"},
				"kind":       {Type: "string"},
				"name":       {Type: "string"},
			},
		},
	}
}
```

We would need to update the indexer functions to also check the newly added
field refs, which should be relatively trivial, see the
[PR][pr-realtime-compositions] that introduced them in the first place for a
hint.

The `RunFunctionResponse` would need to be updated as follows:
```protobuf
syntax = "proto3";

message RunFunctionResponse {
  // Existing fields omitted for brevity

  // Requirements that must be satisfied for this Function to run successfully.
  Requirements requirements = 5;
}

message Requirements {
  // Other fields omitted for brevity
  
  // Resources that should be watched by Crossplane.
  repeated ResourceRef watch_resources = 2;
}

message ResourceRef {
  string api_version = 1;
  string kind = 2;
  string name = 3;
}
```

For security reasons, we definitely want Composition authors to be able to
constraint the resources a Function should be able to watch, and it would be
cumbersome to have to specify a separate list of policies with respect to the
one we already have defined in **Q6**. So we'll have to answer the following
question too, **Q7.4**: What resources should Functions be able to watch?

The options we considered are the following:
- **Option 1 (Q7.4)**: Any resource they want.
- **Option 2 (Q7.4)**: Only resources requested in previous iterations.
- **Option 3 (Q7.4)**: Only the resources requested in the current iteration.

Let's assume the following scenario to evaluate these options: a Function needs
to access a value in the status from a resource `bar`, referenced by a resource
`foo`. So, at the `N`-th iteration, the Function requests `foo`, gets back a
response with it, based on some field in `foo` it requests `bar`, Crossplane
fetches it and sends it back at the `N+1`-th iteration, and then the function
can finally its job. The Function wants Crossplane to watch both `foo` and
`bar`.

**Option 1 (Q7.4)** would be the simplest to implement, and in our scenario, it
would obviously allow the function to specify that it wants to watch both `foo`
and `bar`. However, given that the policy language we defined in **Q6** is
also able to specify resources by labels, Crossplane would need to actually
fetch the resources to be able to evaluate the policy, as, although we know we
already have fetched them previously, there would be no guarantee that these are
actually in the set of previously requested extra resources.

In our scenario, the difference between **Option 2 (Q7.4)** and **Option 3
(Q7.4)** would be that in the first case, the Function would be able to ask to
watch both `foo` and `bar` at the `N+1`-th iteration or `foo` at the `N`-th
iteration and `bar` at the `N+1`-th iteration, while in the second case only
the latter would be possible. Given that **Option 2 (Q7.4)** would be more
flexible, without any major complexity increase, we decided to only consider
**Option 2 (Q7.4)**.

This way, by simply keeping track of the references to the resources requested
through all iterations Crossplane will be able to validate that all resources
the Function wants to watch are actually valid according to the policy defined
in **Q6**, given that these would always be a subset of the already validated
resources requested by the Function.

[specification]: ../contributing/specifications/functions.md
[design-doc]: ./design-doc-composition-functions.md
[function-patch-and-transform]: https://github.com/crossplane-contrib/function-patch-and-transform
[function-auto-ready]: https://github.com/crossplane-contrib/function-auto-ready
[function-template-go]: https://github.com/crossplane/function-template-go
[realtime-composition]: https://github.com/crossplane/crossplane/issues/4828
[resolve-resolution-policies]: https://github.com/crossplane/crossplane-runtime/pull/328
[resolve-resolution-policy-discussion]: https://github.com/crossplane/crossplane-runtime/issues/250
[kyverno-context]: https://kyverno.io/docs/writing-policies/external-data-sources/
[Kubernetes API Conventions]: https://github.com/zecke/Kubernetes/blob/master/docs/devel/api-conventions.md#spec-and-status
[pr-realtime-compositions]: https://github.com/crossplane/crossplane/pull/4637/files
[decision-no-policy]: https://github.com/crossplane/crossplane/pull/5099#discussion_r1439894828
[decision-no-realtime]: https://github.com/crossplane/crossplane/pull/5099#issuecomment-1889866515