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

package xcrd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
)

func TestCompareFieldRemoval(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoFieldsRemoved": {
			reason: "No changes should be detected when no fields are removed",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec")},
				existing: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
						"field2": {Type: "integer"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
						"field2": {Type: "integer"},
					},
				},
			},
			want: want{
				changes: nil,
			},
		},
		"FieldRemoved": {
			reason: "A breaking change should be detected when a field is removed",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec")},
				existing: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
						"field2": {Type: "integer"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field2")},
						Type:    ChangeTypeFieldRemoved,
						Message: "field removed (existing resources with this field cannot be read or updated)",
					},
				},
			},
		},
		"MultipleFieldsRemoved": {
			reason: "Multiple breaking changes should be detected when multiple fields are removed",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec")},
				existing: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
						"field2": {Type: "integer"},
						"field3": {Type: "boolean"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field2")},
						Type:    ChangeTypeFieldRemoved,
						Message: "field removed (existing resources with this field cannot be read or updated)",
					},
					{
						Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field3")},
						Type:    ChangeTypeFieldRemoved,
						Message: "field removed (existing resources with this field cannot be read or updated)",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareFieldRemoval(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got, cmpopts.SortSlices(func(a, b SchemaChange) bool {
				return a.Path.String() < b.Path.String()
			})); diff != "" {
				t.Errorf("%s\nCompareFieldRemoval(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareTypeChange(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoTypeChange": {
			reason: "No changes should be detected when type is unchanged",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field1")},
				existing: &extv1.JSONSchemaProps{Type: "string"},
				proposed: &extv1.JSONSchemaProps{Type: "string"},
			},
			want: want{
				changes: nil,
			},
		},
		"TypeChanged": {
			reason: "A breaking change should be detected when type changes",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field1")},
				existing: &extv1.JSONSchemaProps{Type: "string"},
				proposed: &extv1.JSONSchemaProps{Type: "integer"},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field1")},
						Type:     ChangeTypeTypeChanged,
						Message:  "type changed from string to integer (existing resources will fail to deserialize)",
						OldValue: "string",
						NewValue: "integer",
					},
				},
			},
		},
		"EmptyTypeIgnored": {
			reason: "Empty types should be ignored",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field1")},
				existing: &extv1.JSONSchemaProps{Type: ""},
				proposed: &extv1.JSONSchemaProps{Type: "string"},
			},
			want: want{
				changes: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareTypeChange(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nCompareTypeChange(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareEnumValues(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoEnumChanges": {
			reason: "No changes should be detected when enum values are unchanged",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("status")},
				existing: &extv1.JSONSchemaProps{
					Enum: []extv1.JSON{
						{Raw: []byte(`"active"`)},
						{Raw: []byte(`"inactive"`)},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Enum: []extv1.JSON{
						{Raw: []byte(`"active"`)},
						{Raw: []byte(`"inactive"`)},
					},
				},
			},
			want: want{
				changes: nil,
			},
		},
		"EnumValueRemoved": {
			reason: "A breaking change should be detected when an enum value is removed",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("status")},
				existing: &extv1.JSONSchemaProps{
					Enum: []extv1.JSON{
						{Raw: []byte(`"active"`)},
						{Raw: []byte(`"inactive"`)},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Enum: []extv1.JSON{
						{Raw: []byte(`"active"`)},
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("status")},
						Type:     ChangeTypeEnumRestricted,
						Message:  `enum value removed: "inactive" (resources using this value will fail validation)`,
						OldValue: `"inactive"`,
					},
				},
			},
		},
		"EnumValueAdded": {
			reason: "A non-breaking change should be detected when an enum value is added",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("status")},
				existing: &extv1.JSONSchemaProps{
					Enum: []extv1.JSON{
						{Raw: []byte(`"active"`)},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Enum: []extv1.JSON{
						{Raw: []byte(`"active"`)},
						{Raw: []byte(`"pending"`)},
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("status")},
						Type:     ChangeTypeEnumExpanded,
						Message:  `enum value added: "pending"`,
						NewValue: `"pending"`,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareEnumValues(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nCompareEnumValues(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareRequiredFields(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoRequiredChanges": {
			reason: "No changes should be detected when required fields are unchanged",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec")},
				existing: &extv1.JSONSchemaProps{
					Required: []string{"field1"},
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Required: []string{"field1"},
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
			},
			want: want{
				changes: nil,
			},
		},
		"RequiredFieldAddedWithoutDefault": {
			reason: "A breaking change should be detected when a field becomes required without a default",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec")},
				existing: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Required: []string{"field1"},
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec")},
						Type:     ChangeTypeRequiredAdded,
						Message:  "field field1 is now required without default (existing resources without this field will fail validation)",
						NewValue: "field1",
					},
				},
			},
		},
		"RequiredFieldAddedWithDefault": {
			reason: "No breaking change should be detected when a field becomes required with a default",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec")},
				existing: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Required: []string{"field1"},
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {
							Type:    "string",
							Default: &extv1.JSON{Raw: []byte(`"default"`)},
						},
					},
				},
			},
			want: want{
				changes: nil,
			},
		},
		"RequiredFieldRemoved": {
			reason: "A non-breaking change should be detected when a field is no longer required",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec")},
				existing: &extv1.JSONSchemaProps{
					Required: []string{"field1"},
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec")},
						Type:     ChangeTypeRequiredRemoved,
						Message:  "field field1 is no longer required",
						OldValue: "field1",
					},
				},
			},
		},
		"NewFieldRequiredIgnored": {
			reason: "Required on a newly added field should not be flagged as breaking",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec")},
				existing: &extv1.JSONSchemaProps{
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					Required: []string{"field2"},
					Properties: map[string]extv1.JSONSchemaProps{
						"field1": {Type: "string"},
						"field2": {Type: "string"},
					},
				},
			},
			want: want{
				changes: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareRequiredFields(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nCompareRequiredFields(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestComparePatternChange(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoPatternChange": {
			reason: "No changes should be detected when pattern is unchanged",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
				existing: &extv1.JSONSchemaProps{Pattern: "^[a-z]+$"},
				proposed: &extv1.JSONSchemaProps{Pattern: "^[a-z]+$"},
			},
			want: want{
				changes: nil,
			},
		},
		"PatternChanged": {
			reason: "A breaking change should be detected when pattern changes",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
				existing: &extv1.JSONSchemaProps{Pattern: "^[a-z]+$"},
				proposed: &extv1.JSONSchemaProps{Pattern: "^[a-z0-9]+$"},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
						Type:     ChangeTypePatternChanged,
						Message:  "regex pattern changed (values matching old pattern may no longer be valid)",
						OldValue: "^[a-z]+$",
						NewValue: "^[a-z0-9]+$",
					},
				},
			},
		},
		"EmptyPatternIgnored": {
			reason: "Empty patterns should be ignored",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
				existing: &extv1.JSONSchemaProps{Pattern: ""},
				proposed: &extv1.JSONSchemaProps{Pattern: "^[a-z]+$"},
			},
			want: want{
				changes: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ComparePatternChange(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nComparePatternChange(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareNumericConstraints(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MaximumReduced": {
			reason: "A breaking change should be detected when maximum is reduced",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{Maximum: ptr.To(100.0)},
				proposed: &extv1.JSONSchemaProps{Maximum: ptr.To(50.0)},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "maximum reduced from 100 to 50 (values above new maximum will fail validation)",
						OldValue: "100",
						NewValue: "50",
					},
				},
			},
		},
		"MinimumIncreased": {
			reason: "A breaking change should be detected when minimum is increased",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{Minimum: ptr.To(0.0)},
				proposed: &extv1.JSONSchemaProps{Minimum: ptr.To(10.0)},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "minimum increased from 0 to 10 (values below new minimum will fail validation)",
						OldValue: "0",
						NewValue: "10",
					},
				},
			},
		},
		"ExclusiveMaximumAdded": {
			reason: "A breaking change should be detected when maximum becomes exclusive",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{ExclusiveMaximum: false},
				proposed: &extv1.JSONSchemaProps{ExclusiveMaximum: true},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:    ChangeTypeConstraintTightened,
						Message: "maximum changed to exclusive (values equal to maximum will fail validation)",
					},
				},
			},
		},
		"ExclusiveMinimumAdded": {
			reason: "A breaking change should be detected when minimum becomes exclusive",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{ExclusiveMinimum: false},
				proposed: &extv1.JSONSchemaProps{ExclusiveMinimum: true},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:    ChangeTypeConstraintTightened,
						Message: "minimum changed to exclusive (values equal to minimum will fail validation)",
					},
				},
			},
		},
		"MultipleOfAdded": {
			reason: "A breaking change should be detected when multipleOf is added",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{},
				proposed: &extv1.JSONSchemaProps{MultipleOf: ptr.To(5.0)},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "multipleOf constraint added: 5 (values not divisible by 5 will fail validation)",
						NewValue: "5",
					},
				},
			},
		},
		"MultipleOfChanged": {
			reason: "A breaking change should be detected when multipleOf changes",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{MultipleOf: ptr.To(5.0)},
				proposed: &extv1.JSONSchemaProps{MultipleOf: ptr.To(10.0)},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "multipleOf changed from 5 to 10 (values valid under old constraint may fail)",
						OldValue: "5",
						NewValue: "10",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareNumericConstraints(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nCompareNumericConstraints(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareStringConstraints(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MaxLengthReduced": {
			reason: "A breaking change should be detected when maxLength is reduced",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
				existing: &extv1.JSONSchemaProps{MaxLength: ptr.To(int64(100))},
				proposed: &extv1.JSONSchemaProps{MaxLength: ptr.To(int64(50))},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "maxLength reduced from 100 to 50 (strings longer than 50 will fail validation)",
						OldValue: "100",
						NewValue: "50",
					},
				},
			},
		},
		"MinLengthIncreased": {
			reason: "A breaking change should be detected when minLength is increased",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
				existing: &extv1.JSONSchemaProps{MinLength: ptr.To(int64(0))},
				proposed: &extv1.JSONSchemaProps{MinLength: ptr.To(int64(5))},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "minLength increased from 0 to 5 (strings shorter than 5 will fail validation)",
						OldValue: "0",
						NewValue: "5",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareStringConstraints(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nCompareStringConstraints(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareArrayConstraints(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MaxItemsReduced": {
			reason: "A breaking change should be detected when maxItems is reduced",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("tags")},
				existing: &extv1.JSONSchemaProps{MaxItems: ptr.To(int64(10))},
				proposed: &extv1.JSONSchemaProps{MaxItems: ptr.To(int64(5))},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("tags")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "maxItems reduced from 10 to 5 (arrays with more than 5 items will fail validation)",
						OldValue: "10",
						NewValue: "5",
					},
				},
			},
		},
		"MinItemsIncreased": {
			reason: "A breaking change should be detected when minItems is increased",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("tags")},
				existing: &extv1.JSONSchemaProps{MinItems: ptr.To(int64(0))},
				proposed: &extv1.JSONSchemaProps{MinItems: ptr.To(int64(2))},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("tags")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "minItems increased from 0 to 2 (arrays with fewer than 2 items will fail validation)",
						OldValue: "0",
						NewValue: "2",
					},
				},
			},
		},
		"UniqueItemsAdded": {
			reason: "A breaking change should be detected when uniqueItems is added",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("tags")},
				existing: &extv1.JSONSchemaProps{UniqueItems: false},
				proposed: &extv1.JSONSchemaProps{UniqueItems: true},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("tags")},
						Type:    ChangeTypeConstraintTightened,
						Message: "uniqueItems constraint added (arrays with duplicate items will fail validation)",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareArrayConstraints(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nCompareArrayConstraints(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareObjectConstraints(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MaxPropertiesReduced": {
			reason: "A breaking change should be detected when maxProperties is reduced",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("metadata")},
				existing: &extv1.JSONSchemaProps{MaxProperties: ptr.To(int64(10))},
				proposed: &extv1.JSONSchemaProps{MaxProperties: ptr.To(int64(5))},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("metadata")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "maxProperties reduced from 10 to 5 (objects with more than 5 properties will fail validation)",
						OldValue: "10",
						NewValue: "5",
					},
				},
			},
		},
		"MinPropertiesIncreased": {
			reason: "A breaking change should be detected when minProperties is increased",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("metadata")},
				existing: &extv1.JSONSchemaProps{MinProperties: ptr.To(int64(0))},
				proposed: &extv1.JSONSchemaProps{MinProperties: ptr.To(int64(2))},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("metadata")},
						Type:     ChangeTypeConstraintTightened,
						Message:  "minProperties increased from 0 to 2 (objects with fewer than 2 properties will fail validation)",
						OldValue: "0",
						NewValue: "2",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareObjectConstraints(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nCompareObjectConstraints(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareCELValidations(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoCELChanges": {
			reason: "No changes should be detected when CEL validations are unchanged",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{
					XValidations: []extv1.ValidationRule{
						{Rule: "self > 0"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					XValidations: []extv1.ValidationRule{
						{Rule: "self > 0"},
					},
				},
			},
			want: want{
				changes: nil,
			},
		},
		"CELValidationAdded": {
			reason: "A breaking change should be detected when a CEL validation is added",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{},
				proposed: &extv1.JSONSchemaProps{
					XValidations: []extv1.ValidationRule{
						{Rule: "self > 0"},
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:     ChangeTypeCELValidationAdded,
						Message:  "CEL validation rule added or modified: self > 0 (existing resources may fail new validation)",
						NewValue: "self > 0",
					},
				},
			},
		},
		"CELValidationRemoved": {
			reason: "A non-breaking change should be detected when a CEL validation is removed",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{
					XValidations: []extv1.ValidationRule{
						{Rule: "self > 0"},
					},
				},
				proposed: &extv1.JSONSchemaProps{},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:     ChangeTypeCELValidationRemoved,
						Message:  "CEL validation rule removed: self > 0",
						OldValue: "self > 0",
					},
				},
			},
		},
		"CELValidationModified": {
			reason: "A breaking change should be detected when a CEL validation is modified",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
				existing: &extv1.JSONSchemaProps{
					XValidations: []extv1.ValidationRule{
						{Rule: "self > 0"},
					},
				},
				proposed: &extv1.JSONSchemaProps{
					XValidations: []extv1.ValidationRule{
						{Rule: "self > 10"},
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:     ChangeTypeCELValidationAdded,
						Message:  "CEL validation rule added or modified: self > 10 (existing resources may fail new validation)",
						NewValue: "self > 10",
					},
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("count")},
						Type:     ChangeTypeCELValidationRemoved,
						Message:  "CEL validation rule removed: self > 0",
						OldValue: "self > 0",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareCELValidations(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got, cmpopts.SortSlices(func(a, b SchemaChange) bool {
				return a.Type < b.Type
			})); diff != "" {
				t.Errorf("%s\nCompareCELValidations(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareKubernetesExtensions(t *testing.T) {
	type args struct {
		path     fieldpath.Segments
		existing *extv1.JSONSchemaProps
		proposed *extv1.JSONSchemaProps
	}

	type want struct {
		changes []SchemaChange
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"PreserveUnknownFieldsDisabled": {
			reason: "A breaking change should be detected when x-kubernetes-preserve-unknown-fields changes from true to false",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("metadata")},
				existing: &extv1.JSONSchemaProps{XPreserveUnknownFields: ptr.To(true)},
				proposed: &extv1.JSONSchemaProps{XPreserveUnknownFields: ptr.To(false)},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("metadata")},
						Type:     ChangeTypePreserveUnknownChanged,
						Message:  "x-kubernetes-preserve-unknown-fields changed from true to false (unknown fields will be pruned)",
						OldValue: "true",
						NewValue: "false",
					},
				},
			},
		},
		"ListTypeChanged": {
			reason: "A breaking change should be detected when x-kubernetes-list-type changes",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("items")},
				existing: &extv1.JSONSchemaProps{XListType: ptr.To("atomic")},
				proposed: &extv1.JSONSchemaProps{XListType: ptr.To("map")},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("items")},
						Type:     ChangeTypeListTypeChanged,
						Message:  "x-kubernetes-list-type changed from atomic to map (merge behavior will change)",
						OldValue: "atomic",
						NewValue: "map",
					},
				},
			},
		},
		"MapTypeChanged": {
			reason: "A breaking change should be detected when x-kubernetes-map-type changes",
			args: args{
				path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("labels")},
				existing: &extv1.JSONSchemaProps{XMapType: ptr.To("atomic")},
				proposed: &extv1.JSONSchemaProps{XMapType: ptr.To("granular")},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("labels")},
						Type:     ChangeTypeMapTypeChanged,
						Message:  "x-kubernetes-map-type changed from atomic to granular (merge behavior will change)",
						OldValue: "atomic",
						NewValue: "granular",
					},
				},
			},
		},
		"ListMapKeysChanged": {
			reason: "A breaking change should be detected when x-kubernetes-list-map-keys changes",
			args: args{
				path: fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("items")},
				existing: &extv1.JSONSchemaProps{
					XListMapKeys: []string{"name"},
				},
				proposed: &extv1.JSONSchemaProps{
					XListMapKeys: []string{"name", "namespace"},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:     fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("items")},
						Type:     ChangeTypeListTypeChanged,
						Message:  "x-kubernetes-list-map-keys changed (merge behavior will change)",
						OldValue: "[name]",
						NewValue: "[name namespace]",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareKubernetesExtensions(tc.args.path, tc.args.existing, tc.args.proposed)

			if diff := cmp.Diff(tc.want.changes, got); diff != "" {
				t.Errorf("%s\nCompareKubernetesExtensions(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
