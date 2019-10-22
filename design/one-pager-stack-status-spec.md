# Stack UI Schema Status Spec

- Owner: Steven Rathbauer (@rathpc)
- Reviewers: Crossplane Maintainers
- Status: Draft

## Proposal

[crossplaneio/crossplane#929](https://github.com/crossplaneio/crossplane/issues/929)

## Problem

For a given CRD I want to be able to expose various statuses to a frontend GUI. Initially it makes
sense to at least have the Ready state available. In addition to the value of the Ready state it
would also be helpful to have some additional details about that state if applicable.

This also assumes that statuses are following a particular condition convention, similar to that of a
[podcondition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#podcondition-v1-core)

The conditions should contain at least the following keys:
- type
- status
- reason
- message

#### For example:

If a CRD has the condition type 'Ready' defined but has a status of that type of
_"False"_, I could show some visual indication to a user through a GUI that the CRD is **NOT Ready**.

When that status changes to _"True"_ then I can adjust that visual cue to now reflect that the CRD is
in fact **Ready**.

As of right now there is no defined spec to be able to drive status information for a given resource from a GUI perspective. It would be useful to add a key to the existing `ui-schema.yaml`
file that can be used for this purpose.

This key would contain an array of JSONPath's that point directly to conditions following the convention described above.

This 1-pager will identify the key that should be added and what that structure looks like.

## Design

#### Add two new keys to the `ui-schema.yaml` file:

- `resourceConditionPaths` _(this is an array of JSONPaths)_

#### The arrays could potentially look like this inside of the `ui-schema.yaml` file:

```yaml
version: 0.4
configSections: 
- title: Title
  description: Description
resourceConditionPaths:
- .status.conditionedStatus.conditions[?(@.type=='Ready')]
```

- You can use that key to get the various values you want for a GUI related to that type.

- For example if you wanted a status title, status value and additional details for the 'Ready'
status type you could build an object like this for example (_Using JS for example purposes_):

```js
import { JSONPath } from 'jsonpath-plus';

// ...

const resourceConditionPathsFromUISchema = resourceConditionPaths;

const resourceStatusInfo = resourceConditionPathsFromUISchema.map(statusType => ({
    title: JSONPath({ path: `$${statusType}.type`, json: crdReturnedFromK8SAsJSON }),
    value: JSONPath({ path: `$${statusType}.status`, json: crdReturnedFromK8SAsJSON }),
    details: JSONPath({ path: `$${statusType}.reason`, json: crdReturnedFromK8SAsJSON }),
}));

// ...
```

- You can use a javascript implementation of JSONPath ([jsonpath-plus](https://github.com/s3u/JSONPath))
as shown above to leverage these keys and "_look up_" the status values and additional info if present.

-----

## Fallback Option

If a package author does not include these additional keys in the `ui-schema.yaml` file **OR** if
there is no `ui-schema.yaml` file included at all, you can try to leverage the
**additionalPrinterColumns** values if they exist within the spec.

```yaml
spec:
  additionalPrinterColumns:
    - JSONPath: .status.conditionedStatus.conditions[?(@.type=='Ready')].status
      name: READY
      type: string
```
