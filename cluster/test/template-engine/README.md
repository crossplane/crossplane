# Stack template engine

## Developing

### Testing

These are some rudimentary integration tests for the template controller
engine.

#### A word about image ids

Note that using a `:latest` tag will [set the image pull
policy](https://kubernetes.io/docs/concepts/containers/images/#updating-images)
to `Always` if it is not specified, which is probably not desired during
development. The easiest way around it is to use a tag other than
`latest` for the relevant images. This will be improved in a future
change. To use a tag other than `latest` for the crossplane image, you
should be able to just tag your crossplane image with something other
than `latest`:

```
docker tag build-7afd0511/crossplane-amd64:latest build-7afd0511/crossplane-amd64:test
```

#### Running the test

To test, the Crossplane CRDs must be installed, and the stack manager
must be running with the template controller enabled (the `--templates`
flag must be specified to do this). Here's an example of a command to
run from the **root of this repo** to run the stack manager
out-of-cluster in a terminal window, with the template controller
enabled:

```
make go.build && env STACK_MANAGER_IMAGE=build-7afd0511/crossplane-amd64:test _output/bin/darwin_amd64/crossplane --debug stack manage --templates
```


Then, in another window, run the integration test from **this
directory** to build all the helpers and create test objects:

```
make integration-test
```

It should create a config map named `mycustomname-{{ engine }}`. So, for
example, `mycustomname-helm2` for the helm 2 integration test.

To clean up, run:

```
make clean-integration-test
```

The source files for the integration tests are in the `Makefile` and in
folders named after the resource engine they are intended to test.

To debug the integration test, inspect the logs for any jobs or pods
which were run by the controller. Also take a look at the controller's
logs.
