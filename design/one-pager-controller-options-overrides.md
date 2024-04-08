# Controller Options Overrides

- Owners:
  - @max-melentyev
- Reviewers: Crossplane Maintainers
- Status: draft

## Background

There are cases when single configuration doesn't work for all controllers of a provider.
Different Crossplane controllers may have very different load profile and number of managed resources.
Consider examples in crossplane-contrib/provider-aws:

- Recomciling route53 records is fast, but having many resources of this type may trigger AWS API
throttling when all of them are reconciled with the same interval as other resources.
- Reconciling EC2 instance is relatively slow, and having many managed instances can
build up a long queue.

Having a feature that makes it possible to override default poll interval for a specific controller
would allow to avoid these issues without affecting other responsiveness of controllers in the provider.

### Migrating Controllers to Event Driven Reconciliation

Reconciling only updated resources would also help to resolve issues stated above.
But this will require to change every controller to support it, and may also require
additional infrastructure that will make it harder to use provider.

Enabling this feature for selected controllers will require ability to increase poll interval
for them, keeping it the same for other controllers.

## Design

### Providing Configuration

Controller specific options can be provided with `--controller-option {controller-id}.{option}={value}`,
for example `--controller-option ec2.instance.pollInterval=30m`.

### Configuring Controllers

`crossplane-runtime` provides `OptionsSet` struct that contains default configuration and 
all overrides. `OptionsSet.For(controllerId)` returns `controller.Options` with overrides for a
particular controller.
