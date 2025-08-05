/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package names

import (
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

// rbacGroupKind is a map of the RBAC resources. Needed since name validation
// is different from other k8s resources.
//
//nolint:gochecknoglobals // Ack, this is global.
var rbacGroupKind = map[schema.GroupKind]bool{
	{Group: rbacv1.GroupName, Kind: "Role"}:               true,
	{Group: rbacv1.GroupName, Kind: "ClusterRole"}:        true,
	{Group: rbacv1.GroupName, Kind: "RoleBinding"}:        true,
	{Group: rbacv1.GroupName, Kind: "ClusterRoleBinding"}: true,
}

// ValidateName returns false and an error if the passed name is not a valid
// resource name; true otherwise. For almost all resources, the following
// characters are allowed:
//
//	Most resource types require a name that can be used as a DNS label name
//	as defined in RFC 1123. This means the name must:
//
//	* contain no more than 253 characters
//	* contain only lowercase alphanumeric characters, '-'
//	* start with an alphanumeric character
//	* end with an alphanumeric character
//
// For RBAC resources we also allow the colon character.
//
// Forked from https://github.com/seans3/cli-utils/blob/9d0ec2cd7107b62f3dc263887b40fe0b7c44d813/pkg/object/objmetadata.go
func ValidateName(name string, gk schema.GroupKind) (bool, error) {
	if _, exists := rbacGroupKind[gk]; exists {
		name = strings.ReplaceAll(name, ":", "")
	}
	errs := validation.IsDNS1123Subdomain(name)
	return len(errs) == 0, fmt.Errorf("%s", strings.Join(errs, ", "))
}
