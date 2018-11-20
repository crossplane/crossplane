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

package util

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	maxNameLength          = 253
	randomLength           = 5
	maxGeneratedNameLength = maxNameLength - randomLength
)

// GenerateName  returns the name plus a random suffix of five alphanumerics
// when a name is requested. The string is guaranteed to not exceed the length of a standard Kubernetes
// name (253 characters)
//  GenerateName("foo-")
// would return value similar to: "foo-a1b2c".
// If base string length exceeds 248 (253 - 5) characters, it will be truncated to 248 characters before
// adding random suffix
//  GenerateName("foo...ververylongstringof253chars")
// would return value similar to: "foo...ververylongstringof253x8y9z"
func GenerateName(base string) string {
	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, rand.String(randomLength))
}

// ObjectReference from provided ObjectMeta, apiVersion and kind
func ObjectReference(o metav1.ObjectMeta, apiVersion, kind string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion:      apiVersion,
		Kind:            kind,
		Name:            o.Name,
		Namespace:       o.Namespace,
		ResourceVersion: o.ResourceVersion,
		UID:             o.UID,
	}
}

// ObjectToOwnerReference converts core ObjectReference to meta OwnerReference
func ObjectToOwnerReference(r *corev1.ObjectReference) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: r.APIVersion,
		Kind:       r.Kind,
		Name:       r.Name,
		UID:        r.UID,
	}
}

// ApplyDeployment creates or updates existing deployment
func ApplyDeployment(c kubernetes.Interface, d *appsv1.Deployment) (*appsv1.Deployment, error) {
	dd, err := c.AppsV1().Deployments(d.Namespace).Create(d)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return c.AppsV1().Deployments(d.Namespace).Update(d)
		}
		return nil, err
	}
	return dd, nil
}

// ApplyService creates or updates existing service
func ApplyService(c kubernetes.Interface, s *corev1.Service) (*corev1.Service, error) {
	ss, err := c.CoreV1().Services(s.Namespace).Create(s)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// retrieve the existing server to grab `ClusterIP` value
			ss, err := c.CoreV1().Services(s.Namespace).Get(s.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			s.Spec.ClusterIP = ss.Spec.ClusterIP
			return c.CoreV1().Services(s.Namespace).Update(s)
		}
		return nil, err
	}
	return ss, nil
}

// ApplySecret creates or updates if exist kubernetes secret
func ApplySecret(c kubernetes.Interface, s *corev1.Secret) (*corev1.Secret, error) {
	_, err := c.CoreV1().Secrets(s.Namespace).Get(s.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.CoreV1().Secrets(s.Namespace).Create(s)
		}
		return nil, err
	}
	return c.CoreV1().Secrets(s.Namespace).Update(s)
}

// SecretData returns secret data value for a given secret/key combination or error if secret or key is not found
func SecretData(client kubernetes.Interface, namespace string, ks corev1.SecretKeySelector) ([]byte, error) {
	// find secret
	secret, err := client.CoreV1().Secrets(namespace).Get(ks.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// retrieve data
	data, ok := secret.Data[ks.Key]
	if !ok {
		return nil, fmt.Errorf("secet data is not found for key [%s]", ks.Key)
	}

	return data, nil
}

// LatestDeploymentCondition
func LatestDeploymentCondition(conditions []appsv1.DeploymentCondition) appsv1.DeploymentCondition {
	var latest appsv1.DeploymentCondition
	for _, c := range conditions {
		if c.Status == corev1.ConditionTrue && c.LastUpdateTime.After(latest.LastUpdateTime.Time) {
			latest = c
		}
	}
	return latest
}

// IfEmptyString test input string and if empty, i.e = "", return a replacement string
func IfEmptyString(s, r string) string {
	if s == "" {
		return r
	}
	return s
}
