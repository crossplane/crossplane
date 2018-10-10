# Resource Creation/Consumption Model
This document proposes a model for how resources managed by Conductor will be created and consumed.

## Objective
A well defined abstraction model for managed resource definitions, including their instantiation and consumption, that facilitates a flexible and robust mechanism to support a "separation of concerns" between cluster administrators and application developers. App devs should be able to focus on the high-level general needs of their application deployment ("my app needs a database"), while cluster admins should focus on the details of resource deployment within their specific operating environments ("databases should use AWS db.t2.small instances to save on costs").

- Cluster Administrator - defines and provides configuration for resource classes
- Application Developer - create (claims) instances of the resource by utilizing resource classes predefined by the administrator


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
  ## Connection Secret produced upon successful RDSInstance creation
  connectionSecretName: demo-rds-connection
    
```


Submitting above CRD to Conductor enabled Kubernetes cluster will result in RDSInstance creation on AWS cloud provider, identified by the Cloud Provider Reference.

Upon successful RDSInstance creation, the RDSInstance conductor controller will create a Kubernetes Secreted identified by the `connectionSecretRef` and containing
database connection information.

### Connection Secret
It is expected that RDSInstance controller will create a Connect Secret with following format

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: name
  namespace: namespace
data:
  URL:              db-connection-url
  Username:         db-user-name
  Password:         db-user-password
  ConnectionString: db-connection-string
type: core.conductor.io/dbconnection
```

- name: 
    - can either be set as an explicit value by using the `connectionSecretName` value from the RDSInstanceSpec
    - if `connectionSecretName` is not provided, use RDSInstance name for the connection secret name
- namespace: 
    - the same namespace as RDSInstance
- data:
    - URL: (required) database host:port or otherwise endpoint for establishing the connection
    - Username: (required) database user name
    - Password: (required) database user password
- type: `core.conductor.io/dbconnection` 
    - The underlying CustomSecretDefinition (CSD) type that this Secret should conform to. Validation of this secret can be performed against the schema defined by CSD type stored in this field.


## Class - Claim Concept
To provide the separation of concerns, we can "abstract" managed resource CRD via Resource Class and Resource Claim.

### RDSInstanceClass
RDSInstanceClass Spec is virtually identical to RDSInstance Spec, with the exception of `ConnectionSecretName`,
which is expected to be defined inside the `Claim`.

Using this paradigm, cluster administrator can tailor RDSInstanceClass using specific configuration. 
 
#### Cheap Instance
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstanceClass
metadata:
  name: cheap
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
#### Pricey Instance
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstanceClass
metadata:
  name: pricey
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

### RDSInstanceClaim
To create and consume an instance of the given class we define a `Claim` CRD

RDSInstanceClaim Spec is virtually identical to RDSInstance Spec, with the exception of `ProviderRef`,
which is expected to be defined inside the `Class`.

Using above RDSInstances example, user can create claim to create and consume RDS Database Instances:
##### Cheap Postresql Instance Claim
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: Claim
metadata:
  name: wordpess-postgres-dev
spec:
  # reference to the resource class instance
  classRef: cheap
  # connectionSecretName: my-special-connection
```
Results in:
- New RDSInstance on AWS Cloud Provider via:
    - New RDSInstance CRD with name: `wordpress-postres-dev`
- New Kubernetes secret with name: `wordpress-postgres-dev` 
    - **Note**: To use different secret name, simply uncomment `connectionSecretName` property and provide desired name

##### Pricey MySQL Instance Claim
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: Claim
metadata:
  name: wordpess-mysql-prod
spec:
  # reference to the resource class instance
  classRef: mysql-pricey
  ## Database Spec overrides
  masterUsername: root
  size: 200
  ## Connection Secret produced upon successful RDSInstance creation
  connectionSecretName: super-secret
```
Results in:
- New MySQL RDSInstance on AWS Cloud Provider via:
    - New RDSInstance CRD with name: `wordpress-postres-prod`
- New Kubernetes secret with name: `super-secret` 
