/*
Copyright 2021 The Crossplane Authors.

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

package composition

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

func TestNewCompositionRevision(t *testing.T) {
	sf := "f"
	comp := &v1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "coolcomp",
		},
		Spec: v1.CompositionSpec{
			CompositeTypeRef: v1.TypeReference{
				APIVersion: "v",
				Kind:       "k",
			},
			PatchSets: []v1.PatchSet{{
				Name: "p",
				Patches: []v1.Patch{{
					Type:          v1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1.Combine{
						Strategy: v1.CombineStrategy("s"),
						Variables: []v1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1.Transform{{
						Type: v1.TransformType("t"),
						Math: &v1.MathTransform{
							Multiply: pointer.Int64(42),
						},
						Map: &v1.MapTransform{
							Pairs: map[string]string{"k": "v"},
						},
						String: &v1.StringTransform{
							Format: &sf,
						},
						Convert: &v1.ConvertTransform{
							ToType: "t",
						},
					}},
					Policy: &v1.PatchPolicy{
						FromFieldPath: func() *v1.FromFieldPathPolicy {
							p := v1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
			}},
			Resources: []v1.ComposedTemplate{{
				Name: pointer.String("t"),
				Base: runtime.RawExtension{Raw: []byte("bytes")},
				Patches: []v1.Patch{{
					Type:          v1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1.Combine{
						Strategy: v1.CombineStrategy("s"),
						Variables: []v1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1.Transform{{
						Type: v1.TransformType("t"),
						Math: &v1.MathTransform{
							Multiply: pointer.Int64(42),
						},
						Map: &v1.MapTransform{
							Pairs: map[string]string{"k": "v"},
						},
						String: &v1.StringTransform{
							Format: &sf,
						},
						Convert: &v1.ConvertTransform{
							ToType: "t",
						},
					}},
					Policy: &v1.PatchPolicy{
						FromFieldPath: func() *v1.FromFieldPathPolicy {
							p := v1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
				ConnectionDetails: []v1.ConnectionDetail{{
					Name: pointer.String("cd"),
					Type: func() *v1.ConnectionDetailType {
						t := v1.ConnectionDetailType("t")
						return &t
					}(),
					FromConnectionSecretKey: pointer.String("k"),
					FromFieldPath:           pointer.String("p"),
					Value:                   pointer.String("v"),
				}},
				ReadinessChecks: []v1.ReadinessCheck{{
					Type:         v1.ReadinessCheckType("c"),
					FieldPath:    "p",
					MatchString:  "s",
					MatchInteger: 42,
				}},
			}},
		},
	}

	var (
		rev  int64  = 1
		hash string = "hash"
	)

	ctrl := true
	want := &v1alpha1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: comp.GetName() + "-",
			Labels: map[string]string{
				v1alpha1.LabelCompositionName:     comp.GetName(),
				v1alpha1.LabelCompositionSpecHash: hash,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.CompositionKind,
				Name:       comp.GetName(),
				Controller: &ctrl,
			}},
		},
		Spec: v1alpha1.CompositionRevisionSpec{
			Revision: rev,
			CompositeTypeRef: v1alpha1.TypeReference{
				APIVersion: "v",
				Kind:       "k",
			},
			PatchSets: []v1alpha1.PatchSet{{
				Name: "p",
				Patches: []v1alpha1.Patch{{
					Type:          v1alpha1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1alpha1.Combine{
						Strategy: v1alpha1.CombineStrategy("s"),
						Variables: []v1alpha1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1alpha1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1alpha1.Transform{{
						Type: v1alpha1.TransformType("t"),
						Math: &v1alpha1.MathTransform{
							Multiply: pointer.Int64(42),
						},
						Map: &v1alpha1.MapTransform{
							Pairs: map[string]string{"k": "v"},
						},
						String: &v1alpha1.StringTransform{
							Format: "f",
						},
						Convert: &v1alpha1.ConvertTransform{
							ToType: "t",
						},
					}},
					Policy: &v1alpha1.PatchPolicy{
						FromFieldPath: func() *v1alpha1.FromFieldPathPolicy {
							p := v1alpha1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
			}},
			Resources: []v1alpha1.ComposedTemplate{{
				Name: pointer.String("t"),
				Base: runtime.RawExtension{Raw: []byte("bytes")},
				Patches: []v1alpha1.Patch{{
					Type:          v1alpha1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1alpha1.Combine{
						Strategy: v1alpha1.CombineStrategy("s"),
						Variables: []v1alpha1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1alpha1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1alpha1.Transform{{
						Type: v1alpha1.TransformType("t"),
						Math: &v1alpha1.MathTransform{
							Multiply: pointer.Int64(42),
						},
						Map: &v1alpha1.MapTransform{
							Pairs: map[string]string{"k": "v"},
						},
						String: &v1alpha1.StringTransform{
							Format: "f",
						},
						Convert: &v1alpha1.ConvertTransform{
							ToType: "t",
						},
					}},
					Policy: &v1alpha1.PatchPolicy{
						FromFieldPath: func() *v1alpha1.FromFieldPathPolicy {
							p := v1alpha1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
				ConnectionDetails: []v1alpha1.ConnectionDetail{{
					Name: pointer.String("cd"),
					Type: func() *v1alpha1.ConnectionDetailType {
						t := v1alpha1.ConnectionDetailType("t")
						return &t
					}(),
					FromConnectionSecretKey: pointer.String("k"),
					FromFieldPath:           pointer.String("p"),
					Value:                   pointer.String("v"),
				}},
				ReadinessChecks: []v1alpha1.ReadinessCheck{{
					Type:         v1alpha1.ReadinessCheckType("c"),
					FieldPath:    "p",
					MatchString:  "s",
					MatchInteger: 42,
				}},
			}},
		},
		Status: v1alpha1.CompositionRevisionStatus{
			ConditionedStatus: xpv1.ConditionedStatus{
				Conditions: []xpv1.Condition{
					v1alpha1.CompositionSpecMatches(),
				},
			},
		},
	}

	got := NewCompositionRevision(comp, rev, hash)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NewCompositionRevision(): -want, +got:\n%s", diff)
	}
}
