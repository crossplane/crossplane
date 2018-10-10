# Resource Creation/Consumption Model
The proposal model for the Conduction resources creation and consumption

## Objective
Well defined abstraction model for managed resource definition, instantiation and consumption to facilitate fexible and yet at the same time robust mechanism to support the "separation of concerns":
- Cluster Administrator - defines and provides configuration for resource classes
- Application Developer - create (claims) instances of the resource by utilizing predefined resource classes 

## Overview
Conductor leverages Kubernetes Operator (CRD's and Controllers) to provision, update and delete resources managed by the cloud providers.

Example of managed resources: RDSInstance mysql/postgres (AWS)

```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstance
metadata:
  name: demo-rds
spec:
  ## Cloud Provider Reference
  providerRef:
    name: demo-aws-provider
  ## Database Specs
  class: db.t2.small
  engine: mysql
  masterUsername: masteruser
  securityGroups:
  #  - vpc-default-sg - default security group for your VPC
  #  - vpc-rds-sg - security group to allow RDS connection
  size: 20
  ## Connection Secret produced upon successful RDSInstance cration
  connectionSecretRef:
    name: demo-rds-connection
```


Submitting above CRD to Conductor enabled Kubernetes cluster will result in RDSInstance creation on AWS cloud provider, identified by the Cloud Provider Reference.

Upon successful RDSInstance creation, the RDSInstance conductor controller will create a Kubernetes Secreted identified by the `connectionSecretRef` and containing
database connection information.

##### TODO-1
We need to decide on the convention of the secret content per the resource Type/Kind, for example: `database/mysql`
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dockerhub
  namespace: conductor-system
data:
  Endpoint:         db-connection-url
  Username:         db-user-name
  Password:         db-user-password
  ConnectionString: db-connection-string
type: kubernetes.io/dockerconfigjson
```

##### TODO-2 
We need to decide on the naming convention of the produced artifacts. Currently the name value is explicitly defined as a part of the resource spec.
The alternative model could be: The resource spec defines a suffix value of the resource, however, the prefix is value is always a resource name value.

For example:
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstance
metadata:
  name: demo-rds
spec:
...
  connectionSecretRef:
    name: connection
  databaseConfigMapRef:
    name: stuff
```
Results in following artifacts
- secret: demo-rds-connection
- configmap: demo-rds-stuff


## Resource Class - Claim Concept
To provide the separation of concerns, we can "abstract" managed resource CRD via Resource Class and Reource Claim.

### Class
Resource Classes are defined by Cluster/System administrators and tailored/configured with specific primitives for a given managed resource

#### Examples

##### Spec-less
Example of the resource class that does not provide any `spec` definitions
```yaml
apiVersion: resource.conductor.io/v1alpha1
kind: Class
metadata:
  name: foo
spec:
  apiVersion: foo.conductor.io/v1alpha1
  kind: BarInstance
```

##### RDSInstances
###### Cheap Instance
```yaml
apiVersion: resource.conductor.io/v1alpha1
kind: Class
metadata:
  name: postgresql-cheap
spec:
  # apiVersion + Kind- required, type must be installed prior
  apiVersion: database.aws.conductor.io/v1alpha1
  kind: RDSInstance
  # spec - a generic blob - not part of the Class type definition, however, must be a valid spec for above apiVersion + kind
  spec:
    providerRef:
      name: my-aws-provider
    class: db.t2.small
    engine: postresql
    masterUsername: masteruser
    securityGroups:
    #  - vpc-default-sg - default security group for your VPC
    #  - vpc-rds-sg - security group to allow RDS connection
    size: 10
```
###### Pricey Instance
```yaml
apiVersion: resource.conductor.io/v1alpha1
kind: Class
metadata:
  name: mysql-pricey
spec:
  # apiVersion + Kind- required, type must be installed prior
  apiVersion: database.aws.conductor.io/v1alpha1
  kind: RDSInstance
  # spec - a generic blob - not part of the Class type definition, however, must be a valid spec for above apiVersion + kind
  spec:
    providerRef:
      name: my-aws-provider
    class: db.m4.xlarge
    engine: mysql
    masterUsername: masteruser
    securityGroups:
    #  - vpc-default-sg - default security group for your VPC
    #  - vpc-rds-sg - security group to allow RDS connection
    size: 100
```

### Claim
To create and consume an instance of the given class we define a `Claim` CRD which contains following:
- Reference to the specific class (class must exists prior utilization)
- Input parameters 
    - **TODO-3**: input parameters concept is yet to be flashed out
    
#### Examples

Using above RDSInstances example, user can create claim to create and consume RDS Database Instances:

##### Cheap Postresql Instance Claim
```yaml
apiVersion: resource.conductor.io/v1alpha1
kind: Claim
metadata:
  name: wordpess-dev-postgresql
spec:
  # reference to the resource class instance
  classRef: posgresql-cheap
  # class specification - to define input parameters, optional
  classSpec:
  # input parameters - not part fo the type definition, however, must be a valid properties
  # - masteruserName: foo-bar
```
Results in:
- New RDSInstance on AWS Cloud Provider via:
    - New RDSInstance CRD with name: `wordpress-dev-postresql`
- New Kubernetes secret with name: `wordpress-dev-postres-connection` 
    - **Note**: assuming we adopted naming convention in [TODO-2]()


##### Pricey MySQL Instance Claim
```yaml
apiVersion: resource.conductor.io/v1alpha1
kind: Claim
metadata:
  name: wordpess-prod-postgres
spec:
  # reference to the resource class instance
  classRef: mysql-pricey
  # class specification - to define input parameters, optional
  classSpec:
  # input parameters - not part fo the type definition, however, must be a valid properties
  # - masteruserName: foo-bar
```
Results in:
- New RDSInstance on AWS Cloud Provider via:
    - New RDSInstance CRD with name: `wordpress-prod-mysql`
- New Kubernetes secret with name: `wordpress-prod-mysql-connection` 
    - **Note**: assuming we adopted naming convention in [TODO-2]()

## Implementation
We will use Kubernetes Operator(s) to implement Class-Claim paradigm

### Resource Class

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: classes.resource.conductor.io
spec:
  group: resource.conductor.io
  names:
    kind: Class
    plural: classes
  scope: Not-Namespaced 
  validation:
    openAPIV3Schema:
      properties:
        apiVersion:
          type: string
        kind:
          type: string
        metadata:
          type: object
        spec:
          properties:
            apiVersion:
              type: string
            kind:
              type: string
          type: object
        status:
          type: object
  version: v1alpha1
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
```

What makes this CRD special is that its specification (spec) is defined by two section:
- Declared (Defined in Type):
    - apiVesion - version of the Class Resource (RDSInstance, GKECluster, etc)
    - kind - same as above 
- Undeclared: declared resource type specification

Because we need to define different CRD Resources as classes, we cannot provide "strong typed" definition. This is the main/only reason why we need to use "undeclared" spec section.

```go
// ClassSpec defines specification of this Class
type ClassSpec struct {
	metav1.TypeMeta `json:",inline"`
}
``` 

As you can see, the `ClassSpec` contains only inline `TypeMeta`, which provides following properties:
- `apiVersion`
- `kind`

This implies that we cannot use only `Client` interface provided by `controller-runtime`:
```go
import "sigs.k8s.io/controller-runtime/pkg/client"
...
c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
...
instance := &Class{}
c.Get(context.TODO(), key, instance)
```

If we were, then trying to retrieve following Class Instance:
```yaml
apiVersion: resource.conductor.io/v1alpha1
kind: Class
metadata:
  name: mysql-pricey
spec:
  # apiVersion + Kind- required, type must be installed prior
  apiVersion: database.aws.conductor.io/v1alpha1
  kind: RDSInstance
  # spec - a generic blob - not part of the Class type definition, however, must be a valid spec for above apiVersion + kind
  spec:
    providerRef:
      name: my-aws-provider
    class: db.m4.xlarge
    engine: mysql
    masterUsername: masteruser
    securityGroups:
    #  - vpc-default-sg - default security group for your VPC
    #  - vpc-rds-sg - security group to allow RDS connection
    size: 100
``` 
Would result in:
```yaml
apiVersion: resource.conductor.io/v1alpha1
kind: Class
metadata:
  name: mysql-pricey
spec:
  # apiVersion + Kind- required, type must be installed prior
  apiVersion: database.aws.conductor.io/v1alpha1
  kind: RDSInstance

```