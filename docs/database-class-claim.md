# Database Class Claim
This document proposes a model for how resources managed by Conductor will be created and consumed.

# Objective
A well defined abstraction model for managed resource definitions, including their instantiation and consumption, that 
facilitates a flexible and robust mechanism to support a "separation of concerns" between cluster administrators and 
application developers. Application developers should be able to focus on the high-level general needs of their application deployment
("my app needs a database"), while cluster admins should focus on the details of resource deployment within their 
specific operating environments ("databases should use AWS db.t2.small instances to save on costs").

- Cluster Administrator - defines and provides configuration for resource classes
- Application Developer - create (claims) instances of the resource by utilizing resource classes predefined by the 
administrator


# Overview
Conductor leverages Kubernetes Operator (CRD's and Controllers) to provision, update and delete resources managed by the cloud providers.

# Terminology
This design proposal is inspired and influenced by Kubernetes `PersistentVolume`(`PV`), `PersistentVolumeClaim`(`PVC`), and `StorageClass`(`SC`) with respective:
- `PersistentDatabase` (`PD`)
- `PersistentDatabaseClaim` (`PDC`)
- `DatabaseClass` (`DC`)

# PersistentDatabase
`PresistentDatabase` represents a database resource, which could be represented (actualized) by any of following supported databases:
- `RDSInstance`: AWS Managed database instance resource
- `CloudSQLInstance`: GCP Managed database instance resource
- `AzureSQLIntance`: (Not sure if this is correct terminology) 

`PD` is defined at the cluster-level, i.e. `non-namespaced` resource.

```yaml
    apiVersion: database.core.conductor.io/v1alpha1
    kind: PersistentDatabase
    metadata:
      name: my-name
    spec:
      # Generic Database specs
      # Database engine type, must be supported by the database plugin underlying the PersistentDatabase
      # - mysql
      # - postgres
      engine:
      # Database version for a given type (engine), must be supported by the database plugin
      version: 
      # A description of the persistent database's resources and capacity
      capacity: # Object
      
      # Supported Database plugins. must be one of the following:
      awsRDSInstance:      # object
      azureSQLIntance:     # object
      gcpCloudSQLInstance: # object
      
      # ClaimRef(erence) part of a bi-directional binding between PersistentDatabase and PersistentDatabaseClaim.
      # Expected to be non-nil when bound. claim.DatabaseName is the authoritative bind between PD and PDC.
      claimRef: # ObjectReference
                  
      # Name of DatabaseClass to which this persistent database belongs. 
      # Empty value means that this database does not belong to any DatabaseClass.          
      databaseClassName: 
      
      # What happens to a persistent volume when released from its claim. Valid options are 
      # - Retain (default for manually created PersistentDatabases), 
      # - Delete (default for dynamically provisioned PersistentDatabases), 
      persistentDatabaseReclaimPolicy: Delete
      
    status:
      # Reference to the database instance
      databaseInstanceRef: # ObjectReference
      # PD phase/status 
      phase: Bound/Unbound/Failed
```

## Plugins
Conductor provides support fo following `PersistentDatabase` plugins:

**IMPORTANT**: While Plugins are cluster-level resources, the plugins' artifacts (`secret`) __**are namespaced resources**__

***Convention***: All conductor system resources artifacts are stored in the `conductor-system` namespace
  
### RDSInstance(Provisioner)
`RDSInstance` is AWS managed database resource.  

Requirements:
- AWS Provider        

#### Input
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstance
metadata:
  name: demo-rds
spec:
  # AWS Provider Reference
  providerRef:
    name: my-aws-provider
  # RDS Database Create Input as defined in https://docs.aws.amazon.com/cli/latest/reference/rds/create-db-instance.html
  class: db.t2.small
  engine: mysql
  masterUsername: masteruser
  securityGroups:
  #  - vpc-default-sg - default security group for your VPC
  #  - vpc-rds-sg - security group to allow RDS connection
  size: 20
  
```
#### Output:
RDSInstance (same as the above, but with the updated status)
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstance
metadata:
  name: demo-rds
### Same spec definition as above
status:      
  ## Upon successful provisioning RDSDBInstance endpoint is recorded into status
  endpoint: my-db.cdgefbnnyfl5.us-east-1.rds.amazonaws.com
  ## RDSInstanceSpecific status/phases:
  phase: # Pending, Running, Terminating, etc.
```

`RDSInstance` Secret contains instance's master user password:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: demo-rds-961bn
  namespace: conductor-system
data:
  Password: cGFzc3dvcmQK
```

### CloudSQLInstance(Provisioner)
#### Input
```yaml
apiVersion: database.gcp.conductor.io/v1alpha1
kind: CloudsqlInstance
metadata:
  name: cloudsql-demo
spec:
  # GCP provider Reference
  providerRef:
    name: my-gcp-provider
  
  # GCP Database Create Input as defined in: https://cloud.google.com/sdk/gcloud/reference/sql/instances/create
  databaseVersion: MYSQL_5_7
  memory: 9GiB
  region: us-west2
  storageType: PD_SSD
  storageSize: 10GB
  tier: db-n1-standard-1
```

### AzureSQLInstance
    
    **TODO**

# DatabaseClass
To dynamically provision a `PersistentData` with pre defined configurations, cluster administrators can define `DatabasClass`'es

`DatabaseClass` provides both: 
- `provisioner` which will be used to create new Database Instance, and must be one of the following (as of this writing):
    - `RDSInstance(Provisioner)`
    - `CloudSQLInstance(Provisioner)`
- `parameters` a sub-set of all values which will be used by a given provisioner. Note: the remaining values (for a complete set) are 
provided in `PVC` 

**Note** similar to `PersistentData`, `DatabaseClass` is a **non-namespaced** resource, i.e. defined at the cluster-level

**Important** There is no validation on neither `provisioner` nor `parameters` values at the class creation time. If `provisioner` or `parameters` values
are invalid or yield incorrect/incomplete combination - volume creation will fail at provisioning time.

```yaml
apiVersion: database.core.conductor.io/v1alpha1
kind: DatabaseClass
metadata:
  name: standard
spec:
  # Parameters holds the parameters for the provisioner that should create databases of this database class.
  parameters: # object
  
  # Provisioner indicates the type of the provisioner, could be one of the following:
  # - v1alpha1.database.aws.conductor.io/RDSInstance(Provisioner)
  # - v1alpha1.database.gcp.conductor.io/CloudSQLInstance(Provisioner)
  # - v1alpha1.database.azure.conductor.io/AzureSQLInstance(Provisioner)
  provisioner: # string 
  
  # Dynamically provisioned PersistentDatabases of this storage class are created with this reclaimPolicy. Defaults to Delete.
  reclaimPolicy: Delete
  
  # DatabaseBindingMode indicates how PersistentDatabaseClaims should be provisioned and bound. When unset, DatabaseBindingImmediate is used. 
  # TBD: This field is only honored by servers that enable the DatabaseScheduling feature.
  databaseBindingMode: Immediate 
```

## Example: DatabaseClassForRDS
```yaml
apiVersion: database.core.conductor.io/v1alpha1
kind: DatabaseClass
metadata:
  name: standard
spec:
  parameters:
    providerRef:
      name: demo-aws-provider
    class: db.t2.small
    engine: mysql
    masterUsername: masteruser
    securityGroups:
    #  - vpc-default-sg - default security group for your VPC
    #  - vpc-rds-sg - security group to allow RDS connection
  provisioner: v1alpha1.database.aws.conductor.io/RDSInstance(Provisioner) 
  reclaimPolicy: Retain
  databaseBindingMode: Immediate 
```

## Example: DatabaseClassForCloudSQL
```yaml
apiVersion: database.core.conductor.io/v1alpha1
kind: DatabaseClass
metadata:
  name: standard
spec:
  parameters:
    providerRef:
      name: my-gcp-provider
    region: us-west2
    storageType: PD_SSD
    tier: db-n1-standard-1  
  provisioner: v1alpha1.database.gcp.conductor.io/CloudSQLInstance(Provisioner) 
  reclaimPolicy: Retain
  databaseBindingMode: Immediate 
```

# PersistentDatabaseClaim
To consume the database resource, the user must request (claim) on of the available Database instances

**Note** Unlike `DatabaseClass` or `PersistentDatabase`, `Perc` is a `namespaced` resource and typically provisioned into the same namespace
as the consuming application (deployment/pod) 

```yaml
apiVersion: database.core.conductor.io/v1alpha1
kind: RDSInstanceClaim
metadata:
  name: my-claim
  namespace: demo
spec:
  # Name of the DatabaseClass required by the claim
  databaseClassName: string 
  # Database engine: mysql, postgres. Must be supported by databaseClass
  databseEngine: mysql
  # DatabaseName is the binding reference to the PersistentDatabase backing this claim.
  databaseName: string
  # Database version specific to a given engine.
  databaseVersion: string
  # Resources represents the minimum resources the database should have
  resources:
    requests: 
      size: 10
  # A label query over databases to consider for binding.
  selector: # LabelSelector
```

## Example: PersistentDatabaseClaimForRDS

Input: `PersistentDatabaseClaim`
```yaml
apiVersion: database.core.conductor.io/v1alpha1
kind: RDSInstanceClaim
metadata:
  name: demo-mysql
  namespace: demo
spec:
  databaseClassName: standard  
  databseEngine: mysql
  databaseVersion: 5.7
  resources:
    requests: 
      size: 20
```

Input: `DatabaseClass`
```yaml
apiVersion: database.core.conductor.io/v1alpha1
kind: DatabaseClass
metadata:
  name: standard
spec:
  parameters:
    providerRef:
      name: demo-aws-provider
    class: db.t2.small
    engine: mysql
    masterUsername: masteruser
  provisioner: v1alpha1.database.aws.conductor.io/RDSInstance(Provisioner) 
  reclaimPolicy: Retain
  databaseBindingMode: Immediate 
```

Condition: no database instances are running (available)

Output: `PersistentDatabase`
```yaml
apiVersion: database.core.conductor.io/v1alpha1
kind: PersistentDatabase
metadata:
  name: demo-mysql
spec:
  engine: mysql
  version: 5.7 
  capacity:
    size: 10
  awsRDSInstance:
    ## TODO - not 100% what goes here, seems like the actual RDSInstanceSpec
    providerRef:
      name: my-aws-provider
    class: db.t2.small
    engine: mysql
    masterUsername: masteruser
    size: 20    
  claimRef: 
    name: demo-mysql
    namespace: demo
    # other fields
  databaseClassName: standard 
  persistentDatabaseReclaimPolicy: Delete 
```
 
Output: `RDSInstance(Provisioner)`
```yaml
apiVersion: database.aws.conductor.io/v1alpha1
kind: RDSInstance
metadata:
  name: demo-mysql
  namespace: demo
spec:
  providerRef:
    name: my-aws-provider
  class: db.t2.small
  engine: mysql
  masterUsername: masteruser
  size: 20
```

Output: `Secret` in `conductor-system` namespace
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: demo-rds-961bn
  namespace: conductor-system
data:
  Password: cGFzc3dvcmQK
```

Output `Secret` in application's namespace
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: demo-rds-933bn
  namespace: demo
data:
  Password: cGFzc3dvcmQK
```

Output: `Service` in application's namespace
```yaml
kind: Service
apiVersion: v1
metadata:
  name: demo-rds-933bn
  namespace: demo
spec:
  type: ExternalName
  externalName: my-db.cdgefbnnyfl5.us-east-1.rds.amazonaws.com
```