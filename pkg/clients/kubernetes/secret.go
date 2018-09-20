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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetSecret will return the data from the given secret
func GetSecret(clientset kubernetes.Interface, namespace string, name string, key string) (string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to fetch secret %s from namespace %s: %+v", name, namespace, err)
	}

	dataField, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key %s does not exist in secret %s from namespace %s", key, name, namespace)
	}

	return string(dataField), nil
}
