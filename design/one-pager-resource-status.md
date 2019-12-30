# Resource High Level Stop-Light Status

- Owner: Steven Rathbauer ([@rathpc](https://github.com/rathpc))
- Reviewers: Crossplane Maintainers
- Status: Draft

## Problem

As a stack author there is currently no way to author rules that combine arbitrary conditions of a given state to
represent high level _"stop-light"_ conditions.

The high level conditions would be:
- Online
- Offline
- Warning
- Unknown

The **Unknown** condition is interesting because this would be returned if the conditions were annotated but we cannot
verify a known state based on the given rules. _(Essentially a default case in a switch block)_.

## Design

In contrast to the design [here](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-stack-status-spec.md),
this design is meant to handle combinations of arbitrary statuses to determine a high level "overall" status.

The following examples are meant to be added to a `ui-schema.yaml` file. The `resourceStatus` key should be added as a root key in that file.

<a name="example-1"></a>

```yaml
# Example 1
resourceStatus:
  paths: .status.conditionedStatus.conditions
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
    Ready: .status.conditionedStatus.conditions[?(@.type=='Ready')]
    Synchronized: .status.conditionedStatus.conditions[?(@.type=='Synchronized')]
    SyncFailure: # Example 2a
      status: .status.someRandomStatuses.SyncFailure.value
      message: .status.someRandomStatuses.SyncFailure.text
      lastTransitionTime: .status.someRandomStatuses.SyncFailure.updatedTime
  states:
    Online:
    - Ready: 'True'
      Synchronized: 'True'
      SyncFailure: 'False'
    - Ready: 'True'
      Synchronized: 'True'
      SyncFailure: not 'True'
    Offline:
    - Ready: 'False'
      Synchronized: 'False'
    - Ready: 'True'
      Synchronized: not 'True'
      SyncFailure: 'True'
    Warning:
    - Ready: 'False'
      Synchronized: 'True'
      SyncFailure: 'True'
```

There are two main keys within `resourceStatus`. The first is `paths` which can be a string or map of key/value pairs. Additionally the key/value pairs can be set as <string>: <string> **OR** <string>: <object>.

To expand on how to set the `paths` key a bit:

- If the **states** are using status types that can all be found within a single conditions array that follow the object
structure of a standard [PodCondition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#podcondition-v1-core),
then you can just set the `paths` key to the JSONPath of that conditions array. (_[Example 1](#example-1)_)

- If the states are using status types from various or arbitrary locations, then you can individually set each path as a
**key**/**value** pair. (_[Example 2](#example-2)_)
  > \<status type> / \<JSONPath to condition object>

- Lastly, if you want to be very explicit about the individual values of a particular status type, you can set JSONPaths
for each individual condition key. (_[Example 2a](#example-2)_)

- As a rule of thumb, you will need to define the `status`, `message` and `lastTransitionTime` for a path key, at the
bare minimum.

The `states` key should follow these rules:

- There are 3 nested keys which you can define:
  > `Online`

  > `Offline`

  > `Warning`

  - There is technically a 4th state (`Unknown`) but that is defined automatically as a default status catch.

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

- The keys in the objects should relate to the condition types found in the `paths` key and the values should be
whatever is required to meet that particular criteria.

  - You can also define a logical not by typing `not` before the value you are comparing against so that you can
  specify that a certain key **should not** equal a particular value.