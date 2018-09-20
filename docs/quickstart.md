# Conductor Quickstart

## Install Conductor

Install Conductor in a GKE cluster first, for example with the following `helm` command after setting your preferred values for `image.repository` and `image.tag` in the `values.yaml` file:

```bash
helm install --name conductor --namespace conductor-system ${GOPATH}/src/github.com/upbound/conductor/cluster/charts/conductor
```

## Wordpress on Google Cloud Platform (GCP)

### Pre-requisites

Google service account credentials are needed for two separate accounts, these must be created before starting this Wordpress example.

| Description | Details | Required roles |
| ----- | --------- | ----------- |
| SQL admin | Service account that can perform Cloud SQL admin operations such as creating a new database instance | `roles/cloudsql.admin` |
| SQL client | Service account that can connect to Cloud SQL databases and run SQL commands | `roles/cloudsql.client` |

Please refer to the [targeting GCP section](./troubleshooting.md#targeting-google-cloud-platform-gcp) for details on how to create these accounts with the required roles from the table above.
After the accounts are created, you should have two JSON key files on your local filesystem:

* conductor-gcp-provider-key.json
* conductor-gcp-sql-key.json

### Deploy Wordpress Resources

First create a `demo` namespace:

```console
kubectl create namespace demo
```

Deploy the GCP provider object to your cluster:

```console
sed "s/BASE64ENCODED_GCP_PROVIDER_CREDS/`cat conductor-gcp-provider-key.json|base64|tr -d '\n'`/g" cluster/examples/wordpress/gcp/provider.yaml | kubectl -n demo create -f -
```

Now deploy all the Wordpress resources, including the Cloud SQL database, with the following single command:

```console
sed "s/BASE64ENCODED_GCP_SQL_CREDS/`cat conductor-gcp-sql-key.json|base64|tr -d '\n'`/g" cluster/examples/wordpress/gcp/wordpress.yaml | kubectl -n demo create -f -
```

### Waiting for Completion

We'll need to wait for the Wordpress pod to get to the `Running` status, check on it with:

```console
kubectl -n demo get pod
```

While the Cloud SQL database is being deployed, you'll see the Wordpress pod in the `CreateContainerConfigError` status for awhile.
Follow along with the database deployment progress by watching the Conductor logs:

```console
kubectl -n conductor-system logs -f $(kubectl -n conductor-system get pod -l app=conductor -o jsonpath='{.items[0].metadata.name}')
```

You can also watch the resources over time with the following watch command:
```console
watch -t -n1 'echo CONDUCTOR-SYSTEM PODS && kubectl get pods -n conductor-system -o wide && echo && \
    echo PODS && kubectl get pods -n demo -o wide && echo && \
    echo SERVICES && kubectl -n demo get svc -o wide \
    && echo && echo CLOUD SQL INSTANCES && kubectl -n demo get cloudsqlinstances \
    && echo && echo NODES && kubectl get nodes -o wide'
```

Once the Wordpress pod is in the `Running` status and the Wordpress service has a valid `EXTERNAL-IP`, we can move to the next section to connect to it.

### Connecting to Wordpress

Retrieve the full URL of the Wordpress instance with the following command (note that the service's load balancer may take a bit to become ready):

```bash
echo http://$(kubectl -n demo get service wordpress -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

Copy and paste the URL into a web browser and you should see the Wordpress setup page.

### Cleanup & Teardown

When you are finished with your Wordpress instance, you can delete the resources from your cluster with:

```console
kubectl -n demo delete -f cluster/examples/wordpress/gcp/wordpress.yaml
kubectl -n demo delete -f cluster/examples/wordpress/gcp/provider.yaml
```