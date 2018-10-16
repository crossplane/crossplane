# Database Class Claim
This document proposes a model for how resources managed by Conductor will be created and consumed.

## Objective
A well defined abstraction model for managed resource definitions, including their instantiation and consumption, that 
facilitates a flexible and robust mechanism to support a "separation of concerns" between cluster administrators and 
application developers. Application developers should be able to focus on the high-level general needs of their application deployment
("my app needs a database"), while cluster admins should focus on the details of resource deployment within their 
specific operating environments ("databases should use AWS db.t2.small instances to save on costs").

- Cluster Administrator - defines and provides configuration for resource classes
- Application Developer - create (claims) instances of the resource by utilizing resource classes predefined by the 
administrator


## Overview
Conductor leverages Kubernetes Operator (CRD's and Controllers) to provision, update and delete resources managed by the cloud providers.


### RDSInstance
RDSInstance represents a managed resource hasted by AWS Cloud provider. 
RDSInstance may host one or many RDSInstanceDatabase(s), as well as support one ore many RDSInstanceUser(s) with various 
permission levels.


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
```

Submitting above CRD to Conductor enabled Kubernetes cluster will result in RDSInstance creation on AWS cloud provider, 
identified by the Cloud Provider Reference.
- `providerRef.name`: name of the cloud (in this case AWS) provider
- `template`: RDS Instance create parameters. Note: the initial implementation only supports properties defined above, 
however, the vision is to support a [full set](https://docs.aws.amazon.com/cli/latest/reference/rds/create-db-instance.html)
(or as close to it as possible).

### RDSInstanceBinding
To consume the database resource, the user must create an RDSInstanceBinding with following specs:
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstanceBinding
metadata:
  name: demo-rds
spec:
  instanceRef:
    name: demo-rds
  database: 
    name: demo-database
    user: demo-user
    passord: demo-password # Optional, random generated if not provided
    passwordSecretName: demo-rds # Optional, if not provided RDSInstanceBinding will be used for the secret name
    reclaimPolicy: retains
```
- `instanceRef.name`: name of the RDSInstance object to establish binding to
- `database`: database reference
    - `name`: name of the database that, will be created (if doesn't exist)
    - `user`: name of the database user, will be created (if doesn't exist)
    - `password`: database user's password value, will be randomly generated if not provided
**Note**: password values provided via binding definition are not securely stored and should not be use in production
systems. 
    - `connectionSecretName`: name of the Kubernetes secret object that will be created as a result of the binding and 
    contain database connection properties [see section below](#RDSInstanceBindingConnectionSecret). 
    **Note**: if secret value is not provided, RDSInstanceBinding name will be used as a secret name value.  
    - `reclaimPolicy`: supported polices: 
        - `retain`: database and user information left intact and are subject for manual reclamation 
        - `delete`: database and user information is deleted from the `RDSInstance`
        
    **Note/Important** - we need to decide how to address database/user collision, i.e. do we allow bindings to existing 
    database/user resources, and if so - what is the reclamation ramifications. 

### RDSInstanceBindingConnectionSecret
RDS Instance Binding will store connection information inside created secret.  

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
type: dbconnection.v1alpha1.core.conductor.io
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
    - The underlying CustomSecretDefinition (CSD) type that this Secret should conform to. Validation of this secret can
     be performed against the schema defined by CSD type stored in this field.


## Class - Claim Concept
To provide the separation of concerns, we can "abstract" managed resource CRD via Resource Class and Resource Claim.

### RDSInstanceClass
RDSInstanceClass provides a way for administrators to describe the "classes" of RDSInstances they offer. Different 
classes might map to quality-of-service levels, or to replication/backup policies, or to arbitrary policies supported 
by RDS resource adn determined by the cluster administrator. Kubernetes itself is unopinionated about what classes 
represent. This concept is inspired by [Kubernetes Storage Classes](https://kubernetes.io/docs/concepts/storage/storage-classes/) 

Each `RDSInstanceClass` contains the fields `parameters` and `reclaimPolicy`

    **Note**: as we generalize this concept into `MySqlDatabaseClass`, we can specify an additional field: provisioner

The name of a `RDSInstnaceClass` object is significant, and is how users can request a particular class. Administrators 
set the name and other parameters of a class when first creating `RDSInstanceClass` objects, and the objects cannot be 
updated once they are created.

Administrators can specify a default `RDSInstanceClass` just for `RDSInstanceClaim`s that don’t request any particular 
class to bind to.

```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstanceClass
metadata:
  name: standard
spec:
  providerRef:
    name: my-aws-provider
  template:    
    class: db.t2.small
    engine: postresql
    masterUsername: masteruser
    securityGroups:
    #  - vpc-default-sg - default security group for your VPC
    #  - vpc-rds-sg - security group to allow RDS connection
    size: 10
```

### RDSInstanceClaim
Each `RDSInstanceClaim` contains a spec and status, which is the specification and status of the claim.

```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstanceClaim
metadata:
  name: myclaim
spec:
  binding:
    name: demo-database
    user: demo-user      
  # reference to the resource class instance
  instanceClassName: standard
```

- `binding` defines binding information for a given database/user, see [RDSInstanceBinding](#RDSInstanceBinding) section.
- `className`  A claim can request a particular class by specifying the name of a `RDSInstanceClass` using attribute 
`instanceClassName`. Only `RDSInstanceClaim`s of the requested class, ones with the same `instanceClassName` can be 
bound together.

`RDSInstanceClaim` don’t necessarily have to request a class. A `RDSInstanceClaim` with its `instanceClassName` set 
equal to "" is always interpreted to be requesting a `RDSInstance` with no class, so it can only be bound to 
`DefaultRDSInstanceClass` if such has been defined. If there `DefaultRDSInstnaceClass`, `RDSInstanceClaim` will end up
in the failed to bound state.


## Other Thoughts

### RDSInstance
RDSInstance is a base building block for using Managed RDS DB Instance on AWS

- Input: RDSInstance (spec)
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
      ## RDSInstance Master User Password Secret Name
      masterUserPasswordSecretName: demo-rds-password
    ```
- Output: 
    - Kubernetes Service: 
        ```yaml
        kind: Service
        apiVersion: v1
        metadata:
          name: demo-rds
        spec:
          type: ExternalName
          externalName: my-db.cdgefbnnyfl5.us-east-1.rds.amazonaws.com
        ```
    - Kubernetes Secret
        ```yaml
        apiVersion: v1
        kind: Secret
        metadata:
          name: demo-rds
        data:
          Password: password
        ```
- Requirements
    - AWS Provider        
- Lifecycle Sequence
    - User creates RDSInstance
    - User creates a Deployment providing:
        - database service name
        - database master user name
        - database master user password secret reference
    - User deletes a Deployment
    - User deletes RDSInstance
        - all RDSInstance artifacts (service, secret, etc) are removed automatically
 
### RDSInstanceBinding
RDSInstanceBinding leverages existing RDSInstances to create and use (bind) specific Database with the specific Database User account

- Input: RDSInstanceBinding
    ```yaml
    apiVersion: database.aws.conductor.io/v1alpha1
    kind: RDSInstanceBinding
    metadata:
      name: demo-rds-sockshop
    spec:
      ## RDSInstance Reference
      rdsInstanceRef:
      ## Binding Specs
      databaseName: shockshop
      databaseUser: shockshop-user
    ```
- Output:
    - Kubernetes Secret
        ```yaml
        apiVersion: v1
        kind: Secret
        metadata:
          name: demo-rds-sockshop
        data:
          Password: password
        ```
- Requirements
    - RDSInstance
- Lifecycle Sequence
    - User creates RDSInstanceBinding
    - User creates a Deployment providing:
        - database service name
        - database name (same as in binding)
        - database user name (same as in binding)
        - database user password secret reference
    - User deletes Deployment
    - User deletes RDSInstanceBinding
        - RDSInstanceBinding password secret deleted automatically
     

### RDSInstanceClass
RDSInstanceClass provide a separation of concerts and facilitates dynamic provisioning of RDSInstances

- Input:
    ```yaml
    apiVersion: database.aws.conductor.io/v1alpha1
    kind: RDSInstanceClass
    metadata:
      name: postress-dev
    spec:
      providerRef:
        name: my-aws-provider
      template:    
        class: db.t2.small
        engine: postresql
        masterUsername: masteruser
        securityGroups:
        #  - vpc-default-sg - default security group for your VPC
        #  - vpc-rds-sg - security group to allow RDS connection
        size: 10
    ```
- Output: None
    - RDSInstanceClass acts merely as RDSInstance Template data placeholder and does create any additional resource, hence,
    does not require an active reconciliation.
- Requirements:
    - AWS Provider
- Lifecycle Sequence
    - User creates RDSInstanceClass
    - User deletes RDSInstanceClass
    
### RDSInstanceClaim
RDSInstanceClaim provides a mechanism of selecting existing RDSInstance for binding or creating new one based on the RDSInstanceClass specification.

- Input: 
    ```yaml
    apiVersion: database.aws.conductor.io/v1alpha1
    kind: RDSInstanceClaim
    metadata:
      name: demo-postgress
    spec:
      rdsInstanceClassName: postgres-dev
      resources:
        binding:
          name: demo-database
          user: demo-user      
    ```
- Output:
    - Existing RDSInstance
    If RDSInstance found with matching class definition, RDSInstanceClaim will attempt to create RDSInstanceBinding. 
    For RDSInstanceBinding output see [RDSInstanceBinding section](#RDSInstanceBinding)    
    - New RDSInstance                   
    If no RDSInstance found matching class definition, new RDSInstance will be created. Upon successful RDSInstance creation, 
    RDSInstanceBinding will be created as well. For RDSInstance output see [RDSInstance section](#RDSInstance). For 
    `RDSInstanceBinding` see [RDSInstanceBinding section](#RDSInstanceBinding).    
- Lifecycle Sequence:
    