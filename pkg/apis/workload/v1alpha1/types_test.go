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

package v1alpha1

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
	name      = "test-instance"
)

var (
	ctx = context.TODO()
	c   client.Client

	key    = types.NamespacedName{Name: name, Namespace: namespace}
	labels = map[string]string{"cool": "super"}
)

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, SchemeBuilder.SchemeBuilder, test.CRDs())
	c = t.StartClient()
	t.StopAndExit(m.Run())
}

func TestKubernetesApplication(t *testing.T) {
	g := NewGomegaWithT(t)

	created := &KubernetesApplication{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: KubernetesApplicationSpec{
			ResourceSelector: &metav1.LabelSelector{MatchLabels: labels},
			ClusterSelector:  &metav1.LabelSelector{},
			ResourceTemplates: []KubernetesApplicationResourceTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
					},
					Spec: KubernetesApplicationResourceSpec{
						Secrets:  []corev1.LocalObjectReference{{Name: name}},
						Template: resourceTemplate(),
					},
				},
			},
		},
	}

	// Test Create
	fetched := &KubernetesApplication{}
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the annotations
	updated := fetched.DeepCopy()
	updated.Annotations = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
}

func TestKubernetesApplicationResource(t *testing.T) {
	g := NewGomegaWithT(t)

	created := &KubernetesApplicationResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    labels,
		},
		Spec: KubernetesApplicationResourceSpec{
			Secrets:  []corev1.LocalObjectReference{{Name: name}},
			Template: resourceTemplate(),
		},
	}

	// Test Create
	fetched := &KubernetesApplicationResource{}
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the annotations
	updated := fetched.DeepCopy()
	updated.Annotations = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
}

func resourceTemplate() *unstructured.Unstructured {
	tmpl := &unstructured.Unstructured{}
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}

	s := runtime.NewScheme()
	appsv1.AddToScheme(s)
	s.Convert(deploy, tmpl, nil)

	return tmpl
}

func TestRemoteStatus(t *testing.T) {
	cases := []struct {
		name string
		want []byte
	}{
		{
			name: "ValidJSONObject",
			want: []byte(`{"coolness":"EXTREME!"}`),
		},
		{
			name: "ValidJSONArray",
			want: []byte(`["cool","cooler","coolest"]`),
		},
		{
			name: "ValidJSONString",
			want: []byte(`"hi"`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := &RemoteStatus{}
			if err := json.Unmarshal(tc.want, rs); err != nil {
				t.Fatalf("json.Unmarshal(...): %s", err)
			}

			if diff := cmp.Diff(string(rs.Raw), string(tc.want)); diff != "" {
				t.Errorf("json.Unmarshal(...): got != want: %s", diff)
			}

			got, err := json.Marshal(rs)
			if err != nil {
				t.Fatalf("json.Marshal(...): %s", err)
			}

			if diff := cmp.Diff(string(got), string(tc.want)); diff != "" {
				t.Errorf("json.Marshal(...): got != want: %s", diff)
			}
		})
	}
}
