/*
Copyright 2019 The Crossplane Authors.

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

package install

import (
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// Reader is an interface for reading pod logs
type Reader interface {
	GetReader(namespace, name string) (io.ReadCloser, error)
}

// K8sReader is a concrete implementation of the podLogReader interface
type K8sReader struct {
	Client kubernetes.Interface
}

// GetReader gets a log reader for the specified pod
func (r *K8sReader) GetReader(namespace, name string) (io.ReadCloser, error) {
	req := r.Client.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{})
	return req.Stream()
}
