# WordPress Crossplane Workload on Microsoft Azure

## Set environment variables

Set the following environment variables that will be used in this walkthrough:

```
export PROVIDER=AZURE
export provider=azure
export CLUSTER_TYPE=aksclusters
export DATABASE_TYPE=mysqlservers
```

## Deploy Wordpress Resources

Create the Azure provider object in your cluster:

```console
sed "s/BASE64ENCODED_${PROVIDER}_PROVIDER_CREDS/`cat crossplane-${provider}-provider-key.json|base64|tr -d '\n'`/g;" cluster/examples/workloads/wordpress-${provider}/provider.yaml | kubectl create -f -
```

Next, create the AKS cluster that will eventually be the target cluster for your Workload deployment:

```console
kubectl create -f cluster/examples/workloads/wordpress-${provider}/cluster.yaml
```

It will take a while (~15 minutes) for the AKS cluster to be deployed and becoming ready.
You can keep an eye on its status with the following command:

```console
kubectl -n crossplane-system get akscluster -o custom-columns=NAME:.metadata.name,STATE:.status.state,CLUSTERNAME:.status.clusterName,ENDPOINT:.status.endpoint,LOCATION:.spec.location,CLUSTERCLASS:.spec.classRef.name,RECLAIMPOLICY:.spec.reclaimPolicy
```

Once the cluster is done provisioning, you should see output similar to the following (note the `STATE` field is `Succeeded` and the `ENDPOINT` field has a value):

```console
NAME                                       STATE       CLUSTERNAME                       ENDPOINT                                          LOCATION     CLUSTERCLASS       RECLAIMPOLICY
aks-587762b3-f72b-11e8-bcbe-0800278fedb1   Succeeded   aks-587762b3-f72b-11e8-bcbe-080   crossplane-aks-653c32ef.hcp.centralus.azmk8s.io   Central US   standard-cluster   Delete
```

Now that the target AKS cluster is ready, we can deploy the Workload that contains all the Wordpress resources, including the SQL database, with the following single command:

```console
kubectl create -f cluster/examples/workloads/wordpress-${provider}/workload.yaml
```

This will also take awhile to complete, since the MySQL database needs to be deployed before the Wordpress pod can consume it.
You can follow along with the MySQL database deployment with the following:

```console
kubectl -n crossplane-system get mysqlserver -o custom-columns=NAME:.metadata.name,STATUS:.status.state,CLASS:.spec.classRef.name,VERSION:.spec.version
```

Once the `STATUS` column is `Ready` like below, then the Wordpress pod should be able to connect to it:

```console
NAME                                         STATUS    CLASS            VERSION
mysql-58425bda-f72d-11e8-bcbe-0800278fedb1   Ready     standard-mysql   5.7
```

Now we can watch the Wordpress pod come online and a public IP address will get assigned to it:

```console
kubectl get workload -o custom-columns=NAME:.metadata.name,CLUSTER:.spec.targetCluster.name,NAMESPACE:.spec.targetNamespace,DEPLOYMENT:.spec.targetDeployment.metadata.name,SERVICE-EXTERNAL-IP:.status.service.loadBalancer.ingress[0].ip
```

When a public IP address has been assigned, you'll see output similar to the following:

```console
NAME            CLUSTER        NAMESPACE   DEPLOYMENT   SERVICE-EXTERNAL-IP
test-workload   demo-cluster   demo        wordpress    104.43.240.15
```

Once Wordpress is running and has a public IP address through its service, we can get the URL with the following command:

```console
echo "http://$(kubectl get workload test-workload -o jsonpath='{.status.service.loadBalancer.ingress[0].ip}')"
```

Paste that URL into your browser and you should see Wordpress running and ready for you to walk through the setup experience.

## Clean-up

First delete the workload, which will delete Wordpress and the MySQL database:

```console
kubectl delete -f cluster/examples/workloads/wordpress-${provider}/workload.yaml
```

Then delete the AKS cluster:

```console
kubectl delete -f cluster/examples/workloads/wordpress-${provider}/cluster.yaml
```

Finally, delete the provider credentials:

```console
kubectl delete -f cluster/examples/workloads/wordpress-${provider}/provider.yaml
rm -fr crossplane-${provider}-*
```