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

package e2e

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/test/e2e/config"
	"github.com/crossplane/crossplane/v2/test/e2e/funcs"
)

// Tests for the alpha feature that lets functions declare ordering constraints
// over composed resources.
const SuiteComposedResourceOrdering = "composed-resource-ordering"

const (
	// The XR in the ordering manifests, and the namespace it and its composed
	// resources live in. Not crossplane-system, which is what the package
	// level namespace constant refers to.
	orderedXRName      = "ordered"
	orderedXRNamespace = "default"
)

func init() {
	environment.AddTestSuite(SuiteComposedResourceOrdering,
		config.WithHelmInstallOpts(
			helm.WithArgs("--set args={--debug,--enable-composed-resource-ordering}"),
		),
		config.WithLabelsToSelect(features.Labels{
			config.LabelTestSuite: []string{SuiteComposedResourceOrdering},
		}),
	)
}

// deletingByResourceName reports, per composition resource name, whether that
// composed resource has been asked to delete. A resource that has finished
// deleting is absent from the map.
//
// The composed resources are found through the XR's own
// spec.crossplane.resourceRefs rather than by listing a kind: the references
// are what Crossplane itself uses to observe them, they carry the composition
// resource name directly, and they don't depend on the test guessing the right
// kind or label selector.
func deletingByResourceName(ctx context.Context, t *testing.T, c *envconf.Config) map[string]bool {
	t.Helper()

	xr := &unstructured.Unstructured{}
	xr.SetAPIVersion("ordering.example.org/v1alpha1")
	xr.SetKind("XOrdering")

	if err := c.Client().Resources(orderedXRNamespace).Get(ctx, orderedXRName, orderedXRNamespace, xr); err != nil {
		if kerrors.IsNotFound(err) {
			// The XR is gone. During teardown that should be impossible until
			// every composed resource has gone first, because the XR holds its
			// own finalizer until then. Reaching here means ordered teardown
			// didn't engage and Kubernetes cascaded instead - so say that,
			// rather than reporting it as "no composed resources left".
			logCrossplaneTeardown(ctx, t, c)
			t.Fatalf("XR is already gone; ordered teardown should hold its finalizer until every composed resource has been deleted. Teardown is not engaging.")
		}

		t.Fatalf("cannot get XR: %v", err)
	}

	refs, _, err := unstructured.NestedSlice(xr.Object, "spec", "crossplane", "resourceRefs")
	if err != nil {
		t.Fatalf("cannot read spec.crossplane.resourceRefs: %v", err)
	}

	out := map[string]bool{}

	for _, r := range refs {
		ref, ok := r.(map[string]any)
		if !ok {
			continue
		}

		name, _, _ := unstructured.NestedString(ref, "resourceName")
		if name == "" {
			t.Fatalf("reference %v carries no resourceName; the XR's references should record it", ref)
		}

		cd := &unstructured.Unstructured{}
		cd.SetAPIVersion(ref["apiVersion"].(string))
		cd.SetKind(ref["kind"].(string))

		objName, _, _ := unstructured.NestedString(ref, "name")

		err := c.Client().Resources(orderedXRNamespace).Get(ctx, objName, orderedXRNamespace, cd)
		switch {
		case kerrors.IsNotFound(err):
			// Finished deleting, so absent from the map.
			continue
		case err != nil:
			t.Fatalf("cannot get composed resource %q: %v", name, err)
		}

		out[name] = cd.GetDeletionTimestamp() != nil
	}

	return out
}

// composedStateIs asserts which composed resources exist and which of them
// have been asked to delete. Names mapped to true must be deleting; names
// mapped to false must exist and not be deleting; names absent from want must
// be gone.
//
// This is what distinguishes ordered teardown from a cascade. A cascade asks
// every resource to delete at once, so it would fail the first time this runs.
// The assertion is only observable because each NopResource takes
// spec.forProvider.deleteAfter to go away.
func composedStateIs(want map[string]bool) features.Func {
	return composedStateIsWithin(45*time.Second, want)
}

// composedStateIsWithin is composedStateIs with an explicit deadline.
//
// A graph more than a few levels deep trips the realtime compositions watch
// circuit breaker - the XR reports "Too many watch events" - after which
// events are only allowed periodically and each remaining wave takes about a
// minute instead of seconds. That is documented behavior, not a hang, so
// deeper graphs need a deadline well above the sum of their readyAfter values.
// See notes-circuit-breaker-scale-findings.md.
func composedStateIsWithin(d time.Duration, want map[string]bool) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		err := wait.For(func(context.Context) (bool, error) {
			got := deletingByResourceName(ctx, t, c)
			if len(got) != len(want) {
				return false, nil
			}

			for name, deleting := range want {
				if d, ok := got[name]; !ok || d != deleting {
					return false, nil
				}
			}

			return true, nil
		}, wait.WithTimeout(d), wait.WithInterval(time.Second))
		if err != nil {
			t.Logf("want: %v", want)
			t.Logf("got:  %v (absent names have finished deleting)", deletingByResourceName(ctx, t, c))
			logXRReferences(ctx, t, c)
			t.Fatalf("composed resources did not reach the expected teardown state: %v", err)
		}

		return ctx
	}
}

func TestComposedResourceOrderingTeardownIsOrdered(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/ordering"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that XR teardown deletes composed resources one dependency level at a time, leaves first, rather than all at once. Each NopResource takes 20s to delete, which is what makes the order observable.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteComposedResourceOrdering).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("UseTheSlowDeletingComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "teardown/composition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "teardown/composition.yaml"),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Available()),
			).
			// Check the graph reached the XR before deleting anything. Ordered
			// teardown rebuilds it from these references, so if they don't
			// carry it, teardown silently does nothing and the XR cascades -
			// which is indistinguishable from teardown having finished.
			Assess("GraphIsPersistedOnTheXR", xrReferencesCarryGraph()).
			Assess("DeleteXR",
				// Background propagation. Foreground cascade makes Kubernetes
				// delete anything carrying blockOwnerDeletion as soon as the
				// owner has a deletion timestamp, racing the ordered teardown.
				// That's a documented limitation, not something to test here.
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr.yaml", metav1.DeletePropagationBackground),
			).
			Assess("OnlyTheLeafIsDeleting",
				composedStateIs(map[string]bool{"first": false, "second": false, "third": true}),
			).
			Assess("ThenSecondIsDeleting",
				composedStateIs(map[string]bool{"first": false, "second": true}),
			).
			Assess("ThenFirstIsDeleting",
				composedStateIs(map[string]bool{"first": true}),
			).
			Assess("XRIsGone",
				// The XR holds its own finalizer until the last composed
				// resource is gone, so it can only disappear once teardown has
				// run to completion.
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
			).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

// logXRReferences dumps the XR's composed resource references. They are the
// graph core reads on teardown, so a reference missing resourceName or
// dependsOn means there is no graph and teardown silently does nothing.
func logXRReferences(ctx context.Context, t *testing.T, c *envconf.Config) {
	t.Helper()

	xr := &unstructured.Unstructured{}
	xr.SetAPIVersion("ordering.example.org/v1alpha1")
	xr.SetKind("XOrdering")

	if err := c.Client().Resources(orderedXRNamespace).Get(ctx, orderedXRName, orderedXRNamespace, xr); err != nil {
		t.Logf("cannot get XR to dump its references: %v", err)
		return
	}

	refs, _, _ := unstructured.NestedSlice(xr.Object, "spec", "crossplane", "resourceRefs")
	t.Logf("spec.crossplane.resourceRefs (%d):", len(refs))

	for _, r := range refs {
		t.Logf("  %v", r)
	}

	conds, _, _ := unstructured.NestedSlice(xr.Object, "status", "conditions")
	for _, cd := range conds {
		t.Logf("  condition: %v", cd)
	}
}

// xrReferencesCarryGraph asserts that the XR's composed resource references
// record the dependency graph: every reference names the composition resource
// it corresponds to, and the resources that depend on something say so.
//
// This is the precondition for ordered teardown. Core rebuilds the graph from
// these references when the XR is deleted, because no function runs then.
func xrReferencesCarryGraph() features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		xr := &unstructured.Unstructured{}
		xr.SetAPIVersion("ordering.example.org/v1alpha1")
		xr.SetKind("XOrdering")

		if err := c.Client().Resources(orderedXRNamespace).Get(ctx, orderedXRName, orderedXRNamespace, xr); err != nil {
			t.Fatalf("cannot get XR: %v", err)
		}

		refs, _, err := unstructured.NestedSlice(xr.Object, "spec", "crossplane", "resourceRefs")
		if err != nil {
			t.Fatalf("cannot read spec.crossplane.resourceRefs: %v", err)
		}

		if len(refs) == 0 {
			t.Fatalf("the XR has no composed resource references at all")
		}

		named, withDeps := 0, 0

		for _, r := range refs {
			ref, ok := r.(map[string]any)
			if !ok {
				continue
			}

			t.Logf("  ref: %v", ref)

			if n, _, _ := unstructured.NestedString(ref, "resourceName"); n != "" {
				named++
			}

			if d, _, _ := unstructured.NestedSlice(ref, "dependsOn"); len(d) > 0 {
				withDeps++
			}
		}

		if named != len(refs) {
			t.Fatalf("only %d of %d references carry resourceName; without it the graph cannot be resolved on teardown. Is Crossplane running a build whose CRD schema includes it?", named, len(refs))
		}

		// first <- second <- third: two of the three depend on something.
		if withDeps == 0 {
			t.Fatalf("no reference carries dependsOn, so there is no graph to tear down in order. Is Crossplane running with --enable-composed-resource-ordering?")
		}

		t.Logf("%d references, all named, %d carrying dependencies", len(refs), withDeps)

		return ctx
	}
}

// logCrossplaneTeardown dumps what Crossplane's own logs say about the
// teardown. The e2e captures test output and dumped resources, not pod logs,
// so without this the reconciler's account of why it did what it did is
// invisible - and the cluster is inside the test container, so it can't be
// inspected afterwards either.
func logCrossplaneTeardown(ctx context.Context, t *testing.T, c *envconf.Config) {
	t.Helper()

	cs, err := kubernetes.NewForConfig(c.Client().RESTConfig())
	if err != nil {
		t.Logf("cannot build a clientset to read Crossplane's logs: %v", err)
		return
	}

	pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "app=crossplane"})
	if err != nil || len(pods.Items) == 0 {
		t.Logf("cannot find the Crossplane pod to read its logs: %v", err)
		return
	}

	tail := int64(2000)

	rc, err := cs.CoreV1().Pods(namespace).GetLogs(pods.Items[0].GetName(), &corev1.PodLogOptions{TailLines: &tail}).Stream(ctx)
	if err != nil {
		t.Logf("cannot stream Crossplane's logs: %v", err)
		return
	}
	defer rc.Close()

	b, err := io.ReadAll(rc)
	if err != nil {
		t.Logf("cannot read Crossplane's logs: %v", err)
		return
	}

	t.Log("Crossplane's account of the teardown:")

	found := false

	for line := range strings.SplitSeq(string(b), "\n") {
		if strings.Contains(line, "teardown") || strings.Contains(line, "dependency graph") ||
			strings.Contains(line, "composed resources that are left") || strings.Contains(line, "Deleting") {
			t.Logf("  %s", line)

			found = true
		}
	}

	if !found {
		t.Log("  (nothing about teardown in the last 2000 lines - is the reconciler reaching it at all?)")
	}
}

// xrReportsBlockedTeardown waits for the XR to say it cannot make progress and
// needs a human. This is the policy: rather than abandon the order and cascade,
// Crossplane holds its finalizer and reports what it is waiting on.
func xrReportsBlockedTeardown() features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		var last string

		err := wait.For(func(context.Context) (bool, error) {
			xr := &unstructured.Unstructured{}
			xr.SetAPIVersion("ordering.example.org/v1alpha1")
			xr.SetKind("XOrdering")

			if err := c.Client().Resources(orderedXRNamespace).Get(ctx, orderedXRName, orderedXRNamespace, xr); err != nil {
				if kerrors.IsNotFound(err) {
					return false, errors.New("the XR finished deleting; a resource that cannot be deleted should have held it indefinitely")
				}

				return false, err
			}

			conds, _, _ := unstructured.NestedSlice(xr.Object, "status", "conditions")
			for _, cd := range conds {
				m, ok := cd.(map[string]any)
				if !ok {
					continue
				}

				msg, _, _ := unstructured.NestedString(m, "message")
				// Both the deadlock message and the "asked to delete and it
				// hasn't gone" message say intervention may be needed; either
				// is a correct report of a teardown that cannot proceed.
				if strings.Contains(msg, "manual intervention") {
					last = msg
					return true, nil
				}

				if msg != "" {
					last = msg
				}
			}

			return false, nil
		}, wait.WithTimeout(90*time.Second), wait.WithInterval(2*time.Second))
		if err != nil {
			t.Fatalf("the XR never reported a blocked teardown: %v (last message: %q)", err, last)
		}

		t.Logf("XR reports: %s", last)

		return ctx
	}
}

// unblockComposedDeletion clears spec.forProvider.deleteError on the named
// composed resource, which is what a human would do to release a teardown that
// cannot proceed. The function pipeline doesn't run during teardown, so nothing
// puts the field back.
func unblockComposedDeletion(name string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		xr := &unstructured.Unstructured{}
		xr.SetAPIVersion("ordering.example.org/v1alpha1")
		xr.SetKind("XOrdering")

		if err := c.Client().Resources(orderedXRNamespace).Get(ctx, orderedXRName, orderedXRNamespace, xr); err != nil {
			t.Fatalf("cannot get XR: %v", err)
		}

		refs, _, _ := unstructured.NestedSlice(xr.Object, "spec", "crossplane", "resourceRefs")

		for _, r := range refs {
			ref, ok := r.(map[string]any)
			if !ok {
				continue
			}

			if n, _, _ := unstructured.NestedString(ref, "resourceName"); n != name {
				continue
			}

			objName, _, _ := unstructured.NestedString(ref, "name")

			// The provider is writing to this resource the whole time it's
			// refusing to delete, so a read-modify-write races it and loses
			// often enough to fail the test on a busy cluster. Re-read and
			// retry on conflict, rather than reporting a lost race as the
			// feature being broken.
			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				cd := &unstructured.Unstructured{}
				cd.SetAPIVersion(ref["apiVersion"].(string))
				cd.SetKind(ref["kind"].(string))

				if err := c.Client().Resources(orderedXRNamespace).Get(ctx, objName, orderedXRNamespace, cd); err != nil {
					return err
				}

				unstructured.RemoveNestedField(cd.Object, "spec", "forProvider", "deleteError")

				return c.Client().Resources(orderedXRNamespace).Update(ctx, cd)
			}); err != nil {
				t.Fatalf("cannot clear deleteError on %q: %v", name, err)
			}

			t.Logf("cleared deleteError on %s", name)

			return ctx
		}

		t.Fatalf("no composed resource named %q in the XR's references", name)

		return ctx
	}
}

func TestComposedResourceOrderingTeardownBlocks(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/ordering"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that a composed resource which cannot be deleted blocks XR teardown indefinitely rather than being abandoned in favor of an unordered cascade, that the XR reports it needs intervention, and that clearing the cause lets teardown finish.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteComposedResourceOrdering).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("UseTheBlockedComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "teardown/composition-blocked.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "teardown/composition-blocked.yaml"),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Available()),
			).
			Assess("DeleteXR",
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr.yaml", metav1.DeletePropagationBackground),
			).
			// The leaf refuses to delete, so nothing can proceed. The XR must
			// hold its finalizer and say so, rather than cascading.
			Assess("XRReportsItNeedsIntervention", xrReportsBlockedTeardown()).
			Assess("TheLeafIsStillDeleting",
				composedStateIs(map[string]bool{"first": false, "second": false, "third": true}),
			).
			Assess("ClearTheCause", unblockComposedDeletion("third")).
			// Freed, teardown picks up where it left off rather than needing
			// the XR to be reconciled from scratch.
			Assess("TeardownResumes",
				composedStateIs(map[string]bool{"first": false, "second": true}),
			).
			Assess("XRIsGone",
				funcs.ResourcesDeletedWithin(1*time.Minute, manifests, "xr.yaml"),
			).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

func TestComposedResourceOrderingCreateBeforeDestroy(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/ordering"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests the create-before-destroy lifecycle: a replacement is created without waiting for its predecessor to be deleted, and the predecessor is only deleted once the replacement is ready. The two existing at once is what a symmetric edge would never allow.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteComposedResourceOrdering).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("ComposeOnlyTheOldResource", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "cbd/composition-before.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "cbd/composition-before.yaml"),
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Available()),
			).
			Assess("OnlyOldExists",
				composedStateIs(map[string]bool{"old": false}),
			).
			Assess("SwitchToTheReplacingComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "cbd/composition-after.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "cbd/composition-after.yaml"),
			)).
			// The assertion the lifecycle value exists for. A symmetric edge
			// would make new wait for old to be deleted, so the two would
			// never be observable together.
			Assess("BothExistWhileTheReplacementBecomesReady",
				composedStateIs(map[string]bool{"old": false, "new": false}),
			).
			// Once new is ready, old is allowed to go.
			Assess("OldIsRemovedOnceTheReplacementIsReady",
				composedStateIs(map[string]bool{"new": false}),
			).
			WithTeardown("DeleteXR", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr.yaml", metav1.DeletePropagationBackground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

// exists turns a list of composition resource names into the shape
// composedStateIs wants: present, and not being deleted.
func exists(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = false
	}

	return out
}

func TestComposedResourceOrderingCreatesInWaves(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/ordering"

	// The graph in create/composition-nested.yaml, one dependency level per
	// line. Each level may only be created once the one above it is ready, so
	// asserting the levels in turn is asserting the ordering: composition
	// without a graph would create all ten at once and fail at the first.
	// slices.Concat rather than append: appending to a shared backing array
	// would have each level quietly overwrite the last whenever capacity
	// allowed it.
	var (
		level1 = []string{"standalone", "vpc"}
		level2 = slices.Concat(level1, []string{"gateway", "sg", "subnet-a", "subnet-b"})
		level3 = slices.Concat(level2, []string{"database", "instance"})
		level4 = slices.Concat(level3, []string{"app", "backup"})
	)

	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that composed resources are created one dependency level at a time over a graph four levels deep, including a diamond, fan-out from a single root, two independent branches, and a resource with no dependencies at all.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, SuiteComposedResourceOrdering).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("UseTheNestedGraph", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "create/composition-nested.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "create/composition-nested.yaml"),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			// The roots: the one resource with no dependencies, and the one
			// everything else hangs off. Nothing else may exist yet.
			Assess("OnlyTheRootsExist", composedStateIs(exists(level1...))).
			// Fan-out: four resources become eligible together once the vpc is
			// ready.
			Assess("ThenTheVPCsDependents", composedStateIsWithin(3*time.Minute, exists(level2...))).
			// The diamond converges: instance needed all three of subnet-a,
			// subnet-b and sg. database needed only subnet-a, so the two
			// branches proceed in parallel.
			Assess("ThenTheDiamondAndTheParallelBranch", composedStateIsWithin(3*time.Minute, exists(level3...))).
			Assess("ThenTheLeaves", composedStateIsWithin(3*time.Minute, exists(level4...))).
			// Three minutes, where the other tests allow one. The assessment
			// above only waits for the leaves to exist; they have still to
			// become ready, and by this depth the circuit breaker has tripped,
			// so that last wave alone takes about a minute.
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "xr.yaml", xpv2.Available()),
			).
			WithTeardown("DeleteXR", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr.yaml", metav1.DeletePropagationBackground),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}

// restartCrossplane deletes the Crossplane pod and waits for its replacement
// to be running.
//
// The point is what the replacement does not have: it has never run the
// function pipeline for the XR being torn down, so it cannot have a dependency
// graph in memory. If teardown carries on in order afterwards, the graph it is
// using came from the XR's own references and nowhere else - which is the
// whole reason for persisting it.
func restartCrossplane() features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Helper()

		cs, err := kubernetes.NewForConfig(c.Client().RESTConfig())
		if err != nil {
			t.Fatalf("cannot build a clientset: %v", err)
		}

		pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "app=crossplane"})
		if err != nil || len(pods.Items) == 0 {
			t.Fatalf("cannot find the Crossplane pod: %v", err)
		}

		old := pods.Items[0].GetName()
		if err := cs.CoreV1().Pods(namespace).Delete(ctx, old, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("cannot delete the Crossplane pod: %v", err)
		}

		t.Logf("deleted Crossplane pod %s", old)

		// Keep polling through transient list failures, but remember the
		// last one: a persistent error would otherwise be indistinguishable
		// from the pod simply not being ready yet.
		var lastErr error

		err = wait.For(func(context.Context) (bool, error) {
			pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "app=crossplane"})
			if err != nil {
				lastErr = err
				return false, nil //nolint:nilerr // Retry; lastErr is reported if we time out.
			}

			lastErr = nil

			for _, p := range pods.Items {
				if p.GetName() == old || p.Status.Phase != corev1.PodRunning {
					continue
				}

				for _, cs := range p.Status.ContainerStatuses {
					if !cs.Ready {
						return false, nil
					}
				}

				t.Logf("Crossplane came back as %s", p.GetName())

				return true, nil
			}

			return false, nil
		}, wait.WithTimeout(3*time.Minute), wait.WithInterval(2*time.Second))
		if err != nil {
			t.Fatalf("Crossplane did not come back: %v (last list error: %v)", err, lastErr)
		}

		return ctx
	}
}

func TestComposedResourceOrderingTeardownSurvivesRestart(t *testing.T) {
	manifests := "test/e2e/manifests/apiextensions/composition/ordering"
	environment.Test(t,
		features.NewWithDescription(t.Name(), "Tests that XR teardown continues in dependency order across a Crossplane restart. The replacement process never ran the pipeline for this XR, so the graph it orders by can only have come from the XR's own composed resource references.").
			WithLabel(LabelArea, LabelAreaAPIExtensions).
			WithLabel(LabelSize, LabelSizeLarge).
			WithLabel(config.LabelTestSuite, SuiteComposedResourceOrdering).
			WithSetup("PrerequisitesAreCreated", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/functions.yaml", pkgv1.Healthy(), pkgv1.Active()),
				funcs.ResourcesHaveConditionWithin(3*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("UseTheSlowDeletingComposition", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "teardown/composition.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "teardown/composition.yaml"),
			)).
			Assess("CreateXR", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "xr.yaml"),
			)).
			Assess("XRIsReady",
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "xr.yaml", xpv2.Available()),
			).
			Assess("DeleteXR",
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "xr.yaml", metav1.DeletePropagationBackground),
			).
			Assess("TheFirstWaveStarts",
				composedStateIs(map[string]bool{"first": false, "second": false, "third": true}),
			).
			// Everything in memory goes. Only spec.crossplane.resourceRefs is
			// left to order by.
			Assess("RestartCrossplaneMidTeardown", restartCrossplane()).
			// The XR must still be here: its finalizer outlives the process
			// that set it.
			Assess("TeardownResumesFromTheXRsReferences",
				composedStateIsWithin(3*time.Minute, map[string]bool{"first": false, "second": true}),
			).
			Assess("AndRunsToCompletion",
				composedStateIsWithin(3*time.Minute, map[string]bool{"first": true}),
			).
			Assess("XRIsGone",
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "xr.yaml"),
			).
			WithTeardown("DeletePrerequisites", funcs.AllOf(
				funcs.DeleteResourcesWithPropagationPolicy(manifests, "setup/*.yaml", metav1.DeletePropagationForeground),
				funcs.ResourcesDeletedWithin(3*time.Minute, manifests, "setup/*.yaml"),
			)).
			Feature(),
	)
}
