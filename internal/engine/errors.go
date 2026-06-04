/*
Copyright 2025 The Crossplane Authors.

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

package engine

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// A SubjectAccessReviewError is a structured error reporting what an identity is not allowed to do.
type SubjectAccessReviewError struct {
	// User is the identity is attempting to act on the Resource.
	// For service accounts, format them as "system:serviceaccount:{namespace}:{serviceaccount}".
	User string
	// Resource is the subject.
	Resource schema.GroupVersionResource
	// Namespace is the subject's namespace, or empty for cluster scoped subjects.
	Namespace string
	// DeniedVerbs is the list of verbs the user want to be able to do but can't.
	DeniedVerbs []string
	// Err is an optional wrapped error.
	Err error
}

// Error implements errors.Error.
func (e SubjectAccessReviewError) Error() string {
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "%s is not allowed to [%s] resource %s",
		e.User, strings.Join(e.DeniedVerbs, ", "), e.Resource.Resource)

	if e.Resource.Group != "" {
		fmt.Fprintf(&sb, ".%s", e.Resource.GroupVersion())
	} else {
		fmt.Fprintf(&sb, "/%s", e.Resource.GroupVersion())
	}

	if e.Namespace != "" {
		fmt.Fprintf(&sb, " in namespace %s", e.Namespace)
	}
	if e.Err != nil {
		fmt.Fprintf(&sb, ": %s", e.Err.Error())
	}

	return sb.String()
}

// Unwrap implements errors.Unwrap.
func (e SubjectAccessReviewError) Unwrap() error {
	return e.Err
}
