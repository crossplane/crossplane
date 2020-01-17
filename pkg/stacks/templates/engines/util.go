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

package engines

import (
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/hash"
)

// The main reason this exists as its own method is to encapsulate the hashing logic
func generateConfigMap(name string, fileName string, fileContents string, log logr.Logger) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	cm.Name = name
	cm.Data = map[string]string{}

	cm.Data[fileName] = fileContents
	h, err := hash.ConfigMapHash(cm)
	if err != nil {
		log.Info("Error hashing config map!", "error", err)
		return cm, err
	}
	cm.Name = fmt.Sprintf("%s-%s", cm.Name, h)

	return cm, nil
}
