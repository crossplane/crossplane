# Declaring Dependencies From an SDK

Findings from adding ordering support to `function-sdk-typescript`, and from
testing it against a real typed configuration
(`upbound/configuration-aws-network-ts`).

Work in progress; nothing here is settled.

## The goal

Today a composed resource references another through provider machinery:

```yaml
vpcIdSelector:
  matchControllerRef: true
```

The provider resolves that at reconcile time. It works, but the core engine
never sees the relationship, so it can't order anything, and a provider that
can't resolve a reference yet retries against the cloud API until it can.

Writing the reference as a field value instead makes the relationship visible.
The function says where the value comes from, the SDK records the dependency,
and Crossplane waits rather than letting the provider discover the problem.

## Where things stand

Three repositories, none of it merged anywhere.

* **`crossplane`**, branch `composed-resource-ordering` - the core
  implementation, behind `--enable-composed-resource-ordering`. Pushed to a
  fork.
* **`stevendborrelli/function-ordering`** - a test function that declares
  ordering constraints, published as a package.
* **`function-sdk-typescript`**, branch `composed-resource-ordering` - the work
  described here. Local only.

## The SDK API as it stands

Three modules under `src/dependency/`, in increasing order of how much they
know about your resources.

`to(req)` leaves `dependencies` unset, which Crossplane reads as "no opinion"
and carries forward, so a function that never thinks about ordering needs no
changes at all.

### Declaring an edge outright

Works with anything. No inference, no assumptions.

```typescript
import { dependsOn, dependsOnRequired } from '@crossplane-org/function-sdk-typescript';

// subnet waits for vpc to be ready; vpc waits for subnet to be gone.
dependsOn(rsp, 'subnet', 'vpc');

// A replacement that must exist before its predecessor is torn down.
dependsOn(rsp, 'new-db', 'old-db', { createBeforeDestroy: true });

// Wait on something the function requires but does not compose. Crossplane
// never deletes what it did not compose, so this orders creation only.
dependsOnRequired(rsp, 'instance', 'shared-vpc');
```

`to(req)` carries inherited edges forward, the same way it already carries
desired state and context, so declaring an edge adds to what earlier functions
declared rather than replacing it. Discarding them has to be deliberate:

```typescript
setDependencies(rsp, []);   // no ordering constraints at all
```

An earlier draft required a `passThroughDependencies(req, rsp)` call to keep
inherited edges. That was a footgun - forget it and you silently erase another
function's ordering - and it existed only because `dependencies` was the one
field `to()` didn't carry forward. Making it consistent removed the need for
the call, and it was dropped.

### Referencing a typed model field

For functions built on generated models. A reference is encoded as a sentinel
string, because that is the only thing a model's own `validate()` will accept
in a string field.

```typescript
import {
  externalName, fromModel, named, ref, resolveRefs, setDesiredComposedResources,
} from '@crossplane-org/function-sdk-typescript';
import { Subnet, VPC } from 'crossplane-models/ec2.aws.m.upbound.io/v1beta1';

const vpc = named<VPC>('vpc');

const subnet = new Subnet({
  metadata: { name: 'sn', namespace: 'default' },
  spec: {
    forProvider: {
      region: 'us-east-1',
      cidrBlock: '10.0.1.0/24',
      vpcId: externalName(vpc),       // instead of vpcIdSelector
    },
  },
});

subnet.validate();
desired['subnet'] = fromModel(subnet);

// Records subnet -> vpc, and fills in the VPC's external name.
rsp = setDesiredComposedResources(rsp, resolveRefs(req, rsp, desired));
```

`externalName(vpc)` is what a provider's `vpcIdRef` would have resolved to, so
it is the direct swap for a selector. For any other field, read it and wrap it:

```typescript
arn: ref(vpc.status.atProvider.arn),
```

`vpc` reads like the resource itself - editors complete its fields and the
compiler checks them - and `ref` marks where the value comes from. An earlier
draft took an accessor, `vpc.ref((v) => v.status?.atProvider?.arn)`, which
asked more of the reader than the audience for these functions should have to
give.

### Wiring arbitrary fields

Nothing here replaces a provider mechanism, because none exists. A ConfigMap
has no `addressRef`, and no selector can populate arbitrary data.

```typescript
const db = named<DBInstance>('db');

desired['app-config'] = {
  resource: {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'app-config', namespace: 'default' },
    data: {
      DB_HOST: ref(db.status.atProvider.address),
    },
  },
  ready: Ready.READY_UNSPECIFIED,
};
```

The ordering edge matters more here than for a VPC. Without it the ConfigMap is
created with the field missing, the app starts, reads no host, and connects to
nothing - a failure that looks like an application bug.

A database outside this composite is a required resource, not a composed one:

```typescript
import { namedRequired, ref } from '@crossplane-org/function-sdk-typescript';

const external = namedRequired<DBInstance>('external-db');

data: { DB_HOST: ref(external.status.atProvider.address) }
```

That emits a required-resource edge rather than a composed one, which matters:
Crossplane must not be asked to order the deletion of something it did not
compose. When a requirement matched several objects, name one -
`namedRequired('databases', 'primary')`. If you do not, and there is more than
one, the reference declines to resolve rather than guessing, and ordering holds
the resource back.

### Inferring from reads, for untyped functions

`infer.ts` wraps observed state in a Proxy and derives edges from whatever the
function reads. It suits dynamic functions and is a poor fit for typed ones -
see the findings below.

```typescript
const observed = trackObservedComposedResources(req);

dcds['subnet'] = {
  resource: { spec: { forProvider: {
    vpcId: observed['vpc'].resource.status.atProvider.id,
  } } },
  ready: Ready.READY_UNSPECIFIED,
};

rsp = setDesiredComposedResources(rsp, resolveDependencies(rsp, dcds));
```

## What testing against a real configuration showed

### Typed configurations reference by selector, not by value

In `configuration-aws-network-ts`: **12 selector references, 1 observed value
read.** The dependency that matters - subnet on VPC - is expressed as
`vpcIdSelector: { matchControllerRef: true }` and never flows through the
function. There is no dataflow to observe, so read-tracking finds nothing.

### The Proxy approach changes behavior in typed code

Where the configuration does read an observed value:

```typescript
const subnetId = observedComposed?.[subnetKey]?.resource?.status?.atProvider?.id;
if (subnetId) {
  subnetIds.push(subnetId);
}
```

A tracked read is always a truthy object, so the guard always passes and `?.`
never short-circuits. Placeholders for subnets that don't exist get pushed into
`subnetIds`, which then flows onward. That is silently wrong output, not a
missing edge.

"Placeholders are always truthy" was documented as a caveat. Against real code
it is the dominant idiom, because authors must handle resources that don't
exist yet. The caveat is the common case.

### Typed models reject object placeholders outright

Probed against the real AWS `Subnet` model:

```text
string sentinel    -> validate() PASSED,  survives toJSON() intact
object placeholder -> validate() FAILED:  vpcId must be string
```

`validate()` runs before anything the SDK can hook, and `fromModel` calls
`toJSON()`, which deep-serializes. So a placeholder must be a string, and that
is what `typed.ts` does:

```typescript
vpcId: vpc.externalName            // ${xp-ref:vpc:@externalName}
arn:   ref(vpc.status.atProvider.arn)
```

`externalName` is a plain property and is what a provider's `vpcIdRef` would
have resolved to, so it is the direct swap for a selector. `ref()` takes an
accessor and derives the path by running it against a recording proxy, so the
path can't drift from the field.

At emit time `resolveRefs` walks desired state, records an edge per source, and
substitutes values from observed state. A reference to a resource that doesn't
exist yet resolves to nothing and the field is dropped - correct, because
ordering means the resource isn't applied anyway.

## Forking the model generator

The generated models strip descriptions, but the source CRDs encode reference
targets in an upjet-generated form that parses:

```text
vpcIdRef      -> "Reference to a VPC in ec2 to populate vpcId."
vpcIdSelector -> "Selector for a VPC in ec2 to populate vpcId."
```

Measured across the AWS schemas in that configuration:

| | |
| --- | --- |
| `*Ref` fields described as references | 734 |
| Matching `Reference to a <Kind> in <group> to populate <field>.` | 718 (97%) |
| Distinct target kinds | 43 |

The 16 that don't match use a variant omitting the group
(`"Reference to a InternetGateway to populate gatewayId."`), which a second
pattern covers.

So a forked generator knows, per field, the populated field, the target kind
and the group. That lets it emit something the compiler can check on both ends:

```typescript
new Subnet({ spec: { forProvider: { region, cidrBlock, vpcIdFrom: vpc } } })
```

where `vpcIdFrom` accepts only a `Named<VPC>`. Wiring a subnet to a route table
becomes a type error, which neither selectors nor sentinel strings can catch.

This is strictly better than the SDK-only approach for reference fields, and it
is sugar over the same mechanism - the generated API still emits a sentinel and
relies on `resolveRefs`.

Two caveats. Parsing English descriptions couples the generator to upjet's
phrasing; a structured annotation on the CRD would be the durable version, with
description parsing as a bridge. And the generator only helps where a provider
already declares a reference - arbitrary field-to-field wiring still needs
`ref()`.

## How ref() resolves, and where it breaks

Probed against the implementation rather than reasoned about. Results are from
a run, not from reading the code.

### Paths are relative to the resource, not to observed state

Observed composed resources are wrapped - `{ resource, connectionDetails }` -
but a reference is written against the resource's own type:

```typescript
ref(vpc.status.atProvider.id)   // not vpc.resource.status...
```

The resolver unwraps the wrapper before walking the path, so `.resource` never
appears in a reference. Required resources are wrapped the same way, inside
`items`, and are unwrapped identically. Writing `.resource` explicitly would
look right and resolve to nothing.

### It works for

* Nested field reads, resolving to the value from observed state.
* Whole arrays. `ref(vpc.status.atProvider.ports)` resolved to `[80, 443]`.
* Values that don't exist yet, which resolve to nothing and drop the field
  while still recording the edge.

### It silently fails for

Both of these produce wrong output with no error, which is worse than not
working at all.

**Keys containing a dot.** The path is encoded dot-separated, so a key with a
dot in it cannot survive the round trip:

```typescript
ref(vpc.metadata.annotations['crossplane.io/external-name'])
// sentinel: ${xp-ref:vpc:metadata.annotations.crossplane.io/external-name}
// resolves to: nothing. The field is dropped.
```

That covers every annotation and most labels, since Kubernetes convention puts
a domain in the key. `externalName()` exists partly to route around this for
the one case that comes up constantly, but the general problem stands. The fix
is to encode the path as a JSON array rather than a dotted string.

**String interpolation.** A tracked read is a Proxy, and a template literal
resolves it to the string `"undefined"`:

```typescript
`${vpc.status.atProvider.id}`   // "undefined"
```

Not an error, and not the value - a literal `"undefined"` written into the
resource. The proxy should throw on `toString` and `Symbol.toPrimitive` with a
message pointing at `ref()`, so this fails loudly.

### It is limited by

* **String fields only, for typed models.** The sentinel is a string, so a
  reference in a `number` or `boolean` field fails the model's own validation.
* **No coercion on the way out.** A reference to a numeric field resolves to a
  number, which a ConfigMap will reject even though the sentinel passed the
  model's validation. The failure lands at the API server, not at build time.
* **The type is a polite fiction.** `ref()` returns the field's type but
  produces a string. Anything done with the value other than assigning it -
  comparing, measuring, slicing - operates on the sentinel.
* **No handle for the composite.** References address composed and required
  resources. Reading a field of the XR itself needs
  `getObservedCompositeResource` as usual.

The two silent failures should be fixed before this is used in anger; both are
small. The limitations are worth documenting rather than fixing, except
possibly the coercion one.

## Open questions

* **Non-string fields.** A sentinel is a string, so a reference in a `number`
  or `boolean` field fails the model's validation. Loud rather than silent, but
  a real ceiling. Fixing it means teaching the model layer about references
  rather than the SDK.
* **Degrading without the feature.** A reference that can't resolve leaves the
  field out. With ordering enabled that's fine, because the resource isn't
  applied. Without it, the resource *is* applied, incomplete - so this is worse
  than a selector against an older Crossplane. It should probably refuse to
  resolve unless `CAPABILITY_DEPENDENCIES` is advertised.
* **Whether `infer.ts` should exist.** On the evidence it is the wrong tool for
  typed code and hazardous in the presence of truthiness guards. `typed.ts`
  covers the case that matters.
