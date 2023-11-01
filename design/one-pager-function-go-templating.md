# Go Templating Function
 
* Owner: Ezgi Demirel (@ezgidemirel)
* Reviewers: Crossplane Maintainers
* Status: Accepted

## Background

With Crossplane 1.14, Composition Functions will be promoted to `v1beta1`, and 
several composition functions will be introduced as an extension point to be 
used in pipelines. One of these extensions is the [Go Templating Function], 
which allows users to render Crossplane resources using Go templating 
capabilities like conditionals and loops. In addition to that, users can use 
values in the environment configs or other resource fields to create/configure
Crossplane resources.

## Getting Template Files

With the initial implementation of the Go Templating Function, users will be 
able to provide templates in two different ways:
- An `inline` source can be used to provide template files as a string in the
  `input.inline.template` field of the pipeline step.
- A `fileSystem` source can be used to provide template files with folder path
  in the `input.fileSystem.dirPath` field of the pipeline step.

Both of these options have their own advantages and disadvantages. While 
`inline` source is faster and easier to use, it is not suitable for large 
template files and hard to maintain. On the other hand, `fileSystem` source
requires the files to be present in the local file system, which might be 
possible by mounting config maps with `DeploymentRuntimeConfig` or building 
another container image on top of the existing one with the template files. 

In the next iterations, it is planned to support fetching template files from a
remote location like GitHub repositories or S3 buckets.

## Rendering Templates and Accessing Resources

To make rendering templates and accessing resource fields easier, function 
requests will be converted to `map[string]any` and passed to the 
template engine. This will allow users to access resource fields in the 
template files like:

```yaml
{{- range $i := until ( .observed.composite.resource.spec.count | int ) }}
---
apiVersion: iam.aws.upbound.io/v1beta1
kind: User
metadata:
  annotations:
    gotemplating.fn.crossplane.io/composition-resource-name: test-user-{{ $i }}
  name: test-user-{{ $i }}
  labels:
    testing.upbound.io/example-name: test-user-{{ $i }}
spec:
  forProvider: {}

---
apiVersion: iam.aws.upbound.io/v1beta1
kind: AccessKey
metadata:
  annotations:
    gotemplating.fn.crossplane.io/composition-resource-name: sample-access-key-{{ $i }}
  name: sample-access-key-{{ $i }}
spec:
  forProvider:
    userSelector:
      matchLabels:
        testing.upbound.io/example-name: test-user-{{ $i }}
  writeConnectionSecretToRef:
    name: sample-access-key-secret-{{ $i }}
    namespace: crossplane-system
{{- end }}

```

To compare and prepare the desired composed resources map with consistent keys,
`function-go-templating` requires a special annotation 
`gotemplating.fn.crossplane.io/composition-resource-name` to be assigned to the
input resources. This annotation will match the resource name in the template
file with the resource name in the desired resources.

## Configuring Composite Connection Details

A new meta resource, `CompositeConnectionDetails` with API version 
`meta.gotemplating.fn.crossplane.io/v1alpha1` will be used to set connection 
details of the composite resource. This resource will contain a `data` field,
which expects a map with string keys and base64 encoded string values. This 
meta resource is not a real Crossplane resource, and it will not be applied to
the cluster.

```yaml
apiVersion: meta.gotemplating.fn.crossplane.io/v1alpha1
kind: CompositeConnectionDetails
metadata:
  annotations:
    gotemplating.fn.crossplane.io/composition-resource-name: connection-details
  name: connection-details
{{ if eq $.observed.resources nil }}
data: {}
{{ else }}
data:
  username: {{ ( index $.observed.resources "sample-access-key-0" ).connectionDetails.username }}
  password: {{ ( index $.observed.resources "sample-access-key-0" ).connectionDetails.password }}
  url: {{ "http://www.example.com" | b64enc }}
{{ end }}

```

## Configuring Resource Status

In addition to that, a new custom annotation, 
`gotemplating.fn.crossplane.io/ready` will be introduced to set the ready 
status of the resource, which can be assigned to `Unspecified`, `True` or 
`False`. This annotation will be removed after the resource is rendered and not
applied to the cluster.

```yaml
apiVersion: kubernetes.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: composed-providerconfig
  annotations:
    gotemplating.fn.crossplane.io/ready: True
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: cluster-config
      key: kubeconfig
```

## Helper Functions

A set of helper functions like `getComposedResource` or `getCompositionEnvVar` will be 
provided to make it easier to access resources or environment config variables 
referenced in the composition. Without these functions, users can access 
resources or environment config variables by using the `index` function like 
below:

```yaml
{{- range $i := until ( .observed.composite.resource.spec.count | int ) }}
---
apiVersion: iam.aws.upbound.io/v1beta1
kind: User
metadata:
  annotations:
    gotemplating.fn.crossplane.io/composition-resource-name: test-user-{{ $i }}
  name: test-user-{{ $i }}
  labels:
    testing.upbound.io/example-name: test-user-{{ $i }}
  {{ if eq $.observed.resources nil }}
    dummy: {{ randomChoice "foo" "bar" "baz" }}
  {{ else }}
    dummy: {{ ( index $.observed.resources ( print "test-user-" $i ) ).resource.metadata.labels.dummy }}
  {{ end }}
    env: {{ (index $.context "apiextensions.crossplane.io/environment").key1 }}
spec:
  forProvider: {}
{{- end }}
```

With the helper functions, users can access resources or environment config 
variables like:

```yaml
{{- range $i := until ( .observed.composite.resource.spec.count | int ) }}
---
apiVersion: iam.aws.upbound.io/v1beta1
kind: User
metadata:
  annotations:
    gotemplating.fn.crossplane.io/composition-resource-name: test-user-{{ $i }}
  labels:
    testing.upbound.io/example-name: test-user-{{ $i }}
    {{ $composed := getComposedResource $.observed ( print "test-user-" $i ) }}
    dummy: {{ default ( randomChoice "foo" "bar" "baz" ) $composed.resource.metadata.labels.dummy }}
    env: {{ getCompositionEnvVar $. "key1" }}
  name: test-user-{{ $i }}
spec:
  forProvider: {}
{{- end }}
```

[Go Templating Function]: https://github.com/crossplane-contrib/function-go-templating