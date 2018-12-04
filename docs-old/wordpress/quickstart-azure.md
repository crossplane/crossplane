# Wordpress on Microsoft Azure

## Pre-requisites

Azure service principal credentials are needed for an admin account, which must be created before starting this Wordpress example.

| Description | Details | Required roles |
| ----- | --------- | ----------- |
| SQL admin | Service principal that can perform SQL admin operations such as creating a new database instance | `TODO` |

Please refer to the [targeting Azure section](../../troubleshooting.md#targeting-microsoft-azure) for details on how to create this account with the required permissions from the table above.
After the account is created, you should have 1 file on your local filesystem:

* `crossplane-azure-provider-key.json`

## Set environment variables

First, set the following environment variables that will be used in this walkthrough:

```
export PROVIDER=azure
export DATABASE_TYPE=mysqlservers
```

## Deploy Wordpress Resources

Next, create a `demo` namespace:

```console
kubectl create namespace demo
```

Deploy the Azure provider object to your cluster:

```console
sed "s/BASE64ENCODED_AZURE_PROVIDER_CREDS/`cat crossplane-azure-provider-key.json|base64|tr -d '\n'`/g" cluster/examples/wordpress/azure/class/provider.yaml | kubectl create -f -
```

Now deploy all the Wordpress resources, including the SQL database, with the following single command:

```console
kubectl -n demo create -f cluster/examples/wordpress/azure/class/wordpress.yaml
```

Now you can proceed back to the main quickstart to [wait for the resources to become ready](quickstart.md#waiting-for-completion).
