/*
Copyright 2022 The Crossplane Authors.

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

package funcs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/yaml"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// AllOf runs the supplied functions in order.
func AllOf(fns ...features.Func) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		for _, fn := range fns {
			ctx = fn(ctx, t, c)
		}
		return ctx
	}
}

// DeploymentBecomesAvailableWithin fails a test if the supplied Deployment is
// not Available within the supplied duration.
func DeploymentBecomesAvailableWithin(d time.Duration, namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dp := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		t.Logf("Waiting %s for deployment %s/%s to become Available...", d, dp.GetNamespace(), dp.GetName())
		if err := wait.For(conditions.New(c.Client().Resources()).DeploymentConditionMatch(dp, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(d)); err != nil {
			t.Error(err)
			return ctx
		}
		t.Logf("Deployment %s/%s is Available", dp.GetNamespace(), dp.GetName())
		return ctx
	}
}

// CrossplaneCRDsBecomeEstablishedWithin fails a test if the core Crossplane
// CRDs are not Established within the environment in the supplied duration.
func CrossplaneCRDsBecomeEstablishedWithin(d time.Duration) features.Func {
	return ResourcesHaveConditionWithin(d, "cluster/crds", "*.yaml", xpv1.Condition{
		Type:    "Established",
		Status:  corev1.ConditionTrue,
		Reason:  "InitialNamesAccepted",
		Message: "the initial names have been accepted",
	})
}

// ResourcesCreatedWithin fails a test if the supplied resources are not found
// to exist within the supplied duration.
func ResourcesCreatedWithin(d time.Duration, dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Error(err)
			return ctx
		}

		list := &unstructured.UnstructuredList{}
		for _, o := range rs {
			u := asUnstructured(o)

			list.Items = append(list.Items, *u)
			t.Logf("Waiting %s for %s to exist...", d, identifier(u))
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesFound(list), wait.WithTimeout(d)); err != nil {
			t.Errorf("resources did not exist: %v", err)
			return ctx
		}

		t.Logf("%d resources found to exist", len(rs))
		return ctx
	}
}

// ResourcesDeletedWithin fails a test if the supplied resources are not found
// (i.e. are completely deleted) within the supplied duration.
func ResourcesDeletedWithin(d time.Duration, dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Error(err)
			return ctx
		}

		list := &unstructured.UnstructuredList{}
		for _, o := range rs {
			u := asUnstructured(o)
			t.Logf("Waiting %s for %s to be deleted...", d, identifier(u))
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesDeleted(list), wait.WithTimeout(d)); err != nil {
			t.Errorf("resources not deleted: %v", err)
			return ctx
		}

		t.Logf("%d resources deleted", len(rs))
		return ctx
	}
}

// ResourcesHaveConditionWithin fails a test if the supplied resources do not
// have (i.e. become) the supplied conditions within the supplied duration.
func ResourcesHaveConditionWithin(d time.Duration, dir, pattern string, cds ...xpv1.Condition) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Error(err)
			return ctx
		}

		reasons := make([]string, len(cds))
		for i := range cds {
			reasons[i] = string(cds[i].Reason)
		}
		desired := strings.Join(reasons, ", ")

		list := &unstructured.UnstructuredList{}
		for _, o := range rs {
			u := asUnstructured(o)
			list.Items = append(list.Items, *u)

			t.Logf("Waiting %s for %s to become %s...", d, identifier(u), desired)
		}

		match := func(o k8s.Object) bool {
			u := asUnstructured(o)
			s := xpv1.ConditionedStatus{}
			_ = fieldpath.Pave(u.Object).GetValueInto("status", &s)

			for _, want := range cds {
				if !s.GetCondition(want.Type).Equal(want) {
					return false
				}
			}

			return true
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, match), wait.WithTimeout(d)); err != nil {
			y, _ := yaml.Marshal(list.Items)
			t.Errorf("resources did not have desired conditions: %s: %v:\n\n%s\n\n", desired, err, y)
			return ctx
		}

		t.Logf("%d resources have desired conditions: %s", len(rs), desired)
		return ctx
	}
}

// ResourcesHaveFieldValueWithin fails a test if the supplied resources do not
// have the supplied value at the supplied field path within the supplied
// duration. The supplied 'want' value must cmp.Equal the actual value.
func ResourcesHaveFieldValueWithin(d time.Duration, dir, pattern, path string, want any) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Error(err)
			return ctx
		}

		list := &unstructured.UnstructuredList{}
		for _, o := range rs {
			u := asUnstructured(o)
			list.Items = append(list.Items, *u)

			t.Logf("Waiting %s for %s to have value %q at field path %s...", d, identifier(u), want, path)
		}

		match := func(o k8s.Object) bool {
			u := asUnstructured(o)
			got, err := fieldpath.Pave(u.Object).GetValue(path)
			if err != nil {
				return false
			}

			return cmp.Equal(want, got)
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, match), wait.WithTimeout(d)); err != nil {
			y, _ := yaml.Marshal(list.Items)
			t.Errorf("resources did not have desired value %q at field path %s: %v:\n\n%s\n\n", want, path, err, y)
			return ctx
		}

		t.Logf("%d resources have desired value %q at field path %s", len(rs), want, path)
		return ctx
	}
}

// CreateResources applies all manifests under the supplied directory that match
// the supplied glob pattern (e.g. *.yaml). Resources are created; they must not
// exist.
func CreateResources(dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, decoder.CreateHandler(c.Client().Resources())); err != nil {
			t.Fatal(err)
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("Created resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))
		return ctx
	}
}

// UpdateResources applies all manifests under the supplied directory that match
// the supplied glob pattern (e.g. *.yaml). Resources are updated; they must
// exist.
func UpdateResources(dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, decoder.UpdateHandler(c.Client().Resources())); err != nil {
			t.Fatal(err)
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("Updated resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))
		return ctx
	}
}

// DeleteResources deletes (from the environment) all resources defined by the
// manifests under the supplied directory that match the supplied glob pattern
// (e.g. *.yaml).
func DeleteResources(dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, decoder.DeleteHandler(c.Client().Resources())); err != nil {
			t.Fatal(err)
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("Deleted resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))
		return ctx
	}
}

// asUnstructured turns an arbitrary runtime.Object into an *Unstructured. If
// it's already a concrete *Unstructured it just returns it, otherwise it
// round-trips it through JSON encoding. This is necessary because types that
// are registered with our scheme will be returned as Objects backed by the
// concrete type, whereas types that are not will be returned as *Unstructured.
func asUnstructured(o runtime.Object) *unstructured.Unstructured {
	if u, ok := o.(*unstructured.Unstructured); ok {
		return u
	}

	u := &unstructured.Unstructured{}
	j, _ := json.Marshal(o)
	_ = json.Unmarshal(j, u)
	return u
}

// identifier returns the supplied resource's kind, name, and (if any)
// namespace.
func identifier(u *unstructured.Unstructured) string {
	if u.GetNamespace() == "" {
		return fmt.Sprintf("%s %s", u.GetKind(), u.GetName())
	}

	return fmt.Sprintf("%s %s/%s", u.GetKind(), u.GetNamespace(), u.GetName())
}
