# Wordpress on AWS

## Deploy Wordpress Resources

Now deploy all the Wordpress provider and workload resources, including the RDS database, and EKS cluster with the following single commands:

Create provider
```console
kubectl create -f cluster/examples/workloads/wordpress-aws/provider.yaml
```

Create cluster
```console
kubectl create -f cluster/examples/workloads/wordpress-aws/cluster.yaml
```


Note: It will take about 10 minutes for the EKSCluster to become active, with about another 5 for the nodes to be started and join the cluster.

## Waiting for load balancer and things to be available

It will take a while (~15 minutes) for the EKS cluster to be deployed and becoming ready.
You can keep an eye on its status with the following command:

```console
kubectl -n crossplane-system get ekscluster -o custom-columns=NAME:.metadata.name,STATE:.status.state,CLUSTERNAME:.status.clusterName,ENDPOINT:.status.endpoint,LOCATION:.spec.location,CLUSTERCLASS:.spec.classRef.name,RECLAIMPOLICY:.spec.reclaimPolicy
```

Once the cluster is done provisioning, you should see output similar to the following (note the `STATE` field is `Succeeded` and the `ENDPOINT` field has a value):

```console
NAME                                       STATE      CLUSTERNAME   ENDPOINT                                                                   LOCATION   CLUSTERCLASS       RECLAIMPOLICY
eks-8f1f32c7-f6b4-11e8-844c-025000000001   ACTIVE     <none>        https://B922855C944FC0567E9050FCD75B6AE5.yl4.us-west-2.eks.amazonaws.com   <none>     standard-cluster   Delete
```

Now that the target EKS cluster is ready, we can deploy the Workload that contains all the Wordpress resources, including the SQL database, with the following single command:

```console
kubectl -n demo create -f cluster/examples/workloads/wordpress-${provider}/workload.yaml
```

This will also take awhile to complete, since the MySQL database needs to be deployed before the Wordpress pod can consume it.
You can follow along with the MySQL database deployment with the following:

```console
kubectl -n crossplane-system get rdsinstance -o custom-columns=NAME:.metadata.name,STATUS:.status.state,CLASS:.spec.classRef.name,VERSION:.spec.version
```

Once the `STATUS` column is `Ready` like below, then the Wordpress pod should be able to connect to it:

```console
NAME                                         STATUS      CLASS            VERSION
mysql-2a0be04f-f748-11e8-844c-025000000001   available   standard-mysql   <none>
```

Now we can watch the Wordpress pod come online and a public IP address will get assigned to it:

```console
kubectl -n demo get workload -o custom-columns=NAME:.metadata.name,CLUSTER:.spec.targetCluster.name,NAMESPACE:.spec.targetNamespace,DEPLOYMENT:.spec.targetDeployment.metadata.name,SERVICE-EXTERNAL-IP:.status.service.loadBalancer.ingress[0].ip
```

When a public IP address has been assigned, you'll see output similar to the following:

```console
NAME            CLUSTER        NAMESPACE   DEPLOYMENT   SERVICE-EXTERNAL-IP
demo            demo-cluster   demo        wordpress    104.43.240.15
```

Once Wordpress is running and has a public IP address through its service, we can get the URL with the following command:

```console
echo "http://$(kubectl get workload test-workload -o jsonpath='{.status.service.loadBalancer.ingress[0].ip}')"
```

Paste that URL into your browser and you should see Wordpress running and ready for you to walk through the setup experience. You may need to wait a few minutes for this to become active in the aws elb.

## Connect to your EKSCluster (optional)

Requires:
 * awscli
 * aws-iam-authenticator

Please see [Install instructions](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html) section: `Install and Configure kubectl for Amazon EKS`

When the EKSCluster is up and running, you can update your kubeconfig with:
```console
aws eks update-kubeconfig --name <replace-me-eks-cluster-name>
```

Node pool is created after the master is up, so expect a few more minutes to wait, but eventually you can see that nodes joined with:
```console
kubectl config use-context <context-from-last-command>
kubectl get nodes
```


## Clean-up

First delete the workload, which will delete Wordpress and the MySQL database:

```console
kubectl -n demo delete -f cluster/examples/workloads/wordpress-${provider}/workload.yaml
```

Then delete the EKS cluster:

```console
kubectl delete -f cluster/examples/workloads/wordpress-${provider}/cluster.yaml
```

Finally, delete the provider credentials:

```console
kubectl delete -f cluster/examples/workloads/wordpress-${provider}/provider.yaml
```

> Note: There may still be an ELB that was not properly cleaned up, and you will need
to go to EC2 > ELBs and delete it manually.
