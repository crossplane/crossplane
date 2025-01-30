/*
Copyright 2021 The Crossplane Authors.

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

package composite

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// Annotation keys.
const (
	AnnotationKeyCompositionResourceName = "crossplane.io/composition-resource-name"
)

// SetCompositionResourceName sets the name of the composition template used to
// reconcile a composed resource as an annotation.
func SetCompositionResourceName(o metav1.Object, n ResourceName) {
	meta.AddAnnotations(o, map[string]string{AnnotationKeyCompositionResourceName: string(n)})
}

// GetCompositionResourceName gets the name of the composition template used to
// reconcile a composed resource from its annotations.
func GetCompositionResourceName(o metav1.Object) ResourceName {
	return ResourceName(o.GetAnnotations()[AnnotationKeyCompositionResourceName])
}
