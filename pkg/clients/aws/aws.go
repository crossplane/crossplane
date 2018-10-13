/*
Copyright 2018 The Conductor Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"fmt"
	"io/ioutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-ini/ini"
	"github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CredentialsIDSecret retrieves AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY from the data which contains
// aws credentials under given profile
// Example:
// [default]
// aws_access_key_id = <YOUR_ACCESS_KEY_ID>
// aws_secret_access_key = <YOUR_SECRET_ACCESS_KEY>
func CredentialsIDSecret(data []byte, profile string) (string, string, error) {
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
	if err != nil {
		return "", "", err
	}

	return id.Value(), secret.Value(), err
}

// LoadConfig - AWS configuration which can be used to issue requests against AWS API
func LoadConfig(data []byte, profile, region string) (*aws.Config, error) {
	id, secret, err := CredentialsIDSecret(data, profile)
	if err != nil {
		return nil, err
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

// ValidateConfig - validates AWS configuration by issuing list s3 buckets request
// TODO: find a better way to validate credentials
func ValidateConfig(config *aws.Config) error {
	svc := s3.New(*config)
	_, err := svc.ListBucketsRequest(nil).Send()
	return err
}

// Config - crate AWS Config based on credentials data using [default] profile
func Config(client kubernetes.Interface, p *v1alpha1.Provider) (*aws.Config, error) {
	secret, err := client.CoreV1().Secrets(p.Namespace).Get(p.Spec.Secret.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	data, found := secret.Data[p.Spec.Secret.Key]
	if !found {
		return nil, fmt.Errorf("invalid AWS Provider secret, data key [%s] is not found", p.Spec.Secret.Key)
	}

	return LoadConfig(data, ini.DEFAULT_SECTION, p.Spec.Region)
}

// ConfigFromFile - create AWS Config based on credential file using [default] profile
func ConfigFromFile(file, region string) (*aws.Config, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return LoadConfig(data, ini.DEFAULT_SECTION, region)
}
