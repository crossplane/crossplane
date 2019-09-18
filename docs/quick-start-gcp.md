---
title: Provision a MySQL Database on GCP
toc: true
weight: 240
indent: true
---
# Provisioning a MySQL Database on GCP

Now that Crossplane and the GCP stack are installed in your Kubernetes cluster,
and your GCP account credentials have been configured in
`crossplane-gcp-provider-key.json`, a MySQL database can easily be provisioned
on GCP.

## Namespaces

Namespaces allow for logical grouping of resources in Kubernetes. For this
example, create one for GCP-specific infrastructure resources, and one for
portable application resources:

```bash
kubectl create namespace gcp-infra-dev
kubectl create namespace app-project1-dev
```

## Provider

The `Provider` and `Secret` resources work together to store your GCP account
credentials in Kubernetes. Because these resources are GCP-specific, create them
in the `gcp-infra-dev` namespace. You will need to set the
`$BASE64ENCODED_GCP_PROVIDER_CREDS` and `$PROJECT_ID` variables before creating:

```bash
export PROJECT_ID=[your-demo-project-id]
export BASE64ENCODED_GCP_PROVIDER_CREDS=$(base64 crossplane-gcp-provider-key.json | tr -d "\n")
```

```bash
cat > provider.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: example-provider-gcp
  namespace: gcp-infra-dev
type: Opaque
data:
  credentials.json: $BASE64ENCODED_GCP_PROVIDER_CREDS
---
apiVersion: gcp.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: example
  namespace: gcp-infra-dev
spec:
  credentialsSecretRef:
    name: example-provider-gcp
    key: credentials.json
  projectID: $PROJECT_ID
EOF

kubectl apply -f provider.yaml
```

## Cloud-Specific Resource Class

Cloud-specific resource classes define configuration for a specific type of
service that is offered by a cloud provider. GCP provides a managed MySQL
database offering through its CloudSQL service. The GCP stack in Crossplane has
a `CloudsqlInstanceClass` resource that allows us to store configuration details
for a CloudSQL instance. Because this resource is specific to GCP, create it in
the `gcp-infra-dev` namespace:

```bash
cat > cloudsql.yaml <<EOF
apiVersion: database.gcp.crossplane.io/v1alpha2
kind: CloudsqlInstanceClass
metadata:
  name: standard-cloudsql
  namespace: gcp-infra-dev
specTemplate:
  databaseVersion: MYSQL_5_7
  tier: db-n1-standard-1
  region: us-central1
  storageType: PD_SSD
  storageGB: 10
  ipv4Address: true
  providerRef:
    name: example
    namespace: gcp-infra-dev
  reclaimPolicy: Delete
EOF

kubectl apply -f cloudsql.yaml
```

## Portable Resource Class

Portable resource classes define a class of service for an abstract resource
type (e.g. a MySQL database) that could be fulfilled by any number of managed
service providers. They serve to direct any portable claim types (see below) to
a cloud-specific resource class that can satisfy their abstract request. Create
a `MySQLInstanceClass` in the `app-project1-dev` namespace:

```bash
cat > mysql-class.yaml <<EOF
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: mysql-standard
  namespace: app-project1-dev
classRef:
  kind: CloudsqlInstanceClass
  apiVersion: database.gcp.crossplane.io/v1alpha2
  name: standard-cloudsql
  namespace: gcp-infra-dev
EOF

kubectl apply -f mysql-class.yaml
```

## Portable Resource Claim

Portable resource claims serve as abstract requests to provision a service that
can fulfill their specifications. In this example, we have specified that a
request for a `standard-mysql` database will be fulfilled by GCP's CloudSQL
service. Creating a `MySQLInstance` claim for a `standard-mysql` database in the
same namespace as our `MySQLInstanceClass` will provision a `CloudsqlInstance`
on GCP:

```bash
cat > mysql-claim.yaml <<EOF
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: app-mysql
  namespace: app-project1-dev
spec:
  classRef:
    name: mysql-standard
  writeConnectionSecretToRef:
    name: mysqlconn
  engineVersion: "5.6"
EOF

kubectl apply -f mysql-claim.yaml
```

## Observe

When the claim is created, Crossplane creates a `CloudSQLInstance` resource in
the `gcp-infra-dev` namespace, which mirrors the CloudSQL MySQL database created
in GCP. You can use the following commands to view your `CloudSQLInstance`:

```bash
$ kubectl -n gcp-infra-dev get cloudsqlinstances
NAME                                                 STATUS   STATE            CLASS               VERSION     AGE
mysqlinstance-516345d1-d9af-11e9-a1f2-4eae47c3c2d6            PENDING_CREATE   standard-cloudsql   MYSQL_5_6   3m
```

After some time, GCP will finish creating the CloudSQL database and Crossplane
will inform you that the `STATUS` is `Bound` and the `STATE` is `RUNNABLE`:

```bash
$ kubectl -n gcp-infra-dev get cloudsqlinstances
NAME                                                 STATUS   STATE      CLASS               VERSION     AGE
mysqlinstance-516345d1-d9af-11e9-a1f2-4eae47c3c2d6   Bound    RUNNABLE   standard-cloudsql   MYSQL_5_6   5m
```

You can also login to the GCP [console] to view your resource!

## Clean Up

The CloudSQL database on GCP and all Crossplane resources in your Kubernetes
cluster can be deleted with the following commands:

```bash
kubectl delete -f mysql-claim.yaml
kubectl delete -f mysql-class.yaml
kubectl delete -f cloudsql.yaml
kubectl delete -f provider.yaml
```

<!-- Named Links -->


[console]: https://console.cloud.google.com
