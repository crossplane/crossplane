# Troubleshooting

## General Kubernetes debugging

General help on debugging applications running in Kubernetes can be found in the [Troubleshoot Applications task doc](https://kubernetes.io/docs/tasks/debug-application-cluster/debug-application/).

## Logs

The first place to look for more details about any issue with the controller manager would be its logs:

```console
kubectl -n conductor-system logs -f $(kubectl -n conductor-system get pod -l control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}')
```

## Image Pull Failures

The controller manager may be in the `ImagePullBackOff` status if you pushed to a private repository.

```console
> kubectl -n conductor-system get pod
NAME                                            READY     STATUS    RESTARTS   AGE
conductor-controller-manager-695d9bd759-9czpv   1/1       Running   0          3m
```

The pod has an `imagePullSecrets` named `regcred` that it will use for pulling images from a private repository.
You can create this secret with your personal credentials with a command **similar** to the following:

```console
kubectl -n conductor-system create secret docker-registry regcred --docker-server=https://index.docker.io/v1/ --docker-username=<userName> --docker-password='<password>' --docker-email=<emailAddr>
```

The controller manager pod should be able to use this secret the next time it attempts to pull the image and it should then be in the `Running` state.
Note that you may want to [scale the controller manager](../README.md#scaling-the-controller-manager) `Deployment` to 0 then back to 1 again to force it to retry the image pull more quickly.

## GKE RBAC

On GKE clusters, the default cluster role associated with your Google account does not have permissions to grant further RBAC permissions.
When running `make deploy`, you will see an error that contains a message similar to the following:

```console
clusterroles.rbac.authorization.k8s.io "conductor-manager-role" is forbidden: attempt to grant extra privileges
```

To work around this, you will you need to run a command **one time** that is **similar** to the following in order to bind your Google credentials `cluster-admin` role:

```console
kubectl create clusterrolebinding dev-cluster-admin-binding --clusterrole=cluster-admin --user=<googleEmail>
```
