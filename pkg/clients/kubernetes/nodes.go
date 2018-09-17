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

package kubernetes

import (
	"fmt"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// NodeRegionLabel is the label used to specify the region a node is located in
	NodeRegionLabel = "failure-domain.beta.kubernetes.io/region"
)

// NodeInfo represents information about a node in a Kubernetes cluster
type NodeInfo struct {
	Name   string
	Region string
}

// GetFirstNodeInfo will return information about the first node it finds in the cluster
func GetFirstNodeInfo(clientset kubernetes.Interface) (*NodeInfo, error) {
	nodes, err := clientset.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %+v", err)
	}

	var name, region string
	if len(nodes.Items) > 0 {
		// take the first one, we assume that all nodes are created in the same VPC
		// TODO: what if all the nodes are not in the same VPC?
		node := nodes.Items[0]
		region = node.Labels[NodeRegionLabel]
		if nodes.Items[0].Spec.ProviderID != "" {
			name = node.Spec.ProviderID
		} else {
			name = node.Name
		}
	} else {
		return nil, fmt.Errorf("unable to find any nodes in the cluster")
	}

	log.Printf("found node with name '%s' in region '%s'", name, region)
	return &NodeInfo{Name: name, Region: region}, nil
}
