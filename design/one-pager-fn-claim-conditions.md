# Communication Between Composition Functions and the Claim

* Owner: Dalton Hill (@dalton-hill-0)
* Reviewers: Nic Cope (@negz)
* Status: Draft

## Background

### Desired Behavior
Composition Function authors should be able to communicate and translate
the underlying status with users.

#### Managed Resource Status
We think authors often won't want to surface the status as it appear on an MR,
but will probably want to derive more user-friendly messages from it. Messages 
that are more meaningful to folks reading claims.

Some examples include:
- The external system for an MR is unreachable.
- The MR is incorrectly configured.
- The MR is being created, updated, etc.

#### Internal Errors
We think authors may want to have a catch-all Internal Error
message. Authors should be able to display the real error on the XR and provide
a basic "Internal Error" message on the Claim.

Currently internal errors often leave the Claim in a "Waiting" state. It would
be nice to notify the user that an internal error was encountered, and that the
team has been notified by an alert.

### Existing Behavior

#### Function Results
Currently functions can return Results. Depending on the type of results seen,
you can observe the following behavior on the Composite Resource.

Fatal Result:
- Synced status condition is set to False, contains result's message.
- Warning Event generated (reason: ReconcileError), containing result's message.

Warning Result:
- Warning Event (reason: ComposeResources) generated, containing result's 
  message.

Normal Result:
- Normal Event (reason: ComposeResources) generated, containing result's 
  message.


#### Setting the Claim's Status
Currently the only path to communicate a custom message with the user is by 
defining your own field in the Claim's status.
For example, we can define an XRD with:
```yaml
status:
  someCommunicationField:
    - msg: "Something went wrong."
```

There are a couple issues with this solution.
- If we need to halt resource reconciliation due to a fatal error, we can do so
  with the [SDK](https://github.com/crossplane/function-sdk-go)'s
  `response.Fatal`, however, this does not also allow us to update the XR and
  Claim for communication with the user.
- There is an existing field that would be more intuitive to use as it is
  already performing this same task for Crossplane itself (`status.conditions`).

#### Setting the Composite's Status Conditions
Currently you can update the Composite's status conditions by setting them with
SetDesiredCompositeResource.
There are a couple of limitations to this:
- it only shows up on the XR
- it only shows up if there are no fatal results

Example of setting the Composite's status conditions.
```go
// includes:
//    corev1 "k8s.io/api/core/v1"
//    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//    xpv1   "github.com/crossplane/crossplane-runtime/apis/common/v1"
// 
//    "github.com/crossplane/function-sdk-go/response"
desiredXR, err := request.GetDesiredCompositeResource(req)
c := xpv1.Condition{
    Type:               xpv1.ConditionType("ImageReady"),
    Status:             corev1.ConditionFalse,
    LastTransitionTime: metav1.Now(),
    Reason:             "NotFound",
    Message:            "The image provided does not exist or you are not "+
                        "authorized to use it.",
}
desiredXR.Resource.SetConditions(c)
response.SetDesiredCompositeResource(rsp, desiredXR)
```

## Proposal
We would like to allow the Composition Function author to:
- Choose where results go (Claim or XR)
- Allow results to update the Status Conditions of the XR and Claim

The following sections get into the details of each of the above items.

### Choose Where Results Go
Currently each result returned by a function will create a corresponding
event on the XR (if no previous fatal result exists).

We can expand this functionality by allowing the Result to have targets. In
order to accomplish this, we will need to expand the Result API as follows.
```protobuf
message Result {
  // Omitted for brevity

  Target target = 3;
}

// Target of Function results.
enum Target {
  TARGET_UNSPECIFIED = 0;
  TARGET_COMPOSITE_ONLY = 1;
  TARGET_COMPOSITE_AND_CLAIM = 2;
}
```
The reason for having `TARGET_COMPOSITE_AND_CLAIM` and not `TARGET_CLAIM` is an
implementation limitation. This prevents more involved API changes, and this
is also consistent with existing behavior (func copies to XR, Crossplane copies
XR to Claim).

The following is an example of how a function author could use this behavior.
Note that this is just a sketch and may not be the final API.
```go
// import "github.com/crossplane/function-sdk-go/response"
response.Fatal(rsp, errors.New("The image provided does not exist or you are not authorized to use it.")).
  ConditionFalse("ImageReady", "NotFound").
  TargetCompositeAndClaim()
```

To support this behavior, the status of the Composite would need an additional
field `claimConditions`. This field will contain the types of conditions that
should be propagated to the Claim.
```yaml
# composite status
status:
  # The XR's condition types that should be back-propagated to the claim
  claimConditions: [DatabaseReady, ImageReady]
  # The XR's conditions
  conditions:
  - type: DatabaseReady
    status: True
    reason: Available
  - type: ImageReady
    status: False
    reason: NotFound
    message: The image provided does not exist or you are not authorized to use it.
```

### Allow Results to Set a Condition
We would like the function author to be able to set the Claim's status
conditions. This would allow the function author to clearly communicate the
state of the Claim with their users.

To allow the setting of conditions in the result, we will need to expand the
Result API as follows.
```protobuf
message Result {
  // Omitted for brevity

  // Optionally update the supplied status condition on all targets.
  // The result's reason and message will be used in the condition.
  optional Condition condition = 4;
}

message Condition {
  // Type of the condition, e.g. DatabaseReady.
  // 'Ready' and 'Synced' are reserved for use by Crossplane.
  string type = 1;

  // Status of the condition.
  Status status = 2;

  // Machine-readable PascalCase reason.
  string reason = 3;
}
```

An example of a function utilizing this new ability:
```go
// rb "github.com/crossplane/function-sdk-go/response/result/builder"
// const databaseReady = "DatabaseReady"
// const reasonUnauthorized = "Unauthorized"
// var messageUnauthorized = errors.New("You are unauthorized to access this resource.")
result := rb.Fatal(messageUnauthorized).
  TargetCompositeAndClaim().
  WithConditionFalse(databaseReady, reasonUnauthorized).
  Build()
response.AddResult(rsp, result)
```

## Advanced Usage Example
Lets say we are a team of platform engineers who have a Crossplane offering.
For each Claim, we wish to expose a set of conditions that users can expect to
exist which provide:
- the current status of the underlying resources
- any steps required by the user to remediate an issue

Lets say we have a claim that does the following..
1. Accepts an identifier to an existing database
1. Accepts an image to deploy
1. Configures a deployment that uses the image provided and is authenticated to
the database.

### Scenarios
Given a few different scenarios, users could expect to see the following
`status.conditions` for the claim.

#### Image Not Found
First we found the database and determined that the user has authorization,
however, the image they provided was not found.

An example of the Claim's status:
```yaml
status:
  conditions:
  - type: DatabaseReady
    status: True
    reason: Available
  - type: ImageReady
    status: False
    reason: NotFound
    message: The image provided does not exist or you are not authorized to use
             it.
```
#### Progressing
All is fine and the application is progressing but not yet fully online.

An example of the Claim's status:
```yaml
status:
  conditions:
  - type: DatabaseReady
    status: True
    reason: Available
  - type: ImageReady
    status: True
    reason: Available
  - type: AppReady
    status: False
    reason: Creating
    message: Waiting for the deployment to be available.
```

#### Success
Once everything is online and running smoothly, users should see something like
this.

An example of the Claim's status:
```yaml
status:
  conditions:
  - type: DatabaseReady
    status: True
    reason: Available
  - type: ImageReady
    status: True
    reason: Available
  - type: AppReady
    status: True
    reason: Available
```

## Further Reading
- [k8s typical status properties](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties)
