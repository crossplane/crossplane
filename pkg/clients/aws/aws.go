package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-ini/ini"
)

// CredentialsIdSecret retrieves AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY from the data which contains
// aws credentials under given profile
// Example:
// [default]
// aws_access_key_id = <YOUR_ACCESS_KEY_ID>
// aws_secret_access_key = <YOUR_SECRET_ACCESS_KEY>
func CredentialsIdSecret(data []byte, profile string) (string, string, error) {
	config, err := ini.InsensitiveLoad(data)
	if err != nil {
		return "", "", err
	}

	iniProfile, err := config.GetSection(profile)
	if err != nil {
		return "", "", err
	}

	id, err := iniProfile.GetKey(external.AWSAccessKeyIDEnvVar)
	if err != nil {
		return "", "", err
	}

	secret, err := iniProfile.GetKey(external.AWSSecreteAccessKeyEnvVar)

	return id.Value(), secret.Value(), err
}

// Config - AWS configuration which can be used to issue requests against AWS API
func Config(data []byte, profile, region string) (*aws.Config, error) {
	id, secret, err := CredentialsIdSecret(data, profile)
	if err != nil {
		return aws.NewConfig(), err
	}

	creds := aws.Credentials{
		AccessKeyID:     id,
		SecretAccessKey: secret,
	}

	shared := external.SharedConfig{
		Credentials: creds,
		Region:      region,
	}

	config, err := external.LoadDefaultAWSConfig(shared)
	return &config, err
}

// Validate - validates AWS configuration by issuing list s3 buckets request
// TODO: find a better way to validate credentials
func ValidateConfig(config *aws.Config) error {
	svc := s3.New(*config)
	_, err := svc.ListBucketsRequest(nil).Send()
	return err
}
