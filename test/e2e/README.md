1. Provision a cluster somewhere (e.g. using `kind`)
2. Install crossplane
3. Run end-to-end tests from this directory with 

```shell
go test
```

Use `go test -godog.help` for additional help.

If you want to run a particular test use something like:

```shell
go test -test.v -test.run ^TestFeatures$/^my_scenario$
```