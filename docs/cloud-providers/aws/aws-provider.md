# Adding Amazon Web Services (AWS) to Crossplane

In this guide, we will walk through the steps necessary to configure your AWS account to be ready for integration with Crossplane.

## AWS Credentials

### Option 1: aws Command Line Tool

If you have already installed and configured the [`aws` command line tool](https://aws.amazon.com/cli/), you can simply find your AWS credentials file in `~/.aws/credentials`.

### Option 2: AWS Console in Web Browser

If you do not have the `aws` tool installed, you can alternatively log into the [AWS console](https://aws.amazon.com/console/) and export the credentials.
The steps to follow below are from the [AWS SDK for GO](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html):

1. Open the IAM console.
1. On the navigation menu, choose Users.
1. Choose your IAM user name (not the check box).
1. Open the Security credentials tab, and then choose Create access key.
1. To see the new access key, choose Show. Your credentials resemble the following:
  - Access key ID: AKIAIOSFODNN7EXAMPLE
  - Secret access key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
1. To download the key pair, choose Download .csv file.

Then convert the `*.csv` file to the below format and save it to `~/.aws/credentials`:

```
[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

After the steps above, you should have your AWS credentials stored in `~/.aws/credentials`.
