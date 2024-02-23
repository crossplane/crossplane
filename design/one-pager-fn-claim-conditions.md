# Communication Between Composition Functions and the Claim

* Owner: Dalton Hill (@dalton-hill-0)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background
Composition Function authors should be able to communicate with users. 
Topics of communication include:
- The status of underlying resources.
- Errors that need to be resolved by the user.
- Internal errors.

## Existing Behavior
Currently the only path to communicate with the user is by defining a custom field in the Claim's status.
For example, we can define an XRD with:
```yaml
status:
  someCommunicationField:
    - msg: "Something went wrong."
```

There are a couple issues with this solution.
- If we need to halt resource reconciliation due to a fatal error, we can do so with the [SDK](https://github.com/crossplane/function-sdk-go)'s `response.Fatal`, however, this does not also allow us to update the XR and Claim for communication with the user.
- There is an existing field that would be more intuitive to use as it is already performing this same task for Crossplane itself (`status.conditions`).

## Proposal
Allow the Composition Function author to set conditions inside the Claim's `status.conditions` field.

From the Function author's perspective, they would just need to update the desired XR as follows:
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
      Message:            "The image provided does not exist or you are not authorized to use it.",
  }
  desiredXR.Resource.SetConditions(c)
  response.SetDesiredCompositeResource(rsp, desiredXR)
```

Technically the behavior above is currently supported, though it has a couple limitations in its current form.
- it only updates the XR's conditions (not the Claim)
- it only updates if there were no fatal results returned by the function

After implementing the proposed solution, these conditions would be seen on the Claim as well as the XR.
Additionally, these conditions would also be seen when encountering a fatal result.

## Implementation
From the Crossplane side, the flow will look like this:
- Always copy \*custom `status.conditions` from the desired XR to the XR itself, even when a fatal result is encountered.
- Copy \*custom `status.conditions` from the XR to the Claim
- Clean up any \*custom `status.condtions` from the Claim that were not seen from the most recent XR.

*\* Custom conditions: Any condition that is not of type `Ready` or `Synced`. `Ready` and `Synced` are used
internally by Crossplane, so we will not allow Function authors to override these.*

## Advanced Usage Example
Lets say we are a team of platform engineers who have a Crossplane offering.
For each Claim, we wish to expose a set of conditions that users can expect to exist which provide:
- the current status of the underlying resources
- any steps required by the user to remediate an issue

Lets say we have a claim that does the following..
1. Accepts an identifier to an existing database
1. Accepts an image to deploy
1. Configures a deployment that uses the image provided and is authenticated to the database.

### Scenarios
Given a few different scenarios, users could expect to see the following `status.conditions` for
the claim.

#### Image Not Found
First we found the database and determined that the user has authorization, however, the image they
provided was not found.

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
    message: The image provided does not exist or you are not authorized to use it.
  - type: AppReady
    status: Unknown
    reason: PreviousErrors
    message: There were previous errors which prevented us from updating this condition.
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
Once everything is online and running smoothly, users should see something like this.

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

### Team Implementation
To accomplish this behavior, the team would need to configure their function pipeline to have the following
behavior.
1. The first step of the pipeline must always "reserve" the expected `status.conditions` on the desired XR.
  In this example above, this would be to create an entry for all three condition types
  (`DatabaseReady`, `ImageReady`, `AppReady`) and set each to a default of:
  ```yaml
    - type: <type>
      status: Unknown
      reason: PreviousErrors
      message: There were previous errors which prevented us from updating this condition.
  ```
  This is required in the case that we exit early due to an error. If we did not pre-populate this and we hit an error
  before creating an entry, the Claim will remove that condition from it's status, assuming the condition is no longer
  desired.
1. As we reach points in the function where we wish to update a specific condition, we can do so.
  ```go
  c := xpv1.Condition{...}
  desiredXR.Resource.SetConditions(c)
  response.SetDesiredCompositeResource(rsp, desiredXR)
  ```

## Alternatives Considered

### Events
In our search for providing communication to the Claim, we considered giving Composition Function 
authors the ability to send events to the Claim, however, we believe the proposal above is preferred
as it provides the ability to communicate with users in a more structured way.

## Further Reading
- [k8s typical status properties](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties)
