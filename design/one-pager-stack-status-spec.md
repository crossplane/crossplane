# Stack UI Schema Status Spec

- Owner: Steven Rathbauer (@rathpc)
- Reviewers: Crossplane Maintainers
- Status: Draft

## Proposal

[crossplaneio/crossplane#929](https://github.com/crossplaneio/crossplane/issues/929)

## Problem

As of right now there is no defined spec to be able to drive status information for a given stack or resource from a GUI perspective. It would be useful to add keys to the existing `ui-schema.yaml` file that can be used for this purpose.

We can leverage the types from the array of **conditions** within the **conditionedStatus** key defined in **status**.

This 1-pager will identify the keys that should be added and what that structure looks like.

## Design

#### Add two new keys to the `ui-schema.yaml` file:

- `resourceConditionedStatusTypes` _(this is an array of condition types as strings)_
- `stackConditionedStatusTypes` _(this is an array of condition types as strings)_

#### The arrays could potentially look like this inside of the `ui-schema.yaml` file:

```yaml
version: 0.4
configSections: 
- title: Title
  description: Description
resourceConditionedStatusTypes:
- Ready
stackConditionedStatusTypes:
- Ready
```

- You can use that key to get the various values you want for a GUI related to that type.

- For example if you wanted a status title, status value and additional details for the 'Ready' status type you could build an object like this for example (_Using JS for example purposes_):

```js
import { JSONPath } from 'jsonpath-plus';

// ...

const statusTypesFromUISchema = resourceConditionedStatusTypes;

const statusInfo = statusTypesFromUISchema.map(statusType => ({
    title: JSONPath({
        path: `$.status.conditionedStatus.conditions[?(@.type=='${statusType}')].type`,
        json: crdReturnedFromK8SAsJSON,
    }),
    value: JSONPath({
        path: `$.status.conditionedStatus.conditions[?(@.type=='${statusType}')].status`,
        json: crdReturnedFromK8SAsJSON,
    }),
    details: JSONPath({
        path: `$.status.conditionedStatus.conditions[?(@.type=='${statusType}')].reason`,
        json: crdReturnedFromK8SAsJSON,
    }),
}));

// ...
```

- You can use a javascript implementation of JSONPath ([jsonpath-plus](https://github.com/s3u/JSONPath)) as shown above to leverage these keys and "_look up_" the status values and additional info if present.

-----

## Fallback Option

If a package author does not include these additional keys in the `ui-schema.yaml` file **OR** if there is no `ui-schema.yaml` file included at all, you can try to leverage the **additionalPrinterColumns** values if they exist within the spec.

```yaml
spec:
  additionalPrinterColumns:
    - JSONPath: .status.conditionedStatus.conditions[?(@.type=='Ready')].status
      name: READY
      type: string
```
