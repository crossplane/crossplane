# Resource High Level Stop-Light Status

- Owner: Steven Rathbauer ([@rathpc](https://github.com/rathpc))
- Reviewers: Crossplane Maintainers
- Status: Defunct

## Problem

As a stack author there is currently no way to author rules that combine arbitrary conditions of a given state to
represent high level _"stop-light"_ conditions.

The high level conditions should be:
- Online
  > _When a resource instance is fully available for use with no errors or failures_
- Offline
  > _When a resource instance is **not** available for use, potentially with many errors and failures_
- Warning
  > _When a resource instance is **Online** but may have some errors or failures that do not affect its "Readiness"_
- Unknown
  > _This would be returned if the conditions were annotated but we cannot verify a known state based on the given rules._
  > _(Essentially a default case in a switch block)_

## Design

This design is meant to handle combinations of arbitrary statuses to determine a high level "overall" status.

The following examples are meant to be added to a `ui-schema.yaml` file. The `resourceStatus` key should be added as a root key in that file.

<a name="example-1"></a>

```yaml
# Example 1
resourceStatus:
  paths:
    JSONPath: .status.conditionedStatus.conditions
  states:
    Online:
    - Ready: 'True'
      Synchronized: 'True'
    Offline:
    - Ready: 'False'
      Synchronized: 'False'
    Warning:
    - Ready: 'False'
      Synchronized: 'True'
      SyncFailure: 'True'
```

<a name="example-2"></a>

```yaml
# Example 2
resourceStatus:
  paths:
    JSONPath: .status.conditionedStatus.conditions # Example 2a
    customConditions: # Example 2b
      Ready:
        JSONPath: .status.conditionedStatus.conditions[?(@.type=='Ready')]
      Synchronized:
        JSONPath: .status.conditionedStatus.conditions[?(@.type=='Synchronized')]
      SyncFailure:
        JSONPath: .status.conditionedStatus.conditions[?(@.type=='SyncFailure')] # Example 2c
        customConditionProps: # Example 2d
          status: .status.someRandomStatuses.SyncFailure.value
          message: .status.someRandomStatuses.SyncFailure.text
          lastTransitionTime: .status.someRandomStatuses.SyncFailure.updatedTime
  states:
    Online:
    - Ready: True
      Synchronized: True
      SyncFailure: False
    - Ready: True
      Synchronized: True
      SyncFailure: not 'True'
    Offline:
    - Ready: False
      Synchronized: False
    - Ready: True
      Synchronized: not 'True'
      SyncFailure: True
    Warning:
    - Ready: 'False'
      Synchronized: 'True'
      SyncFailure: 'True'
```

There are two main keys within `resourceStatus`. The first is `paths` which is an object.

To expand on how to set the `paths` key a bit:

- If the **states** are using status types that can all be found within a single conditions array that follow the object
structure of a standard [PodCondition], then you can just define a `JSONPath` key set to the JSONPath of that conditions
array. (_[Example 1](#example-1)_)

- If the states are using status types from various or arbitrary locations, then you can individually set each type as a
**key** of that status type name which will be an object within a `customConditions` key inside of the `paths` key.
Then, just like in the previous example, you can just define a `JSONPath` key set to the JSONPath of that specific
condition object. (_[Example 2](#example-2)_) (_[Example 2b](#example-2)_)

- If you define both a _base_ `JSONPath` key as well as keys within a `customConditions` key, the `customConditions`
keys will act as overrides if the same keys exist in the conditions array you provided. (_[Example 2a](#example-2)_)

- Lastly, if you want to be very explicit about the individual values of a particular status type, you can set JSONPaths
for each individual condition key within a `customConditionProps` key. (_[Example 2d](#example-2)_)

- Similarly as above, if you define both a _base_ `JSONPath` key as well as keys within a `customConditionProps` key,
the `customConditionProps` keys will act as overrides if the same props exist in the condition object you provided.
(_[Example 2c](#example-2)_)

- As a rule of thumb, you will need to define the `status`, `message` and `lastTransitionTime` for a path key, at the
bare minimum.

The `states` key should follow these rules:

- There are 3 nested keys which you should define:
  > `Online`

  > `Offline`

  > `Warning`

  - There is technically another state (`Unknown`) but that is defined automatically as a default status catch.

  _You can define other states if you wish, but the 3 mentioned above are the **standard recommended states**_

- Each key should contain an array of objects. Each object contains a collection of **AND** comparisons, and each object
within the array is treated as **OR** comparisons to adjacent objects.

  - For example, here is some code explaining what the Online state in example 2 equates to:

    ```js
    if (
      (
        Ready == 'True' &&
        Synchronized == 'True' &&
        SyncFailure == 'False'
      ) || (
        Ready == 'True' &&
        Synchronized == 'True' &&
        SyncFailure != 'True'
      )
    ) {
      // Do something for Online state
    }
    ```

- The keys in the objects should correlate to the condition types found in the `paths` key and the values should be
whatever is required to meet that particular criteria.

  - You can also define a logical not by typing `not` before the value you are comparing against so that you can
  specify that a certain key **should not** equal a particular value.


[PodCondition]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#podcondition-v1-core