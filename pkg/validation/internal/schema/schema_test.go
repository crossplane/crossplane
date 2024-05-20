package schema

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestIsKnownJSONType(t *testing.T) {
	type args struct {
		t string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Known",
			args: args{t: "string"},
			want: true,
		},
		{
			name: "Unknown",
			args: args{t: "foo"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.args.t); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKnownJSONTypeIsEquivalent(t *testing.T) {
	tests := []struct {
		name string
		t    KnownJSONType
		t2   KnownJSONType
		want bool
	}{
		{
			name: "Equivalent if same type",
			t:    KnownJSONTypeString,
			t2:   KnownJSONTypeString,
			want: true,
		},
		{
			name: "Not equivalent if different type",
			t:    KnownJSONTypeString,
			t2:   KnownJSONTypeInteger,
			want: false,
		},
		{
			name: "Not equivalent if one is unknown",
			t:    KnownJSONTypeString,
			t2:   "",
			want: false,
		},
		{
			// should never happen as these would not be valid values
			name: "Equivalent if both are unknown",
			t:    "",
			t2:   "",
			want: true,
		},
		{
			name: "Integers are equivalent to numbers",
			t:    KnownJSONTypeInteger,
			t2:   KnownJSONTypeNumber,
			want: true,
		},
		{
			name: "Numbers are not equivalent to integers",
			t:    KnownJSONTypeNumber,
			t2:   KnownJSONTypeInteger,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.IsEquivalent(tt.t2); got != tt.want {
				t.Errorf("IsEquivalent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertTransformTypeToKnownJSONType(t *testing.T) {
	type args struct {
		c v1.TransformIOType
	}
	type want struct {
		t KnownJSONType
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Int": {
			reason: "Int",
			args: args{
				c: v1.TransformIOTypeInt,
			},
			want: want{
				t: KnownJSONTypeInteger,
			},
		},
		"Int64": {
			reason: "Int64",
			args: args{
				c: v1.TransformIOTypeInt64,
			},
			want: want{
				t: KnownJSONTypeInteger,
			},
		},
		"Float64": {
			reason: "Float64",
			args: args{
				c: v1.TransformIOTypeFloat64,
			},
			want: want{
				t: KnownJSONTypeNumber,
			},
		},
		"Unknown": {
			reason: "Unknown returns empty string, should never happen",
			args: args{
				c: v1.TransformIOType("foo"),
			},
			want: want{
				t: "",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := FromTransformIOType(tc.args.c)
			if diff := cmp.Diff(tc.want.t, got); diff != "" {
				t.Errorf("\n%s\nToKnownJSONType(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if tc.want.t == "" && tc.args.c.IsValid() {
				t.Errorf("IsValid() should return false for unknown type: %s", tc.args.c)
			}
		})
	}
}

func TestFromKnownJSONType(t *testing.T) {
	type args struct {
		t KnownJSONType
	}
	type want struct {
		out v1.TransformIOType
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidInt": {
			reason: "Int should be valid and convert properly",
			args: args{
				t: KnownJSONTypeInteger,
			},
			want: want{
				out: v1.TransformIOTypeInt64,
			},
		},
		"ValidNumber": {
			reason: "Number should be valid and convert properly",
			args: args{
				t: KnownJSONTypeNumber,
			},
			want: want{
				out: v1.TransformIOTypeFloat64,
			},
		},
		"InvalidUnknown": {
			reason: "Unknown return an error",
			args: args{
				t: KnownJSONType("foo"),
			},
			want: want{
				err: xperrors.Errorf(errFmtUnknownJSONType, "foo"),
			},
		},
		"InvalidEmpty": {
			reason: "Empty string return an error",
			args: args{
				t: "",
			},
			want: want{
				err: xperrors.Errorf(errFmtUnknownJSONType, ""),
			},
		},
		"InvalidNull": {
			reason: "Null return an error",
			args: args{
				t: KnownJSONTypeNull,
			},
			want: want{
				err: xperrors.Errorf(errFmtUnsupportedJSONType, KnownJSONTypeNull),
			},
		},
		"ValidBoolean": {
			reason: "Boolean should be valid and convert properly",
			args: args{
				t: KnownJSONTypeBoolean,
			},
			want: want{
				out: v1.TransformIOTypeBool,
			},
		},
		"ValidArray": {
			reason: "Array should be valid and convert properly",
			args:   args{t: KnownJSONTypeArray},
			want:   want{out: v1.TransformIOTypeObject},
		},
		"ValidObject": {
			reason: "Object should be valid and convert properly",
			args:   args{t: KnownJSONTypeObject},
			want:   want{out: v1.TransformIOTypeObject},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := FromKnownJSONType(tc.args.t)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFromKnownJSONType(...): -want error, +got error:\n%s", tc.reason, diff)
				return
			}
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("\n%s\nFromKnownJSONType(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
