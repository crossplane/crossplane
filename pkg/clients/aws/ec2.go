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
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/pkg/errors"
	k8sclients "github.com/upbound/conductor/pkg/clients/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func EC2Client() (*ec2.EC2, error) {
	clientset, err := k8sclients.GetClientset()
	if err != nil {
		return nil, err
	}

	return EC2ClientFromClientset(clientset)
}

func EC2ClientFromClientset(clientset kubernetes.Interface) (*ec2.EC2, error) {
	log.Printf("getting ec2 client from clientset...")

	nodes, err := clientset.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "unable to get nodes")
	}

	name := ""
	region := ""
	if len(nodes.Items) > 0 {
		// take the first one, we assume that all nodes are created in the same VPC
		name = nodes.Items[0].Spec.ExternalID
		region = nodes.Items[0].Labels["failure-domain.beta.kubernetes.io/region"]
	} else {
		return nil, fmt.Errorf("unable to find any nodes in the cluster")
	}
	log.Printf("Found node with ID: %v in region %v", name, region)

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}

	// Set the AWS Region that the service clients should use
	cfg.Region = region
	cfg.HTTPClient.Timeout = 5 * time.Second
	return ec2.New(cfg), nil
}
