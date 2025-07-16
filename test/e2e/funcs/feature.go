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
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	apimachinerywait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/printers"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	kubectlevents "k8s.io/kubectl/pkg/cmd/events"
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
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

// DefaultPollInterval is the suggested poll interval for wait.For.
const DefaultPollInterval = time.Millisecond * 500

type onSuccessHandler func(o k8s.Object)

// AllOf runs the supplied functions in order. If a function fails the test and
// the environment is configured to fail fast (e2e-framework's -fail-fast flag)
// the remaining functions will not be run.
func AllOf(fns ...features.Func) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		for _, fn := range fns {
			//nolint:fatcontext // We want to pass the context to each function.
			ctx = fn(ctx, t, c)
			if t.Failed() && c.FailFast() {
				break
			}
		}

		return ctx
	}
}

// InBackground runs the supplied function in a goroutine.
func InBackground(fn features.Func) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		go fn(ctx, t, c)

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
		t.Helper()

		dp := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		t.Logf("Waiting %s for deployment %s/%s to become Available...", d, dp.GetNamespace(), dp.GetName())

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).DeploymentConditionMatch(dp, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Fatal(err)
			return ctx
		}

		t.Logf("Deployment %s/%s is Available after %s", dp.GetNamespace(), dp.GetName(), since(start))

		return ctx
	}
}

// DeploymentPodIsRunningMustNotChangeWithin fails a test if the supplied Deployment does
// not have a running Pod that stays running for the supplied duration.
func DeploymentPodIsRunningMustNotChangeWithin(d time.Duration, namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		dp := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		t.Logf("Ensuring deployment %s/%s has running pod that does not change within %s", dp.GetNamespace(), dp.GetName(), d.String())

		start := time.Now()

		pod, err := podForDeployment(ctx, t, c, dp)
		if err != nil {
			t.Errorf("Failed to get pod for deployment %s/%s: %s", dp.GetNamespace(), dp.GetName(), err)
			return ctx
		}

		// first wait for pod to be running
		if err := wait.For(conditions.New(c.Client().Resources()).PodConditionMatch(pod, corev1.PodReady, corev1.ConditionTrue), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("Deployment %s/%s never got a running pod after %s", dp.GetNamespace(), dp.GetName(), since(start))
			return ctx
		}

		// now wait to make sure the pod stays running (does not change)
		start = time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).PodConditionMatch(pod, corev1.PodReady, corev1.ConditionFalse), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			if deadlineExceed(err) {
				t.Logf("Deployment %s/%s had running pod that did not change after %s", dp.GetNamespace(), dp.GetName(), since(start))
				return ctx
			}

			t.Errorf("Error while observing pod for deployment %s/%s", dp.GetNamespace(), dp.GetName())

			return ctx
		}

		t.Errorf("Deployment %s/%s had pod that changed within %s, but it should not have", dp.GetNamespace(), dp.GetName(), d.String())

		return ctx
	}
}

// ArgExistsWithin fails a test if the supplied Deployment does not have a Pod with
// the given argument within the supplied duration.
func ArgExistsWithin(d time.Duration, arg, namespace, name string) features.Func {
	return checkArgExistsWithin(d, arg, true, namespace, name)
}

// ArgNotExistsWithin fails a test if the supplied Deployment does not have a Pod with
// the given argument not existing within the supplied duration.
func ArgNotExistsWithin(d time.Duration, arg, namespace, name string) features.Func {
	return checkArgExistsWithin(d, arg, false, namespace, name)
}

// checkArgExistsWithin implements a check for the supplied Deployment having a Pod
// with the given argument either existing or not existing within the supplied
// duration.
func checkArgExistsWithin(d time.Duration, arg string, wantExist bool, namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		dp := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		t.Logf("Waiting %s for pod in deployment %s/%s to have arg %s exist=%t...", d, dp.GetNamespace(), dp.GetName(), arg, wantExist)

		start := time.Now()

		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			pod, err := podForDeployment(ctx, t, c, dp)
			if err != nil {
				t.Logf("failed to get pod for deployment %s/%s: %s", dp.GetNamespace(), dp.GetName(), err)
				return false, nil
			}

			found := false
			c := pod.Spec.Containers[0]
			for _, a := range c.Args {
				if a == arg {
					found = true
				}
			}

			switch {
			case wantExist && !found:
				t.Logf("did not find arg %s within %s", arg, c.Args)
				return false, nil
			case !wantExist && found:
				t.Logf("unexpectedly found arg %s within %s", arg, c.Args)
				return false, nil
			default:
				return true, nil
			}
		}, wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Fatal(err)
			return ctx
		}

		t.Logf("Deployment %s/%s has pod with arg %s exist=%t after %s", dp.GetNamespace(), dp.GetName(), arg, wantExist, since(start))

		return ctx
	}
}

// ResourcesCreatedWithin fails a test if the supplied resources are not found
// to exist within the supplied duration.
func ResourcesCreatedWithin(d time.Duration, dir, pattern string, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern, options...)
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

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesFound(list), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("resources did not exist: %v", err)
			return ctx
		}

		t.Logf("%d resources found to exist after %s", len(rs), since(start))

		return ctx
	}
}

// ResourceCreatedWithin fails a test if the supplied resource is not found to
// exist within the supplied duration.
func ResourceCreatedWithin(d time.Duration, o k8s.Object) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		t.Logf("Waiting %s for %s to be created...", d, identifier(o))

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(o, func(_ k8s.Object) bool { return true }), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("resource %s did not exist: %v", identifier(o), err)
			return ctx
		}

		t.Logf("resource %s found to exist after %s", identifier(o), since(start))

		return ctx
	}
}

// ResourcesDeletedWithin fails a test if the supplied resources are not deleted
// within the supplied duration.
func ResourcesDeletedWithin(d time.Duration, dir, pattern string, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern, options...)
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

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesDeleted(list), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			objs := itemsToObjects(list.Items)
			related, _ := RelatedObjects(ctx, t, c.Client().RESTConfig(), objs...)
			events := valueOrError(eventString(ctx, c.Client().RESTConfig(), append(objs, related...)...))

			t.Errorf("resources not deleted: %v:\n\n%s\n%s\nRelated objects:\n\n%s\n", err, toYAML(objs...), events, toYAML(related...))

			return ctx
		}

		t.Logf("%d resources deleted after %s", len(rs), since(start))

		return ctx
	}
}

// ResourceDeletedWithin fails a test if the supplied resource is not deleted
// within the supplied duration.
func ResourceDeletedWithin(d time.Duration, o k8s.Object) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		t.Logf("Waiting %s for %s to be deleted...", d, identifier(o))

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceDeleted(o), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("resource %s not deleted: %v", identifier(o), err)
			return ctx
		}

		t.Logf("resource %s deleted after %s", identifier(o), since(start))

		return ctx
	}
}

// ResourceHasConditionWithin checks if a single resource becomes the supplied
// conditions within the supplied duration. Comparison of conditions is modulo
// messages.
func ResourceHasConditionWithin(d time.Duration, o k8s.Object, cds ...xpv1.Condition) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		reasons := make([]string, len(cds))
		for i := range cds {
			reasons[i] = string(cds[i].Reason)
			if cds[i].Message != "" {
				t.Errorf("message must not be set in ResourceHasConditionWithin: %s", cds[i].Message)
			}
		}

		desired := strings.Join(reasons, ", ")

		t.Logf("Waiting %s for %s to become %s...", d, identifier(o), desired)

		ogReport := make(map[string]bool)
		old := make([]xpv1.Condition, len(cds))
		match := func(o k8s.Object) bool {
			u := asUnstructured(o)
			s := xpv1.ConditionedStatus{}
			_ = fieldpath.Pave(u.Object).GetValueInto("status", &s)

			for i, want := range cds {
				// Update the wanted observed generation to the latest object generation.
				want.ObservedGeneration = u.GetGeneration()

				got := s.GetCondition(want.Type)

				// Until https://github.com/crossplane/crossplane/issues/6420 is resolved, crossplane will be in a
				// transition period. A condition with an observedGeneration of zero means it is not yet being
				// propagated when setting the conditions. To help that transition, we will move the generation forward
				// ONLY if it is zero. But we will also log this fact to find it in the logs.
				if got.ObservedGeneration == 0 {
					got.ObservedGeneration = u.GetGeneration()

					key := fmt.Sprintf("%s[%s]", u.GetKind(), got.Type)
					if !ogReport[key] {
						ogReport[key] = true

						t.Logf("https://github.com/crossplane/crossplane/issues/6420: Warning, an unset observedGeneration was artificially updated for %s.status.conditions[%s]", u.GetKind(), got.Type)
					}
				}

				if !got.Equal(old[i]) {
					old[i] = got
					t.Logf("- CONDITION: %s[@%d]: %s=%s[@%d] Reason=%s: %s (%s)",
						identifier(u), u.GetGeneration(), got.Type, got.Status, got.ObservedGeneration, got.Reason, or(got.Message, `""`), got.LastTransitionTime)
				}

				// do compare modulo message as the message in e2e tests
				// might differ between runs and is not meant for machines.
				got.Message = ""
				if !got.Equal(want) {
					return false
				}
			}

			return true
		}

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(o, match), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			related, _ := RelatedObjects(ctx, t, c.Client().RESTConfig(), o)
			events := valueOrError(eventString(ctx, c.Client().RESTConfig(), append(related, o)...))

			t.Errorf("resource did not have desired conditions: %s: %v:\n\n%s\n%s\nRelated objects:\n\n%s\n", desired, err, toYAML(o), events, toYAML(related...))

			return ctx
		}

		t.Logf("Resource has desired conditions after %s: %s", since(start), desired)

		return ctx
	}
}

// ResourcesHaveConditionWithin fails a test if the supplied resources do not
// have (i.e. become) the supplied conditions within the supplied duration.
// Comparison of conditions is modulo messages.
func ResourcesHaveConditionWithin(d time.Duration, dir, pattern string, cds ...xpv1.Condition) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Error(err)
			return ctx
		}

		for _, o := range rs {
			u := asUnstructured(o)
			ResourceHasConditionWithin(d, u, cds...)(ctx, t, c)
		}

		return ctx
	}
}

func or(a, b string) string {
	if a != "" {
		return a
	}

	return b
}

// CRDInitialNamesAccepted is the status condition CRDs emit when they're
// established. Most of our Crossplane status conditions are defined elsewhere
// (e.g. in the xpv1 package), but this isn't so we define it here for
// convenience.
func CRDInitialNamesAccepted() xpv1.Condition {
	return xpv1.Condition{
		Type:   "Established",
		Status: corev1.ConditionTrue,
		Reason: "InitialNamesAccepted",
	}
}

type notFound struct{}

func (nf notFound) String() string { return "NotFound" }

// NotFound is a special 'want' value that indicates the supplied path should
// not be found.
var NotFound = notFound{} //nolint:gochecknoglobals // We treat this as a constant.

type anyValue struct{}

func (av anyValue) String() string { return "Any" }

// Any is a special 'want' value that indicates any value (except NotFound) should match.
var Any = anyValue{} //nolint:gochecknoglobals // We treat this as a constant.

// ResourcesHaveFieldValueWithin fails a test if the supplied resources do not
// have the supplied value at the supplied field path within the supplied
// duration. The supplied 'want' value must cmp.Equal the actual value.
func ResourcesHaveFieldValueWithin(d time.Duration, dir, pattern, path string, want any, options ...decoder.DecodeOption) features.Func { //nolint:gocognit // Only a little over.
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern, options...)
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
			if fieldpath.IsNotFound(err) {
				if _, ok := want.(notFound); ok {
					return true
				}
				// If we want Any but got NotFound, it doesn't match
				if _, ok := want.(anyValue); ok {
					return false
				}
			}

			if err != nil {
				return false
			}

			// If we want Any and we have a value (not NotFound), it matches
			if _, ok := want.(anyValue); ok {
				return true
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Logf("%s doesn't yet have desired value at field path %s: %s", identifier(u), path, diff)
				return false
			}

			return true
		}

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, match), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			objs := make([]client.Object, len(list.Items))
			for i := range list.Items {
				objs[i] = &list.Items[i]
			}

			t.Errorf("resources did not have desired value %q at field path %s: %v:\n\n%s\n\n", want, path, err, toYAML(objs...))

			return ctx
		}

		if count.Load() == 0 {
			t.Errorf("no resources matched pattern %s", filepath.Join(dir, pattern))
			return ctx
		}

		t.Logf("%d resources have desired value %q at field path %s after %s", len(rs), want, path, since(start))

		return ctx
	}
}

// ResourceHasFieldValueWithin fails a test if the supplied resource does not
// have the supplied value at the supplied field path within the supplied
// duration. The supplied 'want' value must cmp.Equal the actual value.
func ResourceHasFieldValueWithin(d time.Duration, o k8s.Object, path string, want any) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		t.Logf("Waiting %s for %s to have value %q at field path %s...", d, identifier(o), want, path)

		match := func(o k8s.Object) bool {
			u := asUnstructured(o)

			got, err := fieldpath.Pave(u.Object).GetValue(path)
			if fieldpath.IsNotFound(err) {
				if _, ok := want.(notFound); ok {
					return true
				}
				// If we want Any but got NotFound, it doesn't match
				if _, ok := want.(anyValue); ok {
					return false
				}
			}

			if err != nil {
				return false
			}

			// If we want Any and we have a value (not NotFound), it matches
			if _, ok := want.(anyValue); ok {
				return true
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Logf("%s doesn't yet have desired value at field path %s: %s", identifier(u), path, diff)
				return false
			}

			return true
		}

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(o, match), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			y, _ := yaml.Marshal(o)
			t.Errorf("resource did not have desired value %q at field path %s: %v:\n\n%s\n\n", want, path, err, y)

			return ctx
		}

		t.Logf("%s has desired value %q at field path %s after %s", identifier(o), want, path, since(start))

		return ctx
	}
}

// ApplyResources applies all manifests under the supplied directory that match
// the supplied glob pattern (e.g. *.yaml). It uses server-side apply - fields
// are managed by the supplied field manager. It fails the test if any supplied
// resource cannot be applied successfully.
func ApplyResources(manager, dir, pattern string, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		dfs := os.DirFS(dir)

		files, _ := fs.Glob(dfs, pattern)
		if len(files) == 0 {
			t.Errorf("No resources found in %s", filepath.Join(dir, pattern))
			return ctx
		}

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, ApplyHandler(c.Client().Resources(), manager), options...); err != nil {
			t.Fatal(err)
			return ctx
		}

		t.Logf("Applied resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))

		return ctx
	}
}

type claimCtxKey struct{}

// ApplyClaim applies the claim stored in the given folder and file
// and stores it in the test context for later retrival if needed.
func ApplyClaim(manager, dir, cm string, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		dfs := os.DirFS(dir)

		files, _ := fs.Glob(dfs, cm)
		if len(files) == 0 {
			t.Errorf("No resources found in %s", filepath.Join(dir, cm))
			return ctx
		}

		objs, err := decoder.DecodeAllFiles(ctx, dfs, cm, options...)
		if err != nil {
			t.Error(err)
			return ctx
		}

		if len(objs) != 1 {
			t.Errorf("Only one claim allows in %s", filepath.Join(dir, cm))
			return ctx
		}
		// TODO(negz): Only two functions seem to read this key. Either adopt it
		// everywhere it would be relevant, or drop it.
		f := func(o k8s.Object) {
			ctx = context.WithValue(ctx, claimCtxKey{}, &claim.Unstructured{Unstructured: *asUnstructured(o)}) //nolint:fatcontext // We know we have a single claim.
		}
		if err := decoder.DecodeEachFile(ctx, dfs, cm, ApplyHandler(c.Client().Resources(), manager, f)); err != nil {
			t.Fatal(err)
			return ctx
		}

		t.Logf("Applied resources from %s (matched %d manifests)", filepath.Join(dir, cm), len(files))

		return ctx
	}
}

// SetAnnotationMutateOption returns a DecodeOption that sets the supplied
// annotation on the decoded object.
func SetAnnotationMutateOption(key, value string) decoder.DecodeOption {
	return decoder.MutateAnnotations(map[string]string{key: value})
}

// ResourcesFailToApply applies all manifests under the supplied directory that
// match the supplied glob pattern (e.g. *.yaml). It uses server-side apply -
// fields are managed by the supplied field manager. It fails the test if any
// supplied resource _can_ be applied successfully - use it to test that the API
// server should reject a resource.
func ResourcesFailToApply(manager, dir, pattern string, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, ApplyHandler(c.Client().Resources(), manager), options...); err == nil {
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
func ApplyHandler(r *resources.Resources, manager string, osh ...onSuccessHandler) decoder.HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		// TODO(negz): Use r.Patch when the below issue is solved?
		// https://github.com/kubernetes-sigs/e2e-framework/issues/254

		// TODO(negz): Make forcing optional? It seems to be necessary
		// sometimes, e.g. due to conflicts with a provider managing the same
		// fields. I'm guessing controller-runtime is setting providers as a
		// field manager at create time even though it doesn't use SSA?
		if err := r.GetControllerRuntimeClient().Patch(ctx, obj, client.Apply, client.FieldOwner(manager), client.ForceOwnership); err != nil {
			return err
		}

		for _, h := range osh {
			h(obj)
		}

		return nil
	}
}

// DeleteResourcesWithPropagationPolicy deletes (from the environment) all
// resources defined by the manifests under the supplied directory that match
// the supplied glob pattern (e.g. *.yaml). Deletion is done using the
// supplied deletion propagation policy.
func DeleteResourcesWithPropagationPolicy(dir, pattern string, deletePropagation metav1.DeletionPropagation, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, decoder.DeleteHandler(c.Client().Resources(), resources.WithDeletePropagation(string(deletePropagation))), options...); err != nil {
			t.Fatal(err)
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("Deleted resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))

		return ctx
	}
}

// DeleteResources deletes (from the environment) all resources defined by the
// manifests under the supplied directory that match the supplied glob pattern
// (e.g. *.yaml).
func DeleteResources(dir, pattern string, options ...decoder.DecodeOption) features.Func {
	return DeleteResourcesWithPropagationPolicy(dir, pattern, metav1.DeletePropagationBackground, options...)
}

// ClaimUnderTestMustNotChangeWithin asserts that the claim available in
// the test context does not change within the given time.
func ClaimUnderTestMustNotChangeWithin(d time.Duration) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		cm, ok := ctx.Value(claimCtxKey{}).(*claim.Unstructured)
		if !ok {
			t.Fatalf("claim not available in the context")
			return ctx
		}

		list := &unstructured.UnstructuredList{}
		ucm := unstructured.Unstructured{}
		ucm.SetNamespace(cm.GetNamespace())
		ucm.SetName(cm.GetName())
		ucm.SetGroupVersionKind(cm.GroupVersionKind())
		list.Items = append(list.Items, ucm)

		m := func(o k8s.Object) bool {
			return o.GetGeneration() != cm.GetGeneration()
		}

		t.Logf("Ensuring claim %s does not change within %s", identifier(cm), d.String())

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, m), wait.WithTimeout(d)); err != nil {
			if deadlineExceed(err) {
				t.Logf("Claim %s did not change within %s", identifier(cm), d.String())
				return ctx
			}

			t.Errorf("Error while observing claim %s: %v", identifier(cm), err)

			return ctx
		}

		t.Errorf("Claim %s changed within %s, but it should not have", identifier(cm), d.String())

		return ctx
	}
}

// CompositeUnderTestMustNotChangeWithin asserts that the claim available in
// the test context does not change within the given time.
func CompositeUnderTestMustNotChangeWithin(d time.Duration) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		cm, ok := ctx.Value(claimCtxKey{}).(*claim.Unstructured)
		if !ok {
			t.Fatalf("claim not available in the context")
			return ctx
		}

		if err := c.Client().Resources().Get(ctx, cm.GetName(), cm.GetNamespace(), cm); err != nil {
			t.Errorf("Error while getting claim: %v", err)
			return ctx
		}

		cp := &composite.Unstructured{}
		cp.SetName(cm.GetResourceReference().Name)
		cp.SetGroupVersionKind(cm.GetResourceReference().GroupVersionKind())

		if err := c.Client().Resources().Get(ctx, cp.GetName(), cp.GetNamespace(), cp); err != nil {
			t.Errorf("Error while getting composite: %v", err)
			return ctx
		}

		list := &unstructured.UnstructuredList{}
		ucp := unstructured.Unstructured{}
		ucp.SetName(cp.GetName())
		ucp.SetGroupVersionKind(cp.GroupVersionKind())
		list.Items = append(list.Items, ucp)

		m := func(o k8s.Object) bool {
			return o.GetResourceVersion() != cp.GetResourceVersion()
		}

		t.Logf("Ensuring composite resource %s does not change within %s", identifier(cp), d.String())

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, m), wait.WithTimeout(d)); err != nil {
			if deadlineExceed(err) {
				t.Logf("Composite resource %s did not change within %s", identifier(cp), d.String())
				return ctx
			}

			t.Errorf("Error while observing composite resource %s: %v", identifier(cp), err)

			return ctx
		}

		t.Errorf("Composite resource %s changed within %s, but it should not have", identifier(cp), d.String())

		return ctx
	}
}

// CompositeResourceMustMatchWithin assert that a composite referred by the given file
// must be matched by the given function within the given timeout.
func CompositeResourceMustMatchWithin(d time.Duration, dir, claimFile string, match func(xr *composite.Unstructured) bool, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		cm := &claim.Unstructured{}

		if err := decoder.DecodeFile(os.DirFS(dir), claimFile, cm, options...); err != nil {
			t.Error(err)
			return ctx
		}

		if err := c.Client().Resources().Get(ctx, cm.GetName(), cm.GetNamespace(), cm); err != nil {
			t.Errorf("cannot get claim %s: %v", cm.GetName(), err)
			return ctx
		}

		xrRef := cm.GetResourceReference()

		list := &unstructured.UnstructuredList{}

		uxr := unstructured.Unstructured{}
		uxr.SetName(xrRef.Name)
		uxr.SetGroupVersionKind(xrRef.GroupVersionKind())

		list.Items = append(list.Items, uxr)

		count := atomic.Int32{}
		m := func(o k8s.Object) bool {
			count.Add(1)

			u := asUnstructured(o)

			return match(&composite.Unstructured{Unstructured: *u})
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, m), wait.WithTimeout(d)); err != nil && count.Load() > 0 {
			t.Errorf("composite %s did not match the condition before timeout (%s): %s\n\n", identifier(&uxr), d.String(), err)
			return ctx
		}

		if count.Load() == 0 {
			t.Errorf("there were composite resource %s", identifier(&uxr))
			return ctx
		}

		t.Logf("composite resource %s matched", identifier(&uxr))

		return ctx
	}
}

// CompositeResourceHasFieldValueWithin asserts that the XR referred to by the
// claim in the given file has the specified value at the specified path within
// the specified time.
func CompositeResourceHasFieldValueWithin(d time.Duration, dir, claimFile, path string, want any, options ...decoder.DecodeOption) features.Func { //nolint:gocognit // Only a little over.
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		cm := &claim.Unstructured{}

		if err := decoder.DecodeFile(os.DirFS(dir), claimFile, cm, options...); err != nil {
			t.Error(err)
			return ctx
		}

		hasResourceRef := func(o k8s.Object) bool {
			u := asUnstructured(o)

			got, err := fieldpath.Pave(u.Object).GetString("spec.resourceRef.name")
			if err != nil {
				return false
			}

			return got != ""
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(cm, hasResourceRef), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval), wait.WithImmediate()); err != nil {
			t.Errorf("Claim %q does not have a resourceRef to an XR: %v", cm.GetName(), err)
			return ctx
		}

		xrRef := cm.GetResourceReference()
		xr := &unstructured.Unstructured{}
		xr.SetGroupVersionKind(xrRef.GroupVersionKind())
		xr.SetName(xrRef.Name)

		count := atomic.Int32{}
		match := func(o k8s.Object) bool {
			count.Add(1)

			u := asUnstructured(o)

			got, err := fieldpath.Pave(u.Object).GetValue(path)
			if fieldpath.IsNotFound(err) {
				if _, ok := want.(notFound); ok {
					return true
				}
				// If we want Any but got NotFound, it doesn't match
				if _, ok := want.(anyValue); ok {
					return false
				}
			}

			if err != nil {
				return false
			}

			// If we want Any and we have a value (not NotFound), it matches
			if _, ok := want.(anyValue); ok {
				return true
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Logf("%s doesn't yet have desired value at field path %s: %s", identifier(xr), path, diff)
				return false
			}

			return true
		}

		start := time.Now()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(xr, match), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			y, _ := yaml.Marshal(xr)
			t.Errorf("XR did not have desired value %q at field path %s: %v:\n\n%s\n\n", want, path, err, y)

			return ctx
		}

		if count.Load() == 0 {
			t.Errorf("no resources matched pattern %s", filepath.Join(dir, claimFile))
			return ctx
		}

		t.Logf("%s has desired value %q at field path %s after %s", identifier(xr), want, path, since(start))

		return ctx
	}
}

// ComposedResourcesHaveFieldValueWithin fails a test if the composed
// resources created by the claim does not have the supplied value at the
// supplied path within the supplied duration.
func ComposedResourcesHaveFieldValueWithin(d time.Duration, dir, file, path string, want any, filter func(object k8s.Object) bool, options ...decoder.DecodeOption) features.Func { //nolint:gocognit // Not too much over.
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		cm := &claim.Unstructured{}
		if err := decoder.DecodeFile(os.DirFS(dir), file, cm, options...); err != nil {
			t.Error(err)
			return ctx
		}

		hasResourceRef := func(o k8s.Object) bool {
			u := asUnstructured(o)

			got, err := fieldpath.Pave(u.Object).GetString("spec.resourceRef.name")
			if err != nil {
				return false
			}

			return got != ""
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(cm, hasResourceRef), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("Claim %q does not have a resourceRef to an XR: %v", cm.GetName(), err)
			return ctx
		}

		// We always find this XR from a claim, so it'll always be legacy.
		uxr := &composite.Unstructured{Schema: composite.SchemaLegacy}

		xrRef := cm.GetResourceReference()
		uxr.SetGroupVersionKind(xrRef.GroupVersionKind())

		if err := c.Client().Resources().Get(ctx, xrRef.Name, "", uxr); err != nil {
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
			if fieldpath.IsNotFound(err) {
				if _, ok := want.(notFound); ok {
					return true
				}
				// If we want Any but got NotFound, it doesn't match
				if _, ok := want.(anyValue); ok {
					return false
				}
			}

			if err != nil {
				return false
			}

			// If we want Any and we have a value (not NotFound), it matches
			if _, ok := want.(anyValue); ok {
				return true
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Logf("%s doesn't yet have desired value at field path %s: %s", identifier(u), path, diff)
				return false
			}

			return true
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesMatch(list, match), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
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
func ListedResourcesValidatedWithin(d time.Duration, list k8s.ObjectList, minObjects int, validate func(object k8s.Object) bool, listOptions ...resources.ListOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		if err := wait.For(conditions.New(c.Client().Resources()).ResourceListMatchN(list, minObjects, validate, listOptions...), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			y, _ := yaml.Marshal(list)
			t.Errorf("resources didn't pass validation: %v:\n\n%s\n\n", err, y)

			return ctx
		}

		t.Logf("at least %d resource(s) have desired conditions", minObjects)

		return ctx
	}
}

// ListedResourcesDeletedWithin fails a test if the supplied list of resources
// is not deleted within the supplied duration.
func ListedResourcesDeletedWithin(d time.Duration, list k8s.ObjectList, listOptions ...resources.ListOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		if err := c.Client().Resources().List(ctx, list, listOptions...); err != nil {
			return ctx
		}

		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesDeleted(list), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
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
func ListedResourcesModifiedWith(list k8s.ObjectList, minObjects int, modify func(object k8s.Object), listOptions ...resources.ListOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

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

		if found < minObjects {
			t.Errorf("expected minimum %d resources to be modified, found %d", minObjects, found)
			return ctx
		}

		t.Logf("%d resource(s) have been modified", found)

		return ctx
	}
}

// LogResources polls the given kind of resources and logs creations, deletions
// and changed conditions.
func LogResources(list k8s.ObjectList, listOptions ...resources.ListOption) features.Func { //nolint:gocognit // this is a test helper
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		prev := map[string]map[xpv1.ConditionType]xpv1.Condition{}

		pollCtx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		_ = apimachinerywait.PollUntilContextCancel(pollCtx, 500*time.Millisecond, true, func(ctx context.Context) (done bool, err error) {
			if err := c.Client().Resources().List(ctx, list, listOptions...); err != nil {
				return false, nil //nolint:nilerr // retry and ignore the error
			}

			metaList, err := meta.ExtractList(list)
			if err != nil {
				return false, err
			}

			found := map[string]bool{}

			for _, obj := range metaList {
				obj, ok := obj.(k8s.Object)
				if !ok {
					return false, fmt.Errorf("unexpected type %T in list, does not satisfy k8s.Object", obj)
				}

				id := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
				if _, ok := prev[id]; !ok {
					t.Logf("- CREATED:   %s (%s)", identifier(obj), obj.GetCreationTimestamp().String())
				}

				u := asUnstructured(obj)
				s := xpv1.ConditionedStatus{}
				_ = fieldpath.Pave(u.Object).GetValueInto("status", &s)

				got := map[xpv1.ConditionType]xpv1.Condition{}
				for _, c := range s.Conditions {
					got[c.Type] = c
				}

				for ty, c := range got {
					if !c.Equal(prev[id][ty]) {
						t.Logf("- CONDITION: %s: %s=%s Reason=%s: %s (%s)", identifier(u), c.Type, c.Status, c.Reason, or(c.Message, `""`), c.LastTransitionTime)
					}
				}

				for ty, c := range prev[id] {
					if _, ok := got[ty]; !ok {
						t.Logf("- %s: %s disappeared", identifier(u), c.Type)
					}
				}

				prev[id] = got
				found[id] = true
			}

			for id := range prev {
				if _, ok := found[id]; !ok {
					t.Logf("- DELETED:   %s", id)
					delete(prev, id)
				}
			}

			return false, nil
		})

		return ctx
	}
}

// DeletionBlockedByUsageWebhook attempts deleting all resources
// defined by the manifests under the supplied directory that match the supplied
// glob pattern (e.g. *.yaml) and verifies that they are blocked by the usage
// webhook.
func DeletionBlockedByUsageWebhook(dir, pattern string, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		dfs := os.DirFS(dir)

		err := decoder.DecodeEachFile(ctx, dfs, pattern, decoder.DeleteHandler(c.Client().Resources()), options...)
		if err == nil {
			t.Fatal("expected the usage webhook to deny the request but deletion succeeded")
			return ctx
		}

		if !strings.Contains(err.Error(), "admission webhook \"nousages.protection.crossplane.io\" denied the request") {
			t.Fatalf("expected the usage webhook to deny the request but it failed with err: %s", err.Error())
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("Deletion blocked for resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))

		return ctx
	}
}

// ResourcesDeletedAfterListedAreGone will ensure that the resources matching
// the supplied pattern under the supplied directory are deleted after the
// supplied list of resources are deleted.
func ResourcesDeletedAfterListedAreGone(d time.Duration, dir, pattern string, list k8s.ObjectList, listOptions ...resources.ListOption) features.Func {
	return AllOf(
		ListedResourcesDeletedWithin(d, list, listOptions...),
		DeleteResourcesWithPropagationPolicy(dir, pattern, metav1.DeletePropagationForeground),
		ResourcesDeletedWithin(d, dir, pattern),
	)
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

// identifier returns the supplied resource's kind, group, name, and (if any)
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

	groupSuffix := ""
	if g := o.GetObjectKind().GroupVersionKind().Group; g != "" {
		groupSuffix = "." + g
	}

	if o.GetNamespace() == "" {
		return fmt.Sprintf("%s%s %s", k, groupSuffix, o.GetName())
	}

	return fmt.Sprintf("%s%s %s/%s", k, groupSuffix, o.GetNamespace(), o.GetName())
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

func toYAML(objs ...client.Object) string {
	docs := make([]string, 0, len(objs))
	for _, obj := range objs {
		// First round-trip the object through JSON to make it
		// unstructured.
		j, _ := json.Marshal(obj)
		u := &unstructured.Unstructured{}
		_ = json.Unmarshal(j, u)

		p := fieldpath.Pave(u.Object)

		// Remove managed fields. They're seldom useful when
		// troubleshooting a broken test, and distract from the rest of
		// the object.
		_ = p.DeleteField("metadata.managedFields")

		// For resources like XRDs and CRDs with huge specs, filter it
		// out. Otherwise it usually overflows the backscroll when a
		// test fails.
		if u.GetKind() == "CustomResourceDefinition" || u.GetKind() == "CompositeResourceDefinition" {
			_ = p.DeleteField("spec")
		}

		oy, _ := yaml.Marshal(u)
		docs = append(docs, string(oy))
	}

	return strings.Join(docs, "---\n")
}

func eventString(ctx context.Context, cfg *rest.Config, objs ...client.Object) (string, error) {
	c, err := client.New(cfg, client.Options{Scheme: clientgoscheme.Scheme})
	if err != nil {
		return "", err
	}

	evts := &corev1.EventList{}

	for _, o := range objs {
		opts := []client.ListOption{
			client.MatchingFields{"involvedObject.uid": string(o.GetUID())},
		}
		if ns := o.GetNamespace(); ns != "" {
			opts = append(opts, client.InNamespace(ns))
		}

		list := &corev1.EventList{}
		if err := c.List(ctx, list, opts...); err != nil {
			return "", errors.Errorf("failed to list events: %v", err)
		}

		evts.Items = append(evts.Items, list.Items...)
	}

	if len(evts.Items) == 0 {
		return "", nil
	}

	sort.Sort(kubectlevents.SortableEvents(evts.Items))

	var buf bytes.Buffer

	w := printers.GetNewTabWriter(&buf)
	if err := kubectlevents.NewEventPrinter(false, true).PrintObj(evts, w); err != nil {
		return "", errors.Errorf("failed to print events: %v", err)
	}

	_ = w.Flush()

	return buf.String(), nil
}

func valueOrError(s string, err error) string {
	if err != nil {
		return err.Error()
	}

	return s
}

func itemsToObjects(items []unstructured.Unstructured) []client.Object {
	objects := make([]client.Object, len(items))
	for i, item := range items {
		objects[i] = &item
	}

	return objects
}

func since(t time.Time) string {
	return fmt.Sprintf("%.3fs", time.Since(t).Seconds())
}

func deadlineExceed(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "would exceed context deadline")
}

// podForDeployment returns the pod for a given Deployment. If the number of
// pods found is not exactly one, or that one pod does not have exactly one
// container, then this function returns an error.
func podForDeployment(ctx context.Context, t *testing.T, c *envconf.Config, dp *appsv1.Deployment) (*corev1.Pod, error) {
	t.Helper()

	if err := c.Client().Resources().Get(ctx, dp.GetName(), dp.GetNamespace(), dp); err != nil {
		t.Logf("failed to get deployment %s/%s: %s", dp.GetNamespace(), dp.GetName(), err)
		return nil, err
	}

	// use the deployment's selector to list all pods belonging to the deployment
	selector := metav1.FormatLabelSelector(dp.Spec.Selector)

	pods := &corev1.PodList{}
	if err := c.Client().Resources().List(ctx, pods, resources.WithLabelSelector(selector)); err != nil {
		t.Logf("failed to list pods for deployment %s/%s: %s", dp.GetNamespace(), dp.GetName(), err)
		return nil, err
	}

	if len(pods.Items) != 1 {
		return nil, errors.Errorf("expected 1 pod, found %d", len(pods.Items))
	}

	pod := pods.Items[0]
	if len(pod.Spec.Containers) != 1 {
		return nil, errors.Errorf("expected 1 container, found %d", len(pod.Spec.Containers))
	}

	return &pod, nil
}
