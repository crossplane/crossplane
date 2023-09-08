/*
Copyright 2019 The Crossplane Authors.

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

package fieldpath

import (
	"math"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestSegments(t *testing.T) {
	cases := map[string]struct {
		s    Segments
		want string
	}{
		"SingleField": {
			s:    Segments{Field("spec")},
			want: "spec",
		},
		"SingleIndex": {
			s:    Segments{FieldOrIndex("0")},
			want: "[0]",
		},
		"FieldsAndIndex": {
			s: Segments{
				Field("spec"),
				Field("containers"),
				FieldOrIndex("0"),
				Field("name"),
			},
			want: "spec.containers[0].name",
		},
		"PeriodsInField": {
			s: Segments{
				Field("data"),
				Field(".config.yml"),
			},
			want: "data[.config.yml]",
		},
		"Wildcard": {
			s: Segments{
				Field("spec"),
				Field("containers"),
				FieldOrIndex("*"),
				Field("name"),
			},
			want: "spec.containers[*].name",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, tc.s.String()); diff != "" {
				t.Errorf("s.String(): -want, +got:\n %s", diff)
			}

		})
	}
}

func TestFieldOrIndex(t *testing.T) {
	cases := map[string]struct {
		reason string
		s      string
		want   Segment
	}{
		"Field": {
			reason: "An unambiguous string should be interpreted as a field segment",
			s:      "coolField",
			want:   Segment{Type: SegmentField, Field: "coolField"},
		},
		"QuotedField": {
			reason: "A quoted string should be interpreted as a field segment with the quotes removed",
			s:      "'coolField'",
			want:   Segment{Type: SegmentField, Field: "coolField"},
		},
		"QuotedFieldWithPeriods": {
			reason: "A quoted string with periods should be interpreted as a field segment with the quotes removed",
			s:      "'cool.Field'",
			want:   Segment{Type: SegmentField, Field: "cool.Field"},
		},
		"Index": {
			reason: "An unambiguous integer should be interpreted as an index segment",
			s:      "3",
			want:   Segment{Type: SegmentIndex, Index: 3},
		},
		"Negative": {
			reason: "A negative integer should be interpreted as an field segment",
			s:      "-3",
			want:   Segment{Type: SegmentField, Field: "-3"},
		},
		"Float": {
			reason: "A float should be interpreted as an field segment",
			s:      "3.0",
			want:   Segment{Type: SegmentField, Field: "3.0"},
		},
		"Overflow": {
			reason: "A very big integer will be interpreted as a field segment",
			s:      strconv.Itoa(math.MaxUint32 + 1),
			want:   Segment{Type: SegmentField, Field: strconv.Itoa(math.MaxUint32 + 1)},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := FieldOrIndex(tc.s)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nFieldOrIndex(...): %s: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestParse(t *testing.T) {
	type want struct {
		s   Segments
		err error
	}

	cases := map[string]struct {
		reason string
		path   string
		want   want
	}{
		"SingleField": {
			reason: "A path with no brackets or periods should be interpreted as a single field segment",
			path:   "spec",
			want: want{
				s: Segments{Field("spec")},
			},
		},
		"SingleIndex": {
			reason: "An integer surrounded by brackets should be interpreted as an index",
			path:   "[0]",
			want: want{
				s: Segments{FieldOrIndex("0")},
			},
		},
		"TwoFields": {
			reason: "A path with one period should be interpreted as two field segments",
			path:   "metadata.name",
			want: want{
				s: Segments{Field("metadata"), Field("name")},
			},
		},
		"APIConventionsExample": {
			reason: "The example given by the Kubernetes API convention should be parse correctly",
			path:   "fields[1].state.current",
			want: want{
				s: Segments{
					Field("fields"),
					FieldOrIndex("1"),
					Field("state"),
					Field("current"),
				},
			},
		},
		"SimpleIndex": {
			reason: "Indexing an object field that is an array should result in a field and an index",
			path:   "items[0]",
			want: want{
				s: Segments{Field("items"), FieldOrIndex("0")},
			},
		},
		"FieldsAndIndex": {
			reason: "A path with periods and braces should be interpreted as fields and indices",
			path:   "spec.containers[0].name",
			want: want{
				s: Segments{
					Field("spec"),
					Field("containers"),
					FieldOrIndex("0"),
					Field("name"),
				},
			},
		},
		"NestedArray": {
			reason: "A nested array should result in two consecutive index fields",
			path:   "nested[0][1].name",
			want: want{
				s: Segments{
					Field("nested"),
					FieldOrIndex("0"),
					FieldOrIndex("1"),
					Field("name"),
				},
			},
		},
		"BracketStyleField": {
			reason: "A field name can be specified using brackets rather than a period",
			path:   "spec[containers][0].name",
			want: want{
				s: Segments{
					Field("spec"),
					Field("containers"),
					FieldOrIndex("0"),
					Field("name"),
				},
			},
		},
		"BracketFieldWithPeriod": {
			reason: "A field name specified using brackets can include a period",
			path:   "data[.config.yml]",
			want: want{
				s: Segments{
					Field("data"),
					FieldOrIndex(".config.yml"),
				},
			},
		},
		"QuotedFieldWithPeriodInBracket": {
			reason: "A field name specified using quote and in bracket can include a period",
			path:   "metadata.labels['app.hash']",
			want: want{
				s: Segments{
					Field("metadata"),
					Field("labels"),
					FieldOrIndex("app.hash"),
				},
			},
		},
		"LeadingPeriod": {
			reason: "A path may not start with a period (unlike a JSON path)",
			path:   ".metadata.name",
			want: want{
				err: errors.New("unexpected '.' at position 0"),
			},
		},
		"TrailingPeriod": {
			reason: "A path may not end with a period",
			path:   "metadata.name.",
			want: want{
				err: errors.New("unexpected '.' at position 13"),
			},
		},
		"BracketsFollowingPeriod": {
			reason: "Brackets may not follow a period",
			path:   "spec.containers.[0].name",
			want: want{
				err: errors.New("unexpected '[' at position 16"),
			},
		},
		"DoublePeriod": {
			reason: "A path may not include two consecutive periods",
			path:   "metadata..name",
			want: want{
				err: errors.New("unexpected '.' at position 9"),
			},
		},
		"DanglingRightBracket": {
			reason: "A right bracket may not appear in a field name",
			path:   "metadata.]name",
			want: want{
				err: errors.New("unexpected ']' at position 9"),
			},
		},
		"DoubleOpenBracket": {
			reason: "Brackets may not be nested",
			path:   "spec[bracketed[name]]",
			want: want{
				err: errors.New("unexpected '[' at position 14"),
			},
		},
		"DanglingLeftBracket": {
			reason: "A left bracket must be closed",
			path:   "spec[name",
			want: want{
				err: errors.New("unterminated '[' at position 4"),
			},
		},
		"EmptyBracket": {
			reason: "Brackets may not be empty",
			path:   "spec[]",
			want: want{
				err: errors.New("unexpected ']' at position 5"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\nParse(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.s, got); diff != "" {
				t.Errorf("\nParse(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}
