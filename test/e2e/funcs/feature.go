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

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
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

// DeploymentBecomesAvailableIn fails a test if the supplied Deployment is not
// Available within the supplied duration.
func DeploymentBecomesAvailableIn(namespace, name string, d time.Duration) features.Func {
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

// CrossplaneCRDsBecomeEstablishedIn fails a test if the core Crossplane CRDs
// are not Established within the environment in the supplied duration.
func CrossplaneCRDsBecomeEstablishedIn(d time.Duration) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		crds, err := decoder.DecodeAllFiles(ctx, os.DirFS(crdsDir), "*.yaml")
		if err != nil {
			t.Fatal(err)
			return ctx
		}

		list := &unstructured.UnstructuredList{}
		for _, o := range crds {
			crd := asUnstructured(o)
			list.Items = append(list.Items, *crd)
			t.Logf("Waiting %s for core Crossplane CRD %s to become Established...", d, crd.GetName())
		}

		match := func(o k8s.Object) bool {
			u := asUnstructured(o)
			s := xpv1.ConditionedStatus{}
			_ = fieldpath.Pave(u.Object).GetValueInto("status", &s)

			return s.GetCondition(xpv1.ConditionType("Established")).Equal(xpv1.Condition{
				Type:    "Established",
				Status:  corev1.ConditionTrue,
				Reason:  "InitialNamesAccepted",
				Message: "the initial names have been accepted",
			})
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, match), wait.WithTimeout(d)); err != nil {
			t.Fatal(err)
			return ctx
		}

		t.Logf("%d core Crossplane CRDs are Established", len(crds))

		return ctx
	}
}

// XRDsBecomeEstablishedIn fails a test if the supplied XRDs are not established
// within the supplied duration.
func XRDsBecomeEstablishedIn(dir, pattern string, d time.Duration) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		xrds, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Fatal(err)
			return ctx
		}

		list := &apiextensionsv1.CompositeResourceDefinitionList{}
		for _, o := range xrds {
			xrd := o.(*apiextensionsv1.CompositeResourceDefinition)
			list.Items = append(list.Items, *xrd)
			t.Logf("Waiting %s for XRD %s to become Established...", d, xrd.GetName())
		}

		match := func(o k8s.Object) bool {
			xrd := o.(*apiextensionsv1.CompositeResourceDefinition)
			return xrd.Status.GetCondition(apiextensionsv1.TypeEstablished).Equal(apiextensionsv1.WatchingComposite())
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, match), wait.WithTimeout(d)); err != nil {
			t.Fatal(err)
			return ctx
		}

		t.Logf("%d XRDs are Established", len(xrds))
		return ctx
	}
}

// ResourcesCreatedIn fails a test if the supplied resources are not found to
// exist within the supplied duration.
func ResourcesCreatedIn(dir, pattern string, d time.Duration) features.Func {
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

// ResourcesDeletedIn fails a test if the supplied resources are not found
// (i.e. are completely deleted) within the supplied duration.
func ResourcesDeletedIn(dir, pattern string, d time.Duration) features.Func {
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

// ResourcesBecomeIn fails a test if the supplied resources do not have (i.e.
// become) the supplied conditions within the supplied duration. It is intended
// for use with resources that aren't registered to the environment's scheme -
// e.g. claims, XRs, or MRs.
func ResourcesBecomeIn(dir, pattern string, d time.Duration, cds ...xpv1.Condition) features.Func {
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

// ResourcesHaveIn fails a test if the supplied resources do not have the
// supplied value at the supplied field path within the supplied duration. It is
// intended for use with resources that aren't registered to the environment's
// scheme - e.g. claims, XRs, or MRs.. The supplied 'want' value must cmp.Equal
// the actual value.
func ResourcesHaveIn(dir, pattern string, d time.Duration, path string, want any) features.Func {
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
