# Stack UI Schema Status Spec

- Owner: Steven Rathbauer ([@rathpc](https://github.com/rathpc))
- Reviewers: Crossplane Maintainers
- Status: Draft, revision 2.0

## Proposal

[crossplaneio/crossplane#929](https://github.com/crossplaneio/crossplane/issues/929)

## Problem

For a given CRD I want to be able to expose various condition statuses to a frontend GUI. In addition to the condition
type and status it would also be helpful to have some additional details about that type if applicable, such as message
or lastTransitionTime.

This should assume that condition statuses are following a particular condition convention, similar to that of a
[PodCondition]

The conditions should contain at least the following keys:
- type
- status
- message
- lastTransitionTime

#### For example:

If a CRD has the condition type 'Ready' defined but has a status of that type of _"False"_, you could then show some
visual indication to a user through a GUI that the CRD is **NOT Ready**.

When that status changes to _"True"_ then you can adjust that visual cue to now reflect that the CRD is in fact **Ready**.

As of right now there is no defined spec to be able to drive status information for a given resource from a GUI
perspective. It would be useful to add a key to the existing `ui-schema.yaml` file that can be used for this purpose.

This 1-pager will identify the key that should be added and what that structure looks like.

## Design

#### New key to be added to the `ui-schema.yaml` file:

- `printerColumns` _(map of key/value pairs)_

The `printerColumns` key could potentially look like this:

```yaml
version: 0.4
configSections: 
- title: Title
  description: Description
printerColumns:
  JSONPath: .status.conditionedStatus.conditions
```

- This means that you are defining the location of an array containing condition objects located at the JSONPath:
`.status.conditionedStatus.conditions`.

  - That path could contain any number of condition objects that can be exposed to a GUI.

  - Additionally this would assume that this array contains objects following the [PodCondition] convention. By default
  the column title is defined by the condition type, and the value is defined by the condition status.

The `printerColumns` key could also look like this:

```yaml
version: 0.4
configSections: 
- title: Title
  description: Description
printerColumns:
  Ready: .status.conditionedStatus.conditions[?(@.type=='Ready')]
  Synchronized: .status.conditionedStatus.conditions[?(@.type=='Synchronized')]
  SyncFailure: .status.someRandomStatuses[?(@.type=='SyncFailure')]
```

> \<status type>: \<JSONPath to condition object>

- This uses a map of key/value pairs to define the individual printer columns you would like to expose to a GUI.

- This means that you are defining the specific condition object per key(_column title_) located at a JSONPath.

  - Additionally this would assume that these paths also contain objects following the [PodCondition] convention.
  
    - However, now by default, the column title is defined by the `key` you have assigned a JSONPath value to.

[PodCondition]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#podcondition-v1-core