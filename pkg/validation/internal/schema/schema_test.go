package schema

import (
	"testing"
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
