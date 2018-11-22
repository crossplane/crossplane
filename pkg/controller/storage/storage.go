/*
Copyright 2018 The Crossplane Authors.

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

package storage

import (
	"github.com/crossplaneio/crossplane/pkg/controller/storage/bucket"
	"github.com/crossplaneio/crossplane/pkg/controller/storage/mysql"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func init() {
	AddToManagerFuncs = append(AddToManagerFuncs, bucket.Add)
	AddToManagerFuncs = append(AddToManagerFuncs, mysql.Add)
}

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m); err != nil {
			return err
		}
	}
	return nil
}
