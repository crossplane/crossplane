# Deploying PostgreSQL Databases

This user guide will walk you through how to deploy a PostgreSQL database across many different environments with a focus on portability and reusability.
The database will be dynamically provisioned in the cloud provider of your choice at the request of the application developer via a `ResourceClaim` and created with the environment specific information that the administrator providers in a `ResourceClass`.
The commands in this guide assume you are running from a terminal/shell at the root of the [Crossplane repo](https://github.com/crossplaneio/crossplane/).

## Install Crossplane

The first step will be to install Crossplane and any desired cloud provider stacks by following the steps in the [Crossplane install guide](install-crossplane.md).

## Add Cloud Provider

Next you'll need to add your cloud provider credentials to Crossplane using [these provider specific steps](cloud-providers.md).

After those steps are completed, you should have the cloud provider credentials saved in a file on your local filesystem, for which the path will be stored in the environment variable `PROVIDER_KEY_FILE` in the next section.

## Set Environment Variables

After your cloud provider credentials have been created/added, let's set the following environment variables that have different values for each provider,
but will allow the rest of the steps to be consistent across all of them.
You only need to set the variables for your chosen cloud provider, you can ignore the other ones.

### Google Cloud Platform (GCP)

```console
export PROVIDER=GCP
export provider=gcp
export PROVIDER_KEY_FILE=crossplane-${provider}-provider-key.json
export DATABASE_TYPE=cloudsqlinstances
export versionfield=databaseVersion
```

### Microsoft Azure

```console
export PROVIDER=AZURE
export provider=azure
export PROVIDER_KEY_FILE=crossplane-${provider}-provider-key.json
export DATABASE_TYPE=postgresqlservers
export versionfield=version
```

### Amazon Web Services (AWS)

```console
export PROVIDER=AWS
export provider=aws
export PROVIDER_KEY_FILE=~/.aws/credentials
export DATABASE_TYPE=rdsinstances
export versionfield=engineVersion
```

## Create a PostgreSQL Resource Class

Let's create a `ResourceClass` that acts as a "blueprint" that contains the environment specific details of how a general request from the application to create a PostgreSQL database should be fulfilled.
This is a task that the administrator should complete, since they will have the knowledge and privileges for the specific environment details.

```console
sed "s/BASE64ENCODED_${PROVIDER}_PROVIDER_CREDS/`base64 ${PROVIDER_KEY_FILE} | tr -d '\n'`/g;" cluster/examples/database/${provider}/postgresql/provider.yaml | kubectl create -f -
kubectl create -f cluster/examples/database/${provider}/postgresql/resource-class.yaml
```

## Create a PostgreSQL Resource Claim

After the administrator has created the PostgreSQL `ResourceClass` "blueprint", the application developer is now free to create a PostgreSQL `ResourceClaim`.
This is a general request for a PostgreSQL database to be used by their application and it requires no environment specific information, allowing our applications to express their need for a database in a very portable way.

```console
kubectl create namespace demo
kubectl -n demo create -f cluster/examples/database/${provider}/postgresql/resource-claim.yaml
```

## Check Status of PostgreSQL Provisioning

We can follow along with the status of the provisioning of the database resource with the below commands.
Note that the first command gives us the status of the `ResourceClaim` (general request for a database by the application),
and the second command gives the status of the environment specific database resource that Crossplane is provisioning using the `ResourceClass` "blueprint".

```console
kubectl -n demo get postgresqlinstance -o custom-columns=NAME:.metadata.name,STATUS:.status.bindingPhase,CLASS:.spec.classRef.name,VERSION:.spec.engineVersion,AGE:.metadata.creationTimestamp
kubectl -n crossplane-system get ${DATABASE_TYPE} -o custom-columns=NAME:.metadata.name,STATUS:.status.bindingPhase,STATE:.status.state,CLASS:.spec.classRef.name,VERSION:.spec.${versionfield},AGE:.metadata.creationTimestamp
```

## Access the PostgreSQL Database

Once the dynamic provisioning process has finished creating and preparing the database, the status output will look similar to the following:

```console
> kubectl -n demo get postgresqlinstance -o custom-columns=NAME:.metadata.name,STATUS:.status.bindingPhase,CLASS:.spec.classRef.name,VERSION:.spec.engineVersion,AGE:.metadata.creationTimestamp
NAME                     STATUS    CLASS              VERSION   AGE
cloud-postgresql-claim   Bound     cloud-postgresql   9.6       2018-12-23T04:00:11Z

> kubectl -n crossplane-system get ${DATABASE_TYPE} -o custom-columns=NAME:.metadata.name,STATUS:.status.bindingPhase,STATE:.status.state,CLASS:.spec.classRef.name,VERSION:.spec.${versionfield},AGE:.metadata.creationTimestamp
NAME                                              STATUS    STATE     CLASS              VERSION   AGE
postgresql-3ef70bf9-0667-11e9-99e1-080027cf2340   Bound     Ready     cloud-postgresql   9.6       2018-12-23T04:00:12Z
```

Note that both the general `postgresqlinstance` `ResourceClaim` and the cloud provider specific PostgreSQL database have the `Bound` status, meaning the dynamic provisioning is done and the resource is ready for consumption.

The connection information will be stored in a secret specified via the `writeConnectionSecretTo` field.
Since the secret is base64 encoded, we'll need to decode its fields to view them in plain-text.
To view all the connection information in plain-text, run the following command:

```console
for r in endpoint username password; do echo -n "${r}: "; kubectl -n demo get secret cloud-postgresql-claim -o jsonpath='{.data.'"${r}"'}' | base64 -D; echo; done
```

A workload or pod manifest will usually reference this connection information through injecting the secret contents into environment variables in the manifest.
You can see this in action as an example in the [Azure Workload example](https://github.com/crossplaneio/crossplane/blob/release-0.1/cluster/examples/workloads/wordpress-azure/workload.yaml#L47-L62).

More information about consuming secrets from manifests can be found in the [Kubernetes documentation](https://kubernetes.io/docs/concepts/configuration/secret/#use-cases).

## Clean-up

When you are finished with the PostgreSQL instance from this guide, you can clean up all the resources by running the below commands.

First, delete the resource claim, which will start the operation of deleting the PostgreSQL database from your cloud provider.

```console
kubectl -n demo delete -f cluster/examples/database/${provider}/postgresql/resource-claim.yaml
```

Next. delete the `ResourceClass` "blueprint":

```console
kubectl delete -f cluster/examples/database/${provider}/postgresql/resource-class.yaml
```

Finally, delete the cloud provider credentials from your local environment:

```console
kubectl delete -f cluster/examples/database/${provider}/postgresql/provider.yaml
```
