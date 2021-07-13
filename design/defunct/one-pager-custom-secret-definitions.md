# Custom Secret Definitions
* Owner: Bassam Tabbara (@bassam)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Abstract

Secrets in Kubernetes are objects that hold sensitive data like passwords, tokens and keys. Secrets can be consumed directly from Pods and mounted as volumes or through environment variables, and as result present a powerful way to pass sensitive config to a pod.

While secrets are objects/resources themselves they do not have a schema or spec, like other objects. The payload of a secret is an arbitrary map of values. Secrets do however have a `type` field that specifies the payload type and is typically set to `Opaque`. For example, `kubernetes.io/service-account-token` and `kubernetes.io/dockercfg`.

For tools that consume and generate secrets it's desirable to define the schema for a secret, in the same way that we define a schema for resource via Customer Resource Definitions (CRDs). This would enable a consistent representation of secrets that can be validated, and consumed by the eco-system of tools building on-top of the Kubernetes API.

This proposal explores the idea of a Custom Secret Definition (CSD).

## Design

We borrow heavily from CRDs in this design. Here's an example, CSD:

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: CustomSecretDefinition
metadata:
  name: mysql.database.crossplane.io
spec:
  group: database.crossplane.io
  names:
    kind: mysql
  version: v1alpha1
  validation:
    openAPIV3Schema:
      properties:
        username:
          type: string
        password:
          type: string
        required:
        - username
        - password
```

this would serve as the schema for the following secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mysecret
type: mysql.v1alpha1.database.crossplane.io
data:
  username: YWRtaW4=
  password: MWYyZDFlMmU2N2Rm
```

The secret could can be validated but that would require a validating webhook. For now, the CSD could merely be used as the documentation for the secret schema.

