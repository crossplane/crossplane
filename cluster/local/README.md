# Deploying Crossplane Locally

This directory contains scripts that automate common local development flows for
Crossplane, allowing you to deploy your local build of Crossplane to a `kind`
cluster. [kind-with-registry.sh] is being used to setup a single-node
kind Kubernetes cluster.

## Using Tilt

[Tilt] can be used to reduce the repetative tasks you might have to do to keep your
local deployment up to date. It watches required folders locally and takes action
accordingly (e.g. build crossplane binary and image, deploy it on local cluster, etc).

In order to use tilt, the following steps are needed:

1. Prepare local `.work/` folder (only once)

   ```sh
   make local.prepare
   ```

2. Spin up a kind cluster:

    ```sh
    USE_TILT=true make kind.up
    ```

3. Update helm chart dependency, this must be done [outside of Tilt]:

    ```sh
    make local.helmdep
    ```

    This is needed once or when `cluster/charts/crossplane/charts` is missing or any of the
    dependencies of Crossplane chart have been changed, otherwise you'd see this error:

    ```text
    Error: found in Chart.yaml, but missing in charts/ directory
    ```

4. Use tilt!

    ```sh
    make tilt.up
    ```

    If you've encountered `Error: notify.Add(...): no space left on device` check out
    [this comment] for a solution.

To see all available options for local developement look at [build/makelib/local.mk].

### Working with Providers

If you are working on a Crossplane provider and you want it to get deployed with Tilt
you can add the provider details to a `gitignore`-d file called `tilt-providers.json`
in the root of `crossplane/crossplane` repository.

```json
{
    "provider-aws": {
        "enabled": true,
        "context": "/path/to/local/clone/provider-aws"
    },
    "provider-gcp": {
        "enabled": true,
        "package_owner": "crossplane",
        "package_ref": "alpha"
    }
}
```

You also would be able to set some of the configuration options for the overall state of the deployment in `tilt-settings.json`.

```json
{
    "args": [],
    "debug": false,
    "namespace": "crossplane-system"
}
```

This will deploy two providers as part of running Tilt:

- `provider-gcp` from `crossplane` Dockerhub namespace of `alpha` channel
- `provider-aws` built locally from `/path/to/local/clone/provider-aws`
  - note that `context` can also be relative to crossplane local cloned folder

[kind-with-registry.sh]: ./kind-with-registry.sh
[Tilt]: https://tilt.dev
[outside of Tilt]: https://docs.tilt.dev/helm.html#sub-charts-and-requirementstxt
[this comment]: https://github.com/tilt-dev/tilt/issues/2079#issuecomment-632226113
[build/makelib/local.mk]: build/makelib/local.mk
