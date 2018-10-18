# Conductor Quickstart

## Install Conductor

Install Conductor in a GKE cluster first, for example with the following `helm` command after setting your preferred values for `image.repository` and `image.tag` in the `values.yaml` file:

```bash
helm install --name conductor --namespace conductor-system ${GOPATH}/src/github.com/upbound/conductor/cluster/charts/conductor
```

## Wordpress on Google Cloud Platform (GCP)

Follow the instructions in the [GCP quickstart](quickstart-gcp.md) to start the process of running Wordpress on Google Cloud Platform.

## Wordpress on Microsoft Azure

Follow the instructions in the [Azure quickstart](quickstart-azure.md) to start the process of running Wordpress on Microsoft Azure.

## Waiting for Completion

After finishing the specific instructions for your chosen cloud provider from the links above, we'll need to wait for the Wordpress pod to get to the `Running` status. Check on it with:

```console
kubectl -n demo get pod
```

While the database is being deployed, you'll see the Wordpress pod in the `CreateContainerConfigError` status for awhile.
Follow along with the database deployment progress by watching the Conductor logs:

```console
kubectl -n conductor-system logs -f $(kubectl -n conductor-system get pod -l app=conductor -o jsonpath='{.items[0].metadata.name}')
```

You can also watch the resources over time with the following watch command:
```console
watch -t -n1 'echo CONDUCTOR-SYSTEM PODS && kubectl get pods -n conductor-system -o wide && echo && \
    echo PODS && kubectl get pods -n demo -o wide && echo && \
    echo SERVICES && kubectl -n demo get svc -o wide \
    && echo && echo DATABASES && kubectl -n demo get ${DATABASE_TYPE} \
    && echo && echo NODES && kubectl get nodes -o wide'
```

Once the Wordpress pod is in the `Running` status and the Wordpress service has a valid `EXTERNAL-IP`, we can move to the next section to connect to it.

## Connecting to Wordpress

Retrieve the full URL of the Wordpress instance with the following command (note that the service's load balancer may take a bit to become ready):

```bash
echo http://$(kubectl -n demo get service wordpress -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

Copy and paste the URL into a web browser and you should see the Wordpress setup page.

## Cleanup & Teardown

When you are finished with your Wordpress instance, you can delete the resources from your cluster with:

```console
kubectl -n demo delete -f cluster/examples/wordpress/${PROVIDER}/wordpress.yaml
kubectl -n demo delete -f cluster/examples/wordpress/${PROVIDER}/provider.yaml
```