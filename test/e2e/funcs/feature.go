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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
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

// ReadyToTestWithin fails a test if Crossplane is not ready to test within the
// supplied duration. It's typically called in a feature's Setup function. Its
// purpose isn't to test that Crossplane installed successfully (we have a
// specific test for that). Instead its purpose is to make sure tests don't
// start before Crossplane has finished installing.
func ReadyToTestWithin(d time.Duration, namespace string) features.Func {
	// Tests might fail if they start running before...
	//
	// * The core Crossplane CRDs are established.
	// * The Composition validation webhook isn't yet serving.
	//
	// The core Crossplane deployment being available is a pretty good signal
	// that both those things are true. It means the same pods that power the
	// webhook service are up-and-running, and that their init containers (which
	// install the CRDs) ran successfully.
	//
	// TODO(negz): We could explicitly test the above if we need to, but this is
	// a little faster and spams the test logs less.
	return DeploymentBecomesAvailableWithin(d, namespace, "crossplane")
}

// DeploymentBecomesAvailableWithin fails a test if the supplied Deployment is
// not Available within the supplied duration.
func DeploymentBecomesAvailableWithin(d time.Duration, namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dp := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		t.Logf("Waiting %s for deployment %s/%s to become Available...", d, dp.GetNamespace(), dp.GetName())
		if err := wait.For(conditions.New(c.Client().Resources()).DeploymentConditionMatch(dp, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(d)); err != nil {
			t.Fatal(err)
			return ctx
		}
		t.Logf("Deployment %s/%s is Available", dp.GetNamespace(), dp.GetName())
		return ctx
	}
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

// ResourceCreatedWithin fails a test if the supplied resource is not found to
// exist within the supplied duration.
func ResourceCreatedWithin(d time.Duration, o k8s.Object) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Logf("Waiting %s for %s to be created...", d, identifier(o))

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(o, func(object k8s.Object) bool { return true }), wait.WithTimeout(d)); err != nil {
			t.Errorf("resource %s did not exist: %v", identifier(o), err)
			return ctx
		}

		t.Logf("resource %s found to exist", identifier(o))
		return ctx
	}
}

// ResourcesDeletedWithin fails a test if the supplied resources are not deleted
// within the supplied duration.
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
			list.Items = append(list.Items, *u)
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

// ResourceDeletedWithin fails a test if the supplied resource is not deleted
// within the supplied duration.
func ResourceDeletedWithin(d time.Duration, o k8s.Object) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Logf("Waiting %s for %s to be deleted...", d, identifier(o))

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceDeleted(o), wait.WithTimeout(d)); err != nil {
			t.Errorf("resource %s not deleted: %v", identifier(o), err)
			return ctx
		}

		t.Logf("resource %s deleted", identifier(o))
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

// CRDInitialNamesAccepted is the status condition CRDs emit when they're
// established. Most of our Crossplane status conditions are defined elsewhere
// (e.g. in the xpv1 package), but this isn't so we define it here for
// convenience.
func CRDInitialNamesAccepted() xpv1.Condition {
	return xpv1.Condition{
		Type:    "Established",
		Status:  corev1.ConditionTrue,
		Reason:  "InitialNamesAccepted",
		Message: "the initial names have been accepted",
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

// ResourceHasFieldValueWithin fails a test if the supplied resource does not
// have the supplied value at the supplied field path within the supplied
// duration. The supplied 'want' value must cmp.Equal the actual value.
func ResourceHasFieldValueWithin(d time.Duration, o k8s.Object, path string, want any) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Logf("Waiting %s for %s to have value %q at field path %s...", d, identifier(o), want, path)

		match := func(o k8s.Object) bool {
			u := asUnstructured(o)
			got, err := fieldpath.Pave(u.Object).GetValue(path)
			if err != nil {
				return false
			}

			return cmp.Equal(want, got)
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(o, match), wait.WithTimeout(d)); err != nil {
			y, _ := yaml.Marshal(o)
			t.Errorf("resource did not have desired value %q at field path %s: %v:\n\n%s\n\n", want, path, err, y)
			return ctx
		}

		t.Logf("%s has desired value %q at field path %s", identifier(o), want, path)
		return ctx
	}
}

// ApplyResources applies all manifests under the supplied directory that match
// the supplied glob pattern (e.g. *.yaml). It uses server-side apply - fields
// are managed by the supplied field manager. Use force to apply the manifest
// even if it conflicts with another field manager.
func ApplyResources(manager, dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, ApplyHandler(c.Client().Resources(), manager)); err != nil {
			t.Fatal(err)
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("Applied resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))
		return ctx
	}
}

// ApplyHandler is a decoder.Handler that uses server-side apply to apply the
// supplied object.
func ApplyHandler(r *resources.Resources, manager string) decoder.HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		// TODO(negz): Use r.Patch when the below issue is solved?
		// https://github.com/kubernetes-sigs/e2e-framework/issues/254

		// TODO(negz): Make forcing optional? It seems to be necessary
		// sometimes, e.g. due to conflicts with a provider managing the same
		// fields. I'm guessing controller-runtime is setting providers as a
		// field manager at create time even though it doesn't use SSA?
		return r.GetControllerRuntimeClient().Patch(ctx, obj, client.Apply, client.FieldOwner(manager), client.ForceOwnership)
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
func identifier(o k8s.Object) string {
	k := o.GetObjectKind().GroupVersionKind().Kind
	if o.GetNamespace() == "" {
		return fmt.Sprintf("%s %s", k, o.GetName())
	}
	return fmt.Sprintf("%s %s/%s", k, o.GetNamespace(), o.GetName())
}
