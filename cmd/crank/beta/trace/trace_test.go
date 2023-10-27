package trace

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestCmd_getResourceAndName(t *testing.T) {
	type args struct {
		Resource string
		Name     string
	}
	type want struct {
		resource string
		name     string
		err      error
	}
	tests := map[string]struct {
		reason string
		fields args
		want   want
	}{
		"Splitted": {
			reason: "Should return the resource and name if both are provided",
			fields: args{
				Resource: "resource",
				Name:     "name",
			},
			want: want{
				resource: "resource",
				name:     "name",
				err:      nil,
			},
		},
		"OnlyResource": {
			reason: "Should return an error if only resource is provided",
			fields: args{
				Resource: "resource",
				Name:     "",
			},
			want: want{
				err: errors.New(errMissingName),
			},
		},
		"Empty": {
			// should never happen, resource is required by kong
			reason: "Should return an error if no resource is provided",
			fields: args{
				Resource: "",
				Name:     "",
			},
			want: want{
				err: errors.New(errInvalidResource),
			},
		},
		"Combined": {
			reason: "Should return the resource and name if both are provided combined as resource",
			fields: args{
				Resource: "resource/name",
				Name:     "",
			},
			want: want{
				resource: "resource",
				name:     "name",
			},
		},
		"MoreSlashes": {
			reason: "Should return an error if the resource contains more than one slashes",
			fields: args{
				Resource: "resource/name/other",
				Name:     "",
			},
			want: want{
				err: errors.New(errInvalidResource),
			},
		},
		"BothAndCombined": {
			reason: "Should return an error if a name is provided both in the resource and separately",
			fields: args{
				Resource: "resource/name",
				Name:     "name",
			},
			want: want{
				err: errors.New(errNameDoubled),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &Cmd{
				Resource: tt.fields.Resource,
				Name:     tt.fields.Name,
			}
			gotResource, gotName, err := c.getResourceAndName()
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Cmd.getResourceAndName() error = %v, wantErr %v", err, tt.want.err)
			}
			if diff := cmp.Diff(tt.want.resource, gotResource); diff != "" {
				t.Errorf("Cmd.getResourceAndName() resource = %v, want %v", gotResource, tt.want.resource)
			}
			if diff := cmp.Diff(tt.want.name, gotName); diff != "" {
				t.Errorf("Cmd.getResourceAndName() name = %v, want %v", gotName, tt.want.name)
			}
		})
	}
}
