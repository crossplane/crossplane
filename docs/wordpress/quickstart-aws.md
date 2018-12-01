# Wordpress on AWS

## Pre-requisites

### AWS Credentials

AWS Credentials file

Follow the steps in the [AWS SDK for GO](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html) to get your access key ID and secret access key
1. Open the IAM console.
1. On the navigation menu, choose Users.
1. Choose your IAM user name (not the check box).
1. Open the Security credentials tab, and then choose Create access key.
1. To see the new access key, choose Show. Your credentials resemble the following:
    - Access key ID: AKIAIOSFODNN7EXAMPLE
    - Secret access key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
1. To download the key pair, choose Download .csv file. Store the keys

Convert *.csv to `.aws/credentials` format
```
[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

**Note** If you have installed and configured `aws cli` you can find your AWS credentials file in  `~/.aws/credentials`

## Deploy Wordpress Resources

Next, create a `demo` namespace:

```console
kubectl create namespace demo
```

Deploy the AWS provider secret to your cluster:

```console
sed "s/BASE64ENCODED_AWS_PROVIDER_CREDS/`cat ~/.aws/credentials|base64|tr -d '\n'`/g" cluster/examples/wordpress/aws/class/provider.yaml | kubectl create -f -
```

Now deploy all the Wordpress resources, including the RDS database, with the following single command:

```console
kubectl -n demo create -f cluster/examples/wordpress/aws/class/wordpress.yaml
```

Now you can proceed back to the main quickstart to [wait for the resources to become ready](quickstart.md#waiting-for-completion).
