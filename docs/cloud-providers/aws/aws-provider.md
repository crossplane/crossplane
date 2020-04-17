# Adding Amazon Web Services (AWS) to Crossplane

In this guide, we will walk through the steps necessary to configure your AWS
account to be ready for integration with Crossplane. This will be done by adding
an AWS `Provider` resource type, which enables Crossplane to communicate with an
AWS account. 

## Requirements

Prior to adding AWS to Crossplane, following steps need to be taken

- Crossplane is installed in a k8s cluster
- AWS Stack is installed in the same cluster
- `kubectl` is configured to communicate with the same cluster

## Step 1: Configure `aws` CLI

Crossplane uses [AWS security credentials], and stores them as a [secret] which
is managed by an AWS `Provider` instance. In addition, the AWS default region is
also used for targeting a specific region. Crossplane requires to have [`aws`
command line tool] [installed] and [configured]. Once installed, the credentials
and configuration will reside in `~/.aws/credentials` and `~/.aws/config`
respectively.

## Step 2: Setup `aws` Provider

Run [setup.sh] script to read `aws` credentials and region, and create an `aws
provider` instance in Crossplane:

```bash
./cluster/examples/aws-setup-provider/setup.sh [--profile aws_profile]
```

The `--profile` switch is optional and specifies the [aws named profile] that
was set in Step 1. If not provided, the `default` profile will be selected.

Once the script is successfully executed, Crossplane will use the specified aws
account and region in the given named profile to create subsequent AWS managed
resources.

You can confirm the existense of the  AWS `Provider` by running:

```bash
kubectl -n crossplane-system get provider/aws-provider
```

## Optional: Setup AWS Provider Manually

An AWS [user][aws user] with `Administrative` privileges is needed to enable
Crossplane to create the required resources. Once the user is provisioned, an
[Access Key][] needs to be created so the user can have API access.

Using the set of [access key credentials][AWS security credentials] for the user
with the right access, we need to [install][install-aws] [`aws cli`][aws command
line tool], and then [configure][aws-cli-configure] it.

When the AWS cli is configured, the credentials and configuration will be in
`~/.aws/credentials` and `~/.aws/config` respectively. These will be consumed in
the next step.

When configuring the AWS cli, the user credentials could be configured under a
specific [AWS named profile][], or under `default`. Without loss of generality,
in this guide let's assume that the credentials are configured under the
`aws_profile` profile (which could also be `default`). We'll use this profile to
setup cloud provider in the next section.

Crossplane uses the AWS user credentials that were configured in the previous
step to create resources in AWS. These credentials will be stored as a
[secret][kubernetes secret] in Kubernetes, and will be used by an AWS `Provider`
instance. The default AWS region is also pulled from the cli configuration, and
added to the AWS provider.

To store the credentials as a secret, run:

```bash
# retrieve profile's credentials, save it under 'default' profile, and base64 encode it
BASE64ENCODED_AWS_ACCOUNT_CREDS=$(echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $aws_profile)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $aws_profile)" | base64  | tr -d "\n")
# retrieve the profile's region from config
AWS_REGION=$(aws configure get region --profile ${aws_profile})
```

At this point, the region and the encoded credentials are stored in respective
variables. Next, we'll need to create an instance of AWS provider:

```bash
cat > provider.yaml <<EOF
---
apiVersion: v1
kind: Secret
metadata:
  name: aws-account-creds
  namespace: crossplane-system
type: Opaque
data:
  credentials: ${BASE64ENCODED_AWS_ACCOUNT_CREDS}
---
apiVersion: aws.crossplane.io/v1alpha3
kind: Provider
metadata:
  name: aws-provider
spec:
  region: ${AWS_REGION}
  credentialsSecretRef:
    namespace: crossplane-system
    name: aws-account-creds
    key: credentials
EOF

# apply it to the cluster:
kubectl apply -f "provider.yaml"

# delete the credentials variable
unset BASE64ENCODED_AWS_ACCOUNT_CREDS
```

The output will look like the following:

```bash
secret/aws-user-creds created
provider.aws.crossplane.io/aws-provider created
```

The `aws-provider` resource will be used in other resources that we will create,
to provide access information to the configured AWS account.

<!-- Named Links -->

[`aws` command line tool]: https://aws.amazon.com/cli/
[AWS SDK for GO]: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html
[installed]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html
[configured]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html
[AWS security credentials]: https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html
[secret]:https://kubernetes.io/docs/concepts/configuration/secret/ 
[setup.sh]: https://github.com/crossplane/crossplane/blob/master/cluster/examples/aws-setup-provider/setup.sh
[aws named profile]: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html
[aws user]: https://docs.aws.amazon.com/mediapackage/latest/ug/setting-up-create-iam-user.html
[Access Key]: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_access-keys.html
[AWS security credentials]: https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html
[aws command line tool]: https://aws.amazon.com/cli/
[install-aws]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html
[aws-cli-configure]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html
[kubernetes secret]: https://kubernetes.io/docs/concepts/configuration/secret/
[AWS named profile]: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html
