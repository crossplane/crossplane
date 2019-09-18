
# Adding Amazon Web Services (AWS) to Crossplane

In this guide, we will walk through the steps necessary to configure your AWS account to be ready for integration with Crossplane. This will be done by adding a [`aw provider`] resource type, which enables Crossplane to communicate with an AWS account. 

## Requirements

Prior to adding AWS to Crossplane, following steps need to be taken

- Crossplane is installed in a k8s cluster
- AWS Stack is installed in the same cluster
- `kubectl` is configured to communicate with the same cluster

## Step 1: Configure `aws` CLI

Crossplane uses [AWS security credentials], and stores them as a [secret] which is managed by an  [`aw provider`]  instance. In addition, the AWS default region is also used for targeting a specific region.
Crossplane requires to have [`aws` command line tool] [installed] and [configured]. Once installed, the credentials and configuration will reside in `~/.aws/credentials` and `~/.aws/config` respectively.

## Step 2: Setup `aws` Provider

Run [setup.sh] script to read `aws` credentials and region, and create an [`aw provider`] instance in Crossplane:

```bash
./cluster/examples/setup-aws-provider/setup.sh [--profile aws_profile]
```

The `--profile` switch is optional and specifies the [aws named profile] that was set in Step 1. If not provided, the `default` profile will be selected.

Once the script is successfully executed, Crossplane will use the specified aws account and region in the given named profile to create subsequent AWS managed resources.

You can confirm the existense of the  [`aw provider`] by running:

```bash
kubectl -n crossplane-system get provider/aws-provider
```

[`aw provider`]: https://github.com/crossplaneio/stack-aws/blob/master/aws/apis/v1alpha2/types.go#L43
 [`aws` command line tool]: https://aws.amazon.com/cli/
[AWS SDK for GO]: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html
[installed]: [https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)
[configured]: [https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html)
[AWS security credentials]: https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html
[secret]:https://kubernetes.io/docs/concepts/configuration/secret/ 
[setup.sh]: github.com/crossplaneio/crossplane/cluster/examples/setup-aws-provider/setup.sh
[aws named profile]: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html
