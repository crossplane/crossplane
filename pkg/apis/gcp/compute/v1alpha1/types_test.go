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

package v1alpha1

import (
	"context"
	"log"
	"testing"

	"github.com/go-test/deep"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
	name      = "test-cluster"
)

var (
	cfg *rest.Config
	c   client.Client
	ctx = context.TODO()
)

func TestMain(m *testing.M) {
	err := SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	t := test.NewEnv(namespace, test.CRDs())
	cfg = t.Start()

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	t.StopAndExit(m.Run())
}

func TestGKECluster(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &GKECluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: GKEClusterSpec{
			ClusterVersion: "1.1.1",
			NumNodes:       int64(1),
			Zone:           "us-central1-a",
			MachineType:    "n1-standard-1",
		},
	}
	g := NewGomegaWithT(t)

	// Test Create
	fetched := &GKECluster{}
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
}

func TestParseClusterSpec(t *testing.T) {
	tests := []struct {
		name string
		args map[string]string
		want *GKEClusterSpec
	}{
		{
			name: "nil properties",
			args: nil,
			want: &GKEClusterSpec{ReclaimPolicy: DefaultReclaimPolicy},
		},
		{
			name: "empty properties",
			args: map[string]string{},
			want: &GKEClusterSpec{ReclaimPolicy: DefaultReclaimPolicy},
		},
		{
			name: "valid values",
			args: map[string]string{
				"enableIPAlias": "true",
				"machineType":   "test-machine",
				"numNodes":      "3",
				"scopes":        "foo,bar",
				"zone":          "test-zone",
			},
			want: &GKEClusterSpec{
				ReclaimPolicy: DefaultReclaimPolicy,
				EnableIPAlias: true,
				MachineType:   "test-machine",
				NumNodes:      3,
				Scopes:        []string{"foo", "bar"},
				Zone:          "test-zone",
			},
		},
		{
			name: "invalid values",
			args: map[string]string{
				"enableIPAlias": "really",
				"machineType":   "test-machine",
				"numNodes":      "3.3",
				"scopes":        "foo,bar",
				"zone":          "test-zone",
			},
			want: &GKEClusterSpec{
				ReclaimPolicy: DefaultReclaimPolicy,
				EnableIPAlias: false,
				MachineType:   "test-machine",
				NumNodes:      1,
				Scopes:        []string{"foo", "bar"},
				Zone:          "test-zone",
			},
		},
		{
			name: "defaults",
			args: map[string]string{
				"machineType": "test-machine",
				"zone":        "test-zone",
			},
			want: &GKEClusterSpec{
				ReclaimPolicy: DefaultReclaimPolicy,
				EnableIPAlias: false,
				MachineType:   "test-machine",
				NumNodes:      1,
				Scopes:        []string{},
				Zone:          "test-zone",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseClusterSpec(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("ParseClusterSpec() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_parseNodesNumber(t *testing.T) {
	tests := []struct {
		name string
		args string
		want int64
	}{
		{name: "empty", args: "", want: DefaultNumberOfNodes},
		{name: "invalid", args: "foo", want: DefaultNumberOfNodes},
		{name: "0", args: "0", want: int64(0)},
		{name: "44", args: "44", want: int64(44)},
		{name: "-44", args: "-44", want: DefaultNumberOfNodes},
		{name: "1.2", args: "1.2", want: DefaultNumberOfNodes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseNodesNumber(tt.args); got != tt.want {
				t.Errorf("parseNodesNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}
