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

package watchcondition

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
)

func TestCompileInvalidExpression(t *testing.T) {
	t.Parallel()

	_, err := Compile(v1alpha1.WatchSpec{
		OnChange: &v1alpha1.WatchOnChange{Expression: "object..invalid"},
	})
	if err == nil {
		t.Fatal("expected compile error")
	}
}

func TestCompileWhenMustBeBool(t *testing.T) {
	t.Parallel()

	_, err := Compile(v1alpha1.WatchSpec{
		When: []v1alpha1.WatchCondition{{
			Name:       "not-bool",
			Expression: "object.metadata.name",
		}},
	})
	if err == nil {
		t.Fatal("expected compile error for non-bool when expression")
	}
}

func TestShouldReportWithoutConditions(t *testing.T) {
	t.Parallel()

	p, err := Compile(v1alpha1.WatchSpec{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	})
	if err != nil {
		t.Fatalf("Compile(): %v", err)
	}

	obj := configMap(t, map[string]string{"key": "a"})

	report, fingerprint, err := p.ShouldReport(obj, false, "")
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if !report {
		t.Fatal("expected report=true without conditions")
	}
	if fingerprint != "" {
		t.Fatalf("expected empty fingerprint, got %q", fingerprint)
	}
	if p.HasOnChange() {
		t.Fatal("expected HasOnChange()=false")
	}
}

func TestShouldReportOnChangeFingerprint(t *testing.T) {
	t.Parallel()

	p, err := Compile(v1alpha1.WatchSpec{
		OnChange: &v1alpha1.WatchOnChange{Expression: "object.data['testData']"},
	})
	if err != nil {
		t.Fatalf("Compile(): %v", err)
	}

	obj := configMap(t, map[string]string{"testData": "one"})

	report, fp1, err := p.ShouldReport(obj, false, "")
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if !report {
		t.Fatal("expected first observation to report")
	}
	if fp1 != `"one"` {
		t.Fatalf("expected fingerprint %q, got %q", `"one"`, fp1)
	}

	report, fp2, err := p.ShouldReport(obj, false, fp1)
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if report {
		t.Fatal("expected unchanged fingerprint to skip report")
	}
	if fp2 != fp1 {
		t.Fatalf("expected fingerprint %q, got %q", fp1, fp2)
	}

	obj.Object["data"] = map[string]any{"testData": "two"}
	report, fp3, err := p.ShouldReport(obj, false, fp1)
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if !report {
		t.Fatal("expected changed fingerprint to report")
	}
	if fp3 != `"two"` {
		t.Fatalf("expected fingerprint %q, got %q", `"two"`, fp3)
	}
}

func TestShouldReportWhenGate(t *testing.T) {
	t.Parallel()

	p, err := Compile(v1alpha1.WatchSpec{
		OnChange: &v1alpha1.WatchOnChange{Expression: "object.data['testData']"},
		When: []v1alpha1.WatchCondition{{
			Name:       "enabled",
			Expression: "has(object.data) && object.data['enabled'] == 'true'",
		}},
	})
	if err != nil {
		t.Fatalf("Compile(): %v", err)
	}

	obj := configMap(t, map[string]string{"testData": "one", "enabled": "false"})

	report, _, err := p.ShouldReport(obj, false, "")
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if report {
		t.Fatal("expected when gate to block report")
	}

	obj.Object["data"] = map[string]any{"testData": "one", "enabled": "true"}
	report, fp, err := p.ShouldReport(obj, false, "")
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if !report {
		t.Fatal("expected report when gate passes")
	}
	if fp != `"one"` {
		t.Fatalf("expected fingerprint %q, got %q", `"one"`, fp)
	}
}

func TestShouldReportDeletedVariable(t *testing.T) {
	t.Parallel()

	p, err := Compile(v1alpha1.WatchSpec{
		OnChange: &v1alpha1.WatchOnChange{Expression: "deleted"},
	})
	if err != nil {
		t.Fatalf("Compile(): %v", err)
	}

	obj := &unstructured.Unstructured{Object: map[string]any{}}

	report, fp, err := p.ShouldReport(obj, true, "")
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if !report {
		t.Fatal("expected delete event to report")
	}
	if fp != "true" {
		t.Fatalf("expected fingerprint %q, got %q", "true", fp)
	}

	report, _, err = p.ShouldReport(obj, true, fp)
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if report {
		t.Fatal("expected unchanged deleted=true fingerprint to skip report")
	}

	report, fp2, err := p.ShouldReport(obj, false, fp)
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if !report {
		t.Fatal("expected deleted value change to report")
	}
	if fp2 != "false" {
		t.Fatalf("expected fingerprint %q, got %q", "false", fp2)
	}
}

func TestShouldReportDeletionMetadataOnly(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "test",
			"namespace": "default",
		},
	}}

	type args struct {
		watch               v1alpha1.WatchSpec
		previousFingerprint string
	}
	type want struct {
		report      bool
		fingerprint string
		err         error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GuardedFieldAccess": {
			reason: "Should evaluate when and onChange for a metadata-only deletion object when field access is guarded with deleted",
			args: args{
				watch: v1alpha1.WatchSpec{
					OnChange: &v1alpha1.WatchOnChange{Expression: "deleted ? '' : object.data['testData']"},
					When: []v1alpha1.WatchCondition{{
						Name:       "enabled-or-deleted",
						Expression: "deleted || object.data['enabled'] == 'true'",
					}},
				},
				previousFingerprint: `"one"`,
			},
			want: want{
				report:      true,
				fingerprint: `""`,
			},
		},
		"UnguardedWhen": {
			reason: "Should return an error when a when expression reads fields on a metadata-only deletion object",
			args: args{
				watch: v1alpha1.WatchSpec{
					OnChange: &v1alpha1.WatchOnChange{Expression: "deleted"},
					When: []v1alpha1.WatchCondition{{
						Name:       "enabled",
						Expression: "object.data['enabled'] == 'true'",
					}},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"UnguardedOnChange": {
			reason: "Should return an error when an onChange expression reads fields on a metadata-only deletion object",
			args: args{
				watch: v1alpha1.WatchSpec{
					OnChange: &v1alpha1.WatchOnChange{Expression: "object.data['testData']"},
					When: []v1alpha1.WatchCondition{{
						Name:       "is-deleted",
						Expression: "deleted",
					}},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := Compile(tc.args.watch)
			if err != nil {
				t.Fatalf("\n%s\nCompile(...): %v", tc.reason, err)
			}

			report, fingerprint, err := p.ShouldReport(obj, true, tc.args.previousFingerprint)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nShouldReport(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.report, report); diff != "" {
				t.Errorf("\n%s\nShouldReport(...): -want report, +got report:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.fingerprint, fingerprint); diff != "" {
				t.Errorf("\n%s\nShouldReport(...): -want fingerprint, +got fingerprint:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestShouldReportWhenOnly(t *testing.T) {
	t.Parallel()

	p, err := Compile(v1alpha1.WatchSpec{
		When: []v1alpha1.WatchCondition{{
			Name:       "has-data",
			Expression: "has(object.data)",
		}},
	})
	if err != nil {
		t.Fatalf("Compile(): %v", err)
	}

	obj := configMap(t, map[string]string{"key": "value"})

	report, fingerprint, err := p.ShouldReport(obj, false, "")
	if err != nil {
		t.Fatalf("ShouldReport(): %v", err)
	}
	if !report {
		t.Fatal("expected report when gate passes")
	}
	if fingerprint != "" {
		t.Fatalf("expected empty fingerprint without onChange, got %q", fingerprint)
	}
}

func configMap(t *testing.T, data map[string]string) *unstructured.Unstructured {
	t.Helper()

	d := make(map[string]string, len(data))
	maps.Copy(d, data)

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "test",
			"namespace": "default",
		},
		"data": d,
	}}
}
