/*
Copyright 2020 The Crossplane Authors.

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

package parser

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// fromUnstructured converts an object from map[string]interface{}
// representation into a concrete type.
// TODO(hasheddan): revmove once
// https://github.com/kubernetes/kubernetes/pull/93250 is merged and we consume
// the appropriate k8s.io/apimachinery version (likely k8s v1.20).
func fromUnstructured(unstructured map[string]interface{}, obj interface{}) error {
	b, err := json.Marshal(unstructured)
	if err != nil {
		return errors.Wrap(err, "unable to convert unstructured item to JSON")
	}

	if err = json.Unmarshal(b, &obj); err != nil {
		return errors.Wrap(err, "unable to convert JSON to interface")
	}

	return nil
}
