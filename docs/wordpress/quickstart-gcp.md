## Wordpress on Google Cloud Platform (GCP)

### Pre-requisites

Google service account credentials are needed for two separate accounts, these must be created before starting this Wordpress example.

| Description | Details | Required roles |
| ----- | --------- | ----------- |
| SQL admin | Service account that can perform Cloud SQL admin operations such as creating a new database instance | `roles/cloudsql.admin` |
| SQL client | Service account that can connect to Cloud SQL databases and run SQL commands | `roles/cloudsql.client` |

Please refer to the [targeting GCP section](../../troubleshooting.md#targeting-google-cloud-platform-gcp) for details on how to create these accounts with the required roles from the table above.
After the accounts are created, you should have two JSON key files on your local filesystem:

* `crossplane-gcp-provider-key.json`
* `crossplane-gcp-sql-key.json`

## Set environment variables

First, set the following environment variables that will be used in this walkthrough:

```
export PROVIDER=gcp
export DATABASE_TYPE=cloudsqlinstances
```

### Deploy Wordpress Resources

Next, create a `demo` namespace:

```console
kubectl create namespace demo
```

Deploy the GCP provider object to your cluster:

```console
sed "s/BASE64ENCODED_GCP_PROVIDER_CREDS/`cat crossplane-gcp-provider-key.json|base64|tr -d '\n'`/g" cluster/examples/wordpress/gcp/class/provider.yaml | kubectl create -f -
```

Now deploy all the Wordpress resources, including the Cloud SQL database, with the following single command:

```console
sed "s/BASE64ENCODED_GCP_SQL_CREDS/`cat crossplane-gcp-sql-key.json|base64|tr -d '\n'`/g" cluster/examples/wordpress/gcp/class/wordpress.yaml | kubectl -n demo create -f -
```

Now you can proceed back to the main quickstart to [wait for the resources to become ready](quickstart.md#waiting-for-completion).
