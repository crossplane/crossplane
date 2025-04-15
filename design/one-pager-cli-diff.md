# `crossplane beta diff` command

* Owner: Jonathan Ogilvie (@jcogilvie)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

The Crossplane community has [long desired](https://github.com/crossplane/crossplane/issues/1805) a mechanism to 
understand the would-be surface area of proposed changes to Crossplane resources. Platform administrators and 
application teams would like the ability to review changes before they are applied, especially in GitOps workflows. 
This requirement is particularly critical when working with complex compositions that result in multiple managed 
resources being created or updated.

Currently, users lack visibility into how changes to Composite Resources (XRs) or their configurations translate to 
changes in the underlying Managed Resources (MRs) and ultimately in the external systems. This makes it challenging to 
understand the impact of changes, troubleshoot issues, and confidently apply updates to production environments.

## Goals

The `crossplane beta diff` command aims to:

1. Provide a clear, familiar way to preview changes that would be made to external resources before they are applied
1. Support GitOps workflows by enabling change review within CI/CD pipelines
1. Enhance the debugging experience when working with complex compositions
1. Provide a familiar experience similar to `kubectl diff` or `argocd app diff` for Crossplane resources

## Non-Goals

1. Perform a dry-run in the server
1. Build a GUI for visualizing differences
1. Provide historical tracking of changes over time

## Proposal

We propose introducing a new `crossplane beta diff` command that shows the changes that would result from applying 
Crossplane resources to a live cluster. The command will process resources from files or stdin, compare them against the 
current state in the cluster, and display the differences in a familiar format.

### Basic Usage

The command's basic usage would be:

```
crossplane beta diff [FILE]...
```

Similar to `kubectl diff`, the command will:
1. Accept input from files or stdin (when `-` is specified)
2. Process multiple files when provided
3. Display a diff of the changes that would be made if the resources were applied

Examples:
```
# Show changes that would result from applying an XR from a file
crossplane diff xr.yaml

# Show changes from stdin
cat xr.yaml | crossplane diff -

# Process multiple files
crossplane diff xr1.yaml xr2.yaml xr3.yaml

# Specify namespace
crossplane diff -n other-namespace xr.yaml
```

### Architecture

The implementation consists of several components:

1. **Diff Command**: The main entrypoint that processes arguments, loads resources, and coordinates the diffing process
1. **Cluster Client**: Handles all interactions with the Kubernetes cluster, including fetching current resources and performing dry-run operations
1. **Diff Processor**: Computes diffs between desired and current state
1. **Diff Renderer**: Formats and displays the diffs in a user-friendly way

The process flow is:
1. Load resources from files/stdin
1. For each resource: 
   1. find the matching composition
   1. Render what the resources would look like if applied (using `render`)
   1. While there are unresolved Requirements, fetch them and try to `render` again 
   1. Validate the results against the schemas loaded into the cluster (using `beta validate`)
   1. Compare against current state in the cluster (using `beta trace` for child tracking)
1. Format and display differences

### Output Format

The output will follow familiar diff format conventions.  There will be a standard mode and a compact mode:

```
+++ Resource/new-resource-(generated)
+ apiVersion: nop.crossplane.io/v1alpha1
+ kind: NopResource
+ metadata:
+   annotations:
+     cool-field: I'm new!
+     crossplane.io/composition-resource-name: nop-resource
+     setting: value1
+   generateName: new-resource-
+   labels:
+     crossplane.io/composite: new-resource
+ spec:
+   forProvider:
+     conditionAfter:
+     - conditionStatus: "True"
+       conditionType: Ready
+       time: 0s

---
--- XNopResource/removed-resource-downstream
- apiVersion: diff.example.org/v1alpha1
- kind: XNopResource
- metadata:
-   name: removed-resource-downstream
- spec:
-   coolField: goodbye!
-   parameters:
-     config:
-       setting1: value1
-       setting2: value2

---

~~~ Resource/to-be-modified
  apiVersion: diff.example.org/v1alpha1
  kind: XNopResource
  metadata:
    name: to-be-modified
- spec:
-   oldValue: something
+ spec:
+   newValue: something-else
---

Summary: 1 added, 1 modified, 1 removed
```

The diff output will be colorized by default (can be disabled with `--no-color`), and supports a compact mode with the 
`--compact` flag that shows minimal context around changes.

```
###### modifications, compact with 2 lines of context:

~~~ Resource/to-be-modified
  metadata:
    name: to-be-modified
- spec:
-   oldValue: something
+ spec:
+   newValue: something-else
---

```


### Implementation Details

The diff command leverages the existing Crossplane and/or Kubernetes machinery to:
1. Find the appropriate composition for a composite resource
1. Extract dependency information
1. Perform a simulated reconciliation
1. Use a dry-run approach to determine the changes without applying them

The implementation should include:
- Rate limiting to prevent overloading the API server
- Proper error handling with descriptive messages
- Resource ownership tracking to ensure all dependent resources are considered

## Future Work

While the initial implementation provides significant value, future enhancements could include:

1. Diff against a server with local overrides (e.g. to validate against an unreleased schema)
1. Diff for an upgrade to a new composition version
1. Additional output formats (e.g. gnu .diff) for programmatic consumption
1. Integration with policy tools to evaluate changes against compliance rules
1. Web UI visualization for complex differences
1. Ability to save/export diffs for review or documentation purposes

## Alternatives Considered

1. **External diffing tool**: Building a completely separate tool outside of Crossplane. This would require duplicating 
   significant logic from crank and would be harder to maintain.
1. **Server-side diffing**: Implementing diffing functionality server-side in Crossplane controllers. While this would 
   potentially be more accurate and efficient, it would be more complex to implement and require changes to both the 
   server and the CLI.  Isolating our changes to the CLI reduces the blast radius significantly.
