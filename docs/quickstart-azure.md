# Wordpress on Microsoft Azure

## Pre-requisites

Azure service principal credentials are needed for two separate accounts, these must be created before starting this Wordpress example.

| Description | Details | Required roles |
| ----- | --------- | ----------- |
| SQL admin | Service principal that can perform SQL admin operations such as creating a new database instance | `TODO` |
| SQL client | Service principal that can connect to SQL databases and run SQL commands | `TODO` |

Please refer to the [targeting Azure section](./troubleshooting.md#targeting-microsoft-azure) for details on how to create these accounts with the required permissions from the table above.
After the accounts are created, you should have 2 files on your local filesystem:

* `conductor-azure-provider-key.json`
* `conductor-azure-sql-key.json`

## Set environment variables

First, set the following environment variables that will be used in this walkthrough:

```
export PROVIDER=azure
export DATABASE_TYPE=sqldatabases
```

## Deploy Wordpress Resources

Next, create a `demo` namespace:

```console
kubectl create namespace demo
```

Deploy the Azure provider object to your cluster:

```console
sed "s/BASE64ENCODED_AZURE_PROVIDER_CREDS/`cat conductor-azure-provider-key.json|base64|tr -d '\n'`/g" cluster/examples/wordpress/azure/provider.yaml | kubectl -n demo create -f -
```

Now deploy all the Wordpress resources, including the SQL database, with the following single command:

```console
sed "s/BASE64ENCODED_AZURE_SQL_CREDS/`cat conductor-azure-sql-key.json|base64|tr -d '\n'`/g" cluster/examples/wordpress/azure/wordpress.yaml | kubectl -n demo create -f -
```

Now you can proceed back to the main quickstart to [wait for the resources to become ready](./quickstart.md#waiting-for-completion).
