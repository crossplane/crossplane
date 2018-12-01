---
title: Quick Start Guides
toc: true
weight: 110
indent: true
---
# Crossplane Quickstart

## Install Crossplane

Install Crossplane in a GKE cluster first, for example with the following `helm` command after setting your preferred values for `image.repository` and `image.tag` in the `values.yaml` file:

```bash
helm install --name crossplane --namespace crossplane-system ${GOPATH}/src/github.com/crossplaneio/crossplane/cluster/charts/crossplane
```

## Wordpress on AWS 

Follow the instructions in the [AWS quickstart](quickstart-aws.md) to start the process of running Wordpress on AWS

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
Follow along with the database deployment progress by watching the Crossplane logs:

```console
kubectl -n crossplane-system logs -f $(kubectl -n crossplane-system get pod -l app=crossplane -o jsonpath='{.items[0].metadata.name}')
```

You can also watch the resources over time with the following watch command:
```console
watch -t -n1 "echo crossplane-system PODS && kubectl get pods -n crossplane-system -o wide && echo && \
    echo PODS && kubectl get pods -n demo -o wide && echo && \
    echo SERVICES && kubectl -n demo get svc -o wide && echo && \
    echo MYSQL CLAIMS && kubectl -n demo get mysqlinstance mysql-instance -o jsonpath='{.metadata.name}{\"\t\"}{.status.bindingPhase}{\"\t\"}{range .status.Conditions[*]}{.Type}{\"=\"}{.Status}{\"\t\"}{end}' && echo && echo &&\
    echo MYSQL INSTANCES && kubectl -n crossplane-system get ${DATABASE_TYPE} -o jsonpath='{range .items[*]}{.metadata.name}{\"\t\"}{.status.bindingPhase}{\"\t\"}{.status.state}{\"\t\"}{range .status.Conditions[*]}{.Type}{\"=\"}{.Status}{\"\t\"}{end}{end}' && echo && \
    echo && echo NODES && kubectl get nodes -o wide"
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