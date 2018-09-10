# Project Conductor (codename)

Project Conductor brings cloud provider resources into Kubernetes with unifying cloud abstractions.

## Usage

You can start using the custom controllers and resources from Project Conductor with the following commands:

First build the container image:

```console
make docker-build
```

Then publish the container image to a repository that will be accessible by your testing environment:

```console
make docker-push
```

Finally, you can deploy the custom controllers and resources:

```console
make deploy
```

The controller manager should now be running in the `project-conductor-system` namespace, so you can go ahead and create an instance of a cloud resource.
For example, to create a Google Cloud SQL instance, run the following:

```console
kubectl create -f config/samples/gcp_v1alpha1_cloudsql.yaml
```

You can get status updates and watch progress in the controller manager logs:

```console
kubectl -n project-conductor-system logs -f project-conductor-controller-manager-0
```

It's also possible to get status and information from the CRD itself.
For example, to get information about a Google Cloud SQL CRD, run the following:

```console
kubectl get cloudsql -o yaml
```

### Image Pull Failures

The controller manager may be in the `ImagePullBackOff` status if you pushed to a private repository.

```console
> kubectl -n project-conductor-system get pod -o wide
NAME                                     READY     STATUS             RESTARTS   AGE
project-conductor-controller-manager-0   0/1       ImagePullBackOff   0          14s
```

The pod has an `imagePullSecrets` named `regcred` that it will use for pulling images from a private repository.
You can create this secret with your personal credentials with a command **similar** to the following:

```console
kubectl -n project-conductor-system create secret docker-registry regcred --docker-server=https://index.docker.io/v1/ --docker-username=<userName> --docker-password='<password>' --docker-email=<emailAddr>
```

The controller manager pod should be able to use this secret the next time it attempts to pull the image and it should then be in the `Running` state.
Note that you may want to [scale the controller manager](#scaling-the-controller-manager) `StatefulSet` to 0 then back to 1 again to force it to retry the image pull more quickly.

### Scaling the controller manager

You can scale the controller manager `StatefulSet` down to 0, essentially stopping the controller manager for the time being.
This can be useful if you are working on fixes and don't want it running until you have a new image to use.

```console
kubectl -n project-conductor-system scale --replicas=0 statefulset/project-conductor-controller-manager
```

Similarly, you can scale it up to 1 to start it running again:

```console
kubectl -n project-conductor-system scale --replicas=1 statefulset/project-conductor-controller-manager
```

## Developer iteration workflow

The typical developer iteration workflow is to scale the controller manager down, delete any CRD instances, make your changes and then deploy again.
The commands are summarized below, but there is obviously more automation here that can streamline this:

```bash
kubectl -n project-conductor-system  scale --replicas=0 statefulset/project-conductor-controller-manager
kubectl delete -f config/samples/
# update name of cloud sql CRD in config/samples/gcp_v1alpha1_cloudsql.yaml

make docker-build
make docker-push
make deploy

kubectl -n project-conductor-system  scale --replicas=1 statefulset/project-conductor-controller-manager
kubectl -n project-conductor-system get pod -o wide
kubectl -n project-conductor-system logs -f project-conductor-controller-manager-0

kubectl create -f config/samples/gcp_v1alpha1_cloudsql.yaml
```

## Cleanup/Teardown

To completely clean up all resources in your target cluster, run the following command:

```console
make clean-deploy
```

## Troubleshooting

### Logs

The first place to look for more details about any issue would be the controller manager logs:

```console
kubectl -n project-conductor-system logs -f project-conductor-controller-manager-0
```

### GKE RBAC

On GKE clusters, the default cluster role associated with your Google account does not have permissions to grant further RBAC permissions.
When running `make deploy`, you will see an error that contains a message similar to the following:

```console
clusterroles.rbac.authorization.k8s.io "project-conductor-manager-role" is forbidden: attempt to grant extra privileges
```

To work around this, you will you need to run a command **one time** that is **similar** to the following in order to bind your Google credentials `cluster-admin` role:

```console
kubectl create clusterrolebinding dev-cluster-admin-binding --clusterrole=cluster-admin --user=<googleEmail>
```
