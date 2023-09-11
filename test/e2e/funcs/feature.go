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
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
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

		count := atomic.Int32{}
		match := func(o k8s.Object) bool {
			count.Add(1)
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

		if count.Load() == 0 {
			t.Errorf("no resources matched pattern %s", filepath.Join(dir, pattern))
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
// are managed by the supplied field manager. It fails the test if any supplied
// resource cannot be applied successfully.
func ApplyResources(manager, dir, pattern string, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, ApplyHandler(c.Client().Resources(), manager), options...); err != nil {
			t.Fatal(err)
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		if len(files) == 0 {
			t.Errorf("No resources found in %s", filepath.Join(dir, pattern))
			return ctx
		}
		t.Logf("Applied resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))
		return ctx
	}
}

// SetAnnotationMutateOption returns a DecodeOption that sets the supplied
// annotation on the decoded object.
func SetAnnotationMutateOption(key, value string) decoder.DecodeOption {
	return decoder.MutateOption(func(o k8s.Object) error {
		a := o.GetAnnotations()
		if a == nil {
			a = map[string]string{}
		}
		a[key] = value
		o.SetAnnotations(a)
		return nil
	})
}

// ResourcesFailToApply applies all manifests under the supplied directory that
// match the supplied glob pattern (e.g. *.yaml). It uses server-side apply -
// fields are managed by the supplied field manager. It fails the test if any
// supplied resource _can_ be applied successfully - use it to test that the API
// server should reject a resource.
func ResourcesFailToApply(manager, dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, ApplyHandler(c.Client().Resources(), manager)); err == nil {
			// TODO(negz): Ideally we'd say which one.
			t.Error("Resource applied successfully, but should have failed")
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("All resources from %s (matched %d manifests) failed to apply", filepath.Join(dir, pattern), len(files))
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

// CopyImageToRegistry tries to copy the supplied image to the supplied registry within the timeout
func CopyImageToRegistry(clusterName, ns, sName, image string, timeout time.Duration) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		reg, err := ServiceIngressEndPoint(ctx, c, clusterName, ns, sName)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("registry endpoint %s", reg)
		srcRef, err := name.ParseReference(image)
		if err != nil {
			t.Fatal(err)
		}

		src, err := daemon.Image(srcRef)
		if err != nil {
			t.Fatal(err)
		}

		i := strings.Split(srcRef.String(), "/")
		err = wait.For(func(_ context.Context) (done bool, err error) {
			err = crane.Push(src, fmt.Sprintf("%s/%s", reg, i[1]), crane.Insecure)
			if err != nil {
				return false, nil //nolint:nilerr // we want to retry and to throw error
			}
			return true, nil
		}, wait.WithTimeout(timeout))
		if err != nil {
			t.Fatalf("copying image `%s` to registry `%s` not successful: %v", image, reg, err)
		}

		return ctx
	}
}

// ComposedResourcesOfClaimHaveFieldValueWithin fails a test if the composed
// resources created by the claim does not have the supplied value at the
// supplied path within the supplied duration.
func ComposedResourcesOfClaimHaveFieldValueWithin(d time.Duration, dir, file, path string, want any, filter func(object k8s.Object) bool) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		cm := &claim.Unstructured{}
		if err := decoder.DecodeFile(os.DirFS(dir), file, cm); err != nil {
			t.Error(err)
			return ctx
		}

		if err := c.Client().Resources().Get(ctx, cm.GetName(), cm.GetNamespace(), cm); err != nil {
			t.Errorf("cannot get claim %s: %v", cm.GetName(), err)
			return ctx
		}

		xrRef := cm.GetResourceReference()
		uxr := &composite.Unstructured{}

		uxr.SetGroupVersionKind(xrRef.GroupVersionKind())
		if err := c.Client().Resources().Get(ctx, xrRef.Name, xrRef.Namespace, uxr); err != nil {
			t.Errorf("cannot get composite %s: %v", xrRef.Name, err)
			return ctx
		}

		mrRefs := uxr.GetResourceReferences()

		list := &unstructured.UnstructuredList{}
		for _, ref := range mrRefs {
			mr := &unstructured.Unstructured{}
			mr.SetName(ref.Name)
			mr.SetNamespace(ref.Namespace)
			mr.SetGroupVersionKind(ref.GroupVersionKind())
			list.Items = append(list.Items, *mr)
		}

		count := atomic.Int32{}
		match := func(o k8s.Object) bool {
			// filter function should return true if the object needs to be checked. e.g., if you want to check the field
			// path of a VPC object, filter function should return true for VPC objects only.
			if filter != nil && !filter(o) {
				t.Logf("skipping resource %s/%s/%s due to filtering", o.GetNamespace(), o.GetName(), o.GetObjectKind().GroupVersionKind().String())
				return true
			}
			count.Add(1)
			u := asUnstructured(o)
			got, err := fieldpath.Pave(u.Object).GetValue(path)
			if err != nil {
				return false
			}

			return cmp.Equal(want, got)
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, match), wait.WithTimeout(d)); err != nil {
			y, _ := yaml.Marshal(list.Items)
			t.Errorf("resources did not have desired value %q at field path %q before timeout (%s): %s\n\n%s\n\n", want, path, d.String(), err, y)

			return ctx
		}

		if count.Load() == 0 {
			t.Errorf("there were no unfiltered referred managed resources to check")
			return ctx
		}

		t.Logf("matching resources had desired value %q at field path %s", want, path)
		return ctx
	}
}

// ListedResourcesValidatedWithin fails a test if the supplied list of resources
// does not have the supplied number of resources that pass the supplied
// validation function within the supplied duration.
func ListedResourcesValidatedWithin(d time.Duration, list k8s.ObjectList, min int, validate func(object k8s.Object) bool, listOptions ...resources.ListOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		if err := wait.For(conditions.New(c.Client().Resources()).ResourceListMatchN(list, min, validate, listOptions...), wait.WithTimeout(d)); err != nil {
			y, _ := yaml.Marshal(list)
			t.Errorf("resources didn't pass validation: %v:\n\n%s\n\n", err, y)
			return ctx
		}

		t.Logf("at least %d resource(s) have desired conditions", min)
		return ctx
	}
}

// ListedResourcesDeletedWithin fails a test if the supplied list of resources
// is not deleted within the supplied duration.
func ListedResourcesDeletedWithin(d time.Duration, list k8s.ObjectList, listOptions ...resources.ListOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		if err := c.Client().Resources().List(ctx, list, listOptions...); err != nil {
			return ctx
		}
		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesDeleted(list), wait.WithTimeout(d)); err != nil {
			y, _ := yaml.Marshal(list)
			t.Errorf("resources wasn't deleted: %v:\n\n%s\n\n", err, y)
			return ctx
		}

		t.Log("resources deleted")
		return ctx
	}
}

// ListedResourcesModifiedWith modifies the supplied list of resources with the
// supplied function and fails a test if the supplied number of resources were
// not modified within the supplied duration.
func ListedResourcesModifiedWith(list k8s.ObjectList, min int, modify func(object k8s.Object), listOptions ...resources.ListOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		if err := c.Client().Resources().List(ctx, list, listOptions...); err != nil {
			return ctx
		}
		var found int
		metaList, err := meta.ExtractList(list)
		if err != nil {
			return ctx
		}
		for _, obj := range metaList {
			if o, ok := obj.(k8s.Object); ok {
				modify(o)
				if err = c.Client().Resources().Update(ctx, o); err != nil {
					t.Errorf("failed to update resource %s/%s: %v", o.GetNamespace(), o.GetName(), err)
					return ctx
				}
				found++
			} else if !ok {
				t.Fatalf("unexpected type %T in list, does not satisfy k8s.Object", obj)
				return ctx
			}
		}
		if found < min {
			t.Errorf("expected minimum %d resources to be modified, found %d", min, found)
			return ctx
		}

		t.Logf("%d resource(s) have been modified", found)
		return ctx
	}
}

// DeletionBlockedByUsageWebhook attempts deleting all resources
// defined by the manifests under the supplied directory that match the supplied
// glob pattern (e.g. *.yaml) and verifies that they are blocked by the usage
// webhook.
func DeletionBlockedByUsageWebhook(dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		err := decoder.DecodeEachFile(ctx, dfs, pattern, decoder.DeleteHandler(c.Client().Resources()))
		if err == nil {
			t.Fatal("expected the usage webhook to deny the request but deletion succeeded")
			return ctx
		}

		if !strings.HasPrefix(err.Error(), "admission webhook \"nousages.apiextensions.crossplane.io\" denied the request") {
			t.Fatalf("expected the usage webhook to deny the request but it failed with err: %s", err.Error())
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("Deletion blocked for resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))
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
	if k == "" {
		t := reflect.TypeOf(o)
		if t != nil {
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			k = t.Name()
		} else {
			k = fmt.Sprintf("%T", o)
		}
	}
	if o.GetNamespace() == "" {
		return fmt.Sprintf("%s %s", k, o.GetName())
	}
	return fmt.Sprintf("%s %s/%s", k, o.GetNamespace(), o.GetName())
}

// FilterByGK returns a filter function that returns true if the supplied object is of the supplied GroupKind.
func FilterByGK(gk schema.GroupKind) func(o k8s.Object) bool {
	return func(o k8s.Object) bool {
		if o.GetObjectKind() == nil {
			return false
		}
		return o.GetObjectKind().GroupVersionKind().Group == gk.Group && o.GetObjectKind().GroupVersionKind().Kind == gk.Kind
	}
}
