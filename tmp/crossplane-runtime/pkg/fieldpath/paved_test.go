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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestIsNotFound(t *testing.T) {
	cases := map[string]struct {
		reason string
		err    error
		want   bool
	}{
		"NotFound": {
			reason: "An error with method `IsNotFound() bool` should be considered a not found error.",
			err:    notFoundError{errors.New("boom")},
			want:   true,
		},
		"WrapsNotFound": {
			reason: "An error that wraps an error with method `IsNotFound() bool` should be considered a not found error.",
			err:    errors.Wrap(notFoundError{errors.New("boom")}, "because reasons"),
			want:   true,
		},
		"SomethingElse": {
			reason: "An error without method `IsNotFound() bool` should not be considered a not found error.",
			err:    errors.New("boom"),
			want:   false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsNotFound(tc.err)
			if got != tc.want {
				t.Errorf("IsNotFound(...): Want %t, got %t", tc.want, got)
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	type want struct {
		value any
		err   error
	}

	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataName": {
			reason: "It should be possible to get a field from a nested object",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"name":"cool"}}`),
			want: want{
				value: "cool",
			},
		},
		"ContainerName": {
			reason: "It should be possible to get a field from an object array element",
			path:   "spec.containers[0].name",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				value: "cool",
			},
		},
		"NestedArray": {
			reason: "It should be possible to get a field from a nested array",
			path:   "items[0][1]",
			data:   []byte(`{"items":[["a", "b"]]}`),
			want: want{
				value: "b",
			},
		},
		"OwnerRefController": {
			reason: "Requesting a boolean field path should work.",
			path:   "metadata.ownerRefs[0].controller",
			data:   []byte(`{"metadata":{"ownerRefs":[{"controller": true}]}}`),
			want: want{
				value: true,
			},
		},
		"MetadataVersion": {
			reason: "Requesting an integer field should work",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				value: int64(2),
			},
		},
		"SomeFloat": {
			reason: "Requesting a float field should work",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2.0}}`),
			want: want{
				value: float64(2),
			},
		},
		"MetadataNope": {
			reason: "Requesting a non-existent object field should fail",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"nope":"cool"}}`),
			want: want{
				err: notFoundError{errors.New("metadata.name: no such field")},
			},
		},
		"InsufficientContainers": {
			reason: "Requesting a non-existent array element should fail",
			path:   "spec.containers[1].name",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				err: notFoundError{errors.New("spec.containers[1]: no such element")},
			},
		},
		"NotAnArray": {
			reason: "Indexing an object should fail",
			path:   "metadata[1]",
			data:   []byte(`{"metadata":{"nope":"cool"}}`),
			want: want{
				err: errors.New("metadata: not an array"),
			},
		},
		"NotAnObject": {
			reason: "Requesting a field in an array should fail",
			path:   "spec.containers[nope].name",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				err: errors.New("spec.containers: not an object"),
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NilParent": {
			reason: "Request for a path with a nil parent value",
			path:   "spec.containers[*].name",
			data:   []byte(`{"spec":{"containers": null}}`),
			want: want{
				err: notFoundError{errors.Errorf("%s: expected map, got nil", "spec.containers")},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetValue(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetValue(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetValue(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetValueInto(t *testing.T) {
	type Struct struct {
		Slice       []string `json:"slice"`
		StringField string   `json:"string"`
		IntField    int      `json:"int"`
	}

	type Slice []string

	type args struct {
		path string
		out  any
	}

	type want struct {
		out any
		err error
	}

	cases := map[string]struct {
		reason string
		data   []byte
		args   args
		want   want
	}{
		"Struct": {
			reason: "It should be possible to get a value into a struct.",
			data:   []byte(`{"s":{"slice":["a"],"string":"b","int":1}}`),
			args: args{
				path: "s",
				out:  &Struct{},
			},
			want: want{
				out: &Struct{Slice: []string{"a"}, StringField: "b", IntField: 1},
			},
		},
		"Slice": {
			reason: "It should be possible to get a value into a slice.",
			data:   []byte(`{"s": ["a", "b"]}`),
			args: args{
				path: "s",
				out:  &Slice{},
			},
			want: want{
				out: &Slice{"a", "b"},
			},
		},
		"MissingPath": {
			reason: "Getting a value from a fieldpath that doesn't exist should return an error.",
			data:   []byte(`{}`),
			args: args{
				path: "s",
				out:  &Struct{},
			},
			want: want{
				out: &Struct{},
				err: notFoundError{errors.New("s: no such field")},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			err := p.GetValueInto(tc.args.path, tc.args.out)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetValueInto(%s): %s: -want error, +got error:\n%s", tc.args.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.out, tc.args.out); diff != "" {
				t.Errorf("\np.GetValueInto(%s): %s: -want, +got:\n%s", tc.args.path, tc.reason, diff)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	type want struct {
		value string
		err   error
	}

	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataName": {
			reason: "It should be possible to get a field from a nested object",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"name":"cool"}}`),
			want: want{
				value: "cool",
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotAString": {
			reason: "Requesting an non-string field path should fail",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				err: errors.New("metadata.version: not a string"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetString(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetString(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetString(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetStringArray(t *testing.T) {
	type want struct {
		value []string
		err   error
	}

	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataLabels": {
			reason: "It should be possible to get a field from a nested object",
			path:   "spec.containers[0].command",
			data:   []byte(`{"spec": {"containers": [{"command": ["/bin/bash"]}]}}`),
			want: want{
				value: []string{"/bin/bash"},
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotAnArray": {
			reason: "Requesting an non-object field path should fail",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				err: errors.New("metadata.version: not an array"),
			},
		},
		"NotAStringArray": {
			reason: "Requesting an non-string-object field path should fail",
			path:   "metadata.versions",
			data:   []byte(`{"metadata":{"versions":[1,2]}}`),
			want: want{
				err: errors.New("metadata.versions: not an array of strings"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetStringArray(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetStringArray(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetStringArray(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetStringObject(t *testing.T) {
	type want struct {
		value map[string]string
		err   error
	}

	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataLabels": {
			reason: "It should be possible to get a field from a nested object",
			path:   "metadata.labels",
			data:   []byte(`{"metadata":{"labels":{"cool":"true"}}}`),
			want: want{
				value: map[string]string{"cool": "true"},
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotAnObject": {
			reason: "Requesting an non-object field path should fail",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				err: errors.New("metadata.version: not an object"),
			},
		},
		"NotAStringObject": {
			reason: "Requesting an non-string-object field path should fail",
			path:   "metadata.versions",
			data:   []byte(`{"metadata":{"versions":{"a": 2}}}`),
			want: want{
				err: errors.New("metadata.versions: not an object with string field values"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetStringObject(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetStringObject(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetStringObject(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	type want struct {
		value bool
		err   error
	}

	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"OwnerRefController": {
			reason: "Requesting a boolean field path should work.",
			path:   "metadata.ownerRefs[0].controller",
			data:   []byte(`{"metadata":{"ownerRefs":[{"controller": true}]}}`),
			want: want{
				value: true,
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotABool": {
			reason: "Requesting an non-boolean field path should fail",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"name":"cool"}}`),
			want: want{
				err: errors.New("metadata.name: not a bool"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetBool(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetBool(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetBool(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetInteger(t *testing.T) {
	type want struct {
		value int64
		err   error
	}

	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataVersion": {
			reason: "Requesting a number field should work",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				value: 2,
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotANumber": {
			reason: "Requesting an non-number field path should fail",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"name":"cool"}}`),
			want: want{
				err: errors.New("metadata.name: not a (int64) number"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetInteger(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetNumber(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetNumber(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestSetValue(t *testing.T) {
	type args struct {
		path  string
		value any
		opts  []PavedOption
	}

	type want struct {
		object map[string]any
		err    error
	}

	cases := map[string]struct {
		reason string
		data   []byte
		args   args
		want   want
	}{
		"MetadataName": {
			reason: "Setting an object field should work",
			data:   []byte(`{"metadata":{"name":"lame"}}`),
			args: args{
				path:  "metadata.name",
				value: "cool",
			},
			want: want{
				object: map[string]any{
					"metadata": map[string]any{
						"name": "cool",
					},
				},
			},
		},
		"NonExistentMetadataName": {
			reason: "Setting a non-existent object field should work",
			data:   []byte(`{}`),
			args: args{
				path:  "metadata.name",
				value: "cool",
			},
			want: want{
				object: map[string]any{
					"metadata": map[string]any{
						"name": "cool",
					},
				},
			},
		},
		"ContainerName": {
			reason: "Setting a field of an object that is an array element should work",
			data:   []byte(`{"spec":{"containers":[{"name":"lame"}]}}`),
			args: args{
				path:  "spec.containers[0].name",
				value: "cool",
			},
			want: want{
				object: map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "cool",
							},
						},
					},
				},
			},
		},
		"NonExistentContainerName": {
			reason: "Setting a field of a non-existent object that is an array element should work",
			data:   []byte(`{}`),
			args: args{
				path:  "spec.containers[0].name",
				value: "cool",
			},
			want: want{
				object: map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "cool",
							},
						},
					},
				},
			},
		},
		"NewContainer": {
			reason: "Growing an array object field should work",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			args: args{
				path:  "spec.containers[1].name",
				value: "cooler",
			},
			want: want{
				object: map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "cool",
							},
							map[string]any{
								"name": "cooler",
							},
						},
					},
				},
			},
		},
		"NestedArray": {
			reason: "Setting a value in a nested array should work",
			data:   []byte(`{}`),
			args: args{
				path:  "data[0][0]",
				value: "a",
			},
			want: want{
				object: map[string]any{
					"data": []any{
						[]any{"a"},
					},
				},
			},
		},
		"GrowNestedArray": {
			reason: "Growing then setting a value in a nested array should work",
			data:   []byte(`{"data":[["a"]]}`),
			args: args{
				path:  "data[0][1]",
				value: "b",
			},
			want: want{
				object: map[string]any{
					"data": []any{
						[]any{"a", "b"},
					},
				},
			},
		},
		"GrowArrayField": {
			reason: "Growing then setting a value in an array field should work",
			data:   []byte(`{"data":["a"]}`),
			args: args{
				path:  "data[2]",
				value: "c",
			},
			want: want{
				object: map[string]any{
					"data": []any{"a", nil, "c"},
				},
			},
		},
		"RejectsHighIndexes": {
			reason: "Paths having indexes above the maximum default value are rejected",
			data:   []byte(`{"data":["a"]}`),
			args: args{
				path:  fmt.Sprintf("data[%v]", DefaultMaxFieldPathIndex+1),
				value: "c",
			},
			want: want{
				object: map[string]any{
					"data": []any{"a"},
				},
				err: errors.Errorf("index %v is greater than max allowed index %v",
					DefaultMaxFieldPathIndex+1, DefaultMaxFieldPathIndex),
			},
		},
		"NotRejectsHighIndexesIfNoDefaultOptions": {
			reason: "Paths having indexes above the maximum default value are not rejected if default disabled",
			data:   []byte(`{"data":["a"]}`),
			args: args{
				path:  fmt.Sprintf("data[%v]", DefaultMaxFieldPathIndex+1),
				value: "c",
				opts:  []PavedOption{WithMaxFieldPathIndex(0)},
			},
			want: want{
				object: map[string]any{
					"data": func() []any {
						res := make([]any, DefaultMaxFieldPathIndex+2)
						res[0] = "a"
						res[DefaultMaxFieldPathIndex+1] = "c"

						return res
					}(),
				},
			},
		},
		"MapStringString": {
			reason: "A map of string to string should be converted to a map of string to any",
			data:   []byte(`{"metadata":{}}`),
			args: args{
				path:  "metadata.labels",
				value: map[string]string{"cool": "very"},
			},
			want: want{
				object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{"cool": "very"},
					},
				},
			},
		},
		"OwnerReference": {
			reason: "An ObjectReference (i.e. struct) should be converted to a map of string to any",
			data:   []byte(`{"metadata":{}}`),
			args: args{
				path: "metadata.ownerRefs[0]",
				value: metav1.OwnerReference{
					APIVersion: "v",
					Kind:       "k",
					Name:       "n",
					UID:        types.UID("u"),
				},
			},
			want: want{
				object: map[string]any{
					"metadata": map[string]any{
						"ownerRefs": []any{
							map[string]any{
								"apiVersion": "v",
								"kind":       "k",
								"name":       "n",
								"uid":        "u",
							},
						},
					},
				},
			},
		},
		"NotAnArray": {
			reason: "Indexing an object field should fail",
			data:   []byte(`{"data":{}}`),
			args: args{
				path: "data[0]",
			},
			want: want{
				object: map[string]any{"data": map[string]any{}},
				err:    errors.New("data is not an array"),
			},
		},
		"NotAnObject": {
			reason: "Requesting a field in an array should fail",
			data:   []byte(`{"data":[]}`),
			args: args{
				path: "data.name",
			},
			want: want{
				object: map[string]any{"data": []any{}},
				err:    errors.New("data is not an object"),
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			args: args{
				path: "spec[]",
			},
			want: want{
				object: map[string]any{},
				err:    errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in, tc.args.opts...)

			err := p.SetValue(tc.args.path, tc.args.value)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.SetValue(%s, %v): %s: -want error, +got error:\n%s", tc.args.path, tc.args.value, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.object, p.object); diff != "" {
				t.Fatalf("\np.SetValue(%s, %v): %s: -want, +got:\n%s", tc.args.path, tc.args.value, tc.reason, diff)
			}
		})
	}
}

func TestExpandWildcards(t *testing.T) {
	type want struct {
		expanded []string
		err      error
	}

	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"NoWildcardExisting": {
			reason: "It should return same path if no wildcard in an existing path",
			path:   "password",
			data:   []byte(`{"password":"top-secret"}`),
			want: want{
				expanded: []string{"password"},
			},
		},
		"NoWildcardNonExisting": {
			reason: "It should return no results if no wildcard in a non-existing path",
			path:   "username",
			data:   []byte(`{"password":"top-secret"}`),
			want: want{
				expanded: []string{},
			},
		},
		"NestedNoWildcardExisting": {
			reason: "It should return same path if no wildcard in an existing path",
			path:   "items[0][1]",
			data:   []byte(`{"items":[["a", "b"]]}`),
			want: want{
				expanded: []string{"items[0][1]"},
			},
		},
		"NestedNoWildcardNonExisting": {
			reason: "It should return no results if no wildcard in a non-existing path",
			path:   "items[0][5]",
			data:   []byte(`{"items":[["a", "b"]]}`),
			want: want{
				expanded: []string{},
			},
		},
		"NestedArray": {
			reason: "It should return all possible paths for an array",
			path:   "items[*][*]",
			data:   []byte(`{"items":[["a", "b", "c"], ["d"]]}`),
			want: want{
				expanded: []string{"items[0][0]", "items[0][1]", "items[0][2]", "items[1][0]"},
			},
		},
		"KeysOfMap": {
			reason: "It should return all possible paths for a map in proper syntax",
			path:   "items[*]",
			data:   []byte(`{"items":{ "key1": "val1", "key2.as.annotation": "val2"}}`),
			want: want{
				expanded: []string{"items.key1", "items[key2.as.annotation]"},
			},
		},
		"ArrayOfObjects": {
			reason: "It should return all possible paths for an array of objects",
			path:   "spec.containers[*][*]",
			data:   []byte(`{"spec":{"containers":[{"name":"cool", "image": "latest", "args": ["start", "now"]}]}}`),
			want: want{
				expanded: []string{"spec.containers[0].name", "spec.containers[0].image", "spec.containers[0].args"},
			},
		},
		"MultiLayer": {
			reason: "It should return all possible paths for a multilayer input",
			path:   "spec.containers[*].args[*]",
			data:   []byte(`{"spec":{"containers":[{"name":"cool", "image": "latest", "args": ["start", "now", "debug"]}]}}`),
			want: want{
				expanded: []string{"spec.containers[0].args[0]", "spec.containers[0].args[1]", "spec.containers[0].args[2]"},
			},
		},
		"WildcardInTheBeginning": {
			reason: "It should return all possible paths for a multilayer input with wildcard in the beginning",
			path:   "spec.containers[*].args[1]",
			data:   []byte(`{"spec":{"containers":[{"name":"cool", "image": "latest", "args": ["start", "now", "debug"]}]}}`),
			want: want{
				expanded: []string{"spec.containers[0].args[1]"},
			},
		},
		"WildcardAtTheEnd": {
			reason: "It should return all possible paths for a multilayer input with wildcard at the end",
			path:   "spec.containers[0].args[*]",
			data:   []byte(`{"spec":{"containers":[{"name":"cool", "image": "latest", "args": ["start", "now", "debug"]}]}}`),
			want: want{
				expanded: []string{"spec.containers[0].args[0]", "spec.containers[0].args[1]", "spec.containers[0].args[2]"},
			},
		},
		"NoData": {
			reason: "If there is no input data, no expanded fields could be found",
			path:   "metadata[*]",
			data:   nil,
			want: want{
				expanded: []string{},
			},
		},
		"InsufficientContainers": {
			reason: "Requesting a non-existent array element should return nothing",
			path:   "spec.containers[1].args[*]",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				expanded: []string{},
			},
		},
		"UnexpectedWildcard": {
			reason: "Requesting a wildcard for an object should fail",
			path:   "spec.containers[0].name[*]",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				err: errors.Wrapf(errors.Errorf("%q: unexpected wildcard usage", "spec.containers[0].name"), "cannot expand wildcards for segments: %q", "spec.containers[0].name[*]"),
			},
		},
		"NotAnArray": {
			reason: "Indexing an object should fail",
			path:   "metadata[1]",
			data:   []byte(`{"metadata":{"nope":"cool"}}`),
			want: want{
				err: errors.Wrapf(errors.New("metadata: not an array"), "cannot expand wildcards for segments: %q", "metadata[1]"),
			},
		},
		"NotAnObject": {
			reason: "Requesting a field in an array should fail",
			path:   "spec.containers[nope].name",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				err: errors.Wrapf(errors.New("spec.containers: not an object"), "cannot expand wildcards for segments: %q", "spec.containers.nope.name"),
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NilValue": {
			reason: "Requesting a wildcard for an object that has nil value",
			path:   "spec.containers[*].name",
			data:   []byte(`{"spec":{"containers": null}}`),
			want: want{
				err: errors.Wrapf(notFoundError{errors.Errorf("wildcard field %q is not found in the path", "spec.containers")}, "cannot expand wildcards for segments: %q", "spec.containers[*].name"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.ExpandWildcards(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.ExpandWildcards(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.expanded, got, cmpopts.SortSlices(func(x, y string) bool {
				return x < y
			})); diff != "" {
				t.Errorf("\np.ExpandWildcards(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestDeleteField(t *testing.T) {
	type args struct {
		path string
	}

	type want struct {
		object map[string]any
		err    error
	}

	cases := map[string]struct {
		reason string
		data   []byte
		args   args
		want   want
	}{
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			args: args{
				path: "spec[]",
			},
			want: want{
				object: map[string]any{},
				err:    errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"IndexGivenForNonArray": {
			reason: "Trying to delete a numbered index from a map should fail.",
			data:   []byte(`{"data":{}}`),
			args: args{
				path: "data[0]",
			},
			want: want{
				object: map[string]any{"data": map[string]any{}},
				err:    errors.Wrap(errors.New("not an array"), "cannot delete data[0]"),
			},
		},
		"KeyGivenForNonMap": {
			reason: "Trying to delete a key from an array should fail.",
			data:   []byte(`{"data":[["a"]]}`),
			args: args{
				path: "data[0].a",
			},
			want: want{
				object: map[string]any{"data": []any{[]any{"a"}}},
				err:    errors.Wrap(errors.New("not an object"), "cannot delete data[0].a"),
			},
		},
		"KeyGivenForNonMapInMiddle": {
			reason: "If one of the segments that is a field corresponds to array, it should fail.",
			data:   []byte(`{"data":[{"another": "field"}]}`),
			args: args{
				path: "data.some.another",
			},
			want: want{
				object: map[string]any{"data": []any{
					map[string]any{
						"another": "field",
					},
				}},
				err: errors.New("data is not an object"),
			},
		},
		"IndexGivenForNonArrayInMiddle": {
			reason: "If one of the segments that is an index corresponds to map, it should fail.",
			data:   []byte(`{"data":{"another": ["field"]}}`),
			args: args{
				path: "data[0].another",
			},
			want: want{
				object: map[string]any{"data": map[string]any{
					"another": []any{
						"field",
					},
				}},
				err: errors.New("data is not an array"),
			},
		},
		"ObjectField": {
			reason: "Deleting a field from a map should work.",
			data:   []byte(`{"metadata":{"name":"lame"}}`),
			args: args{
				path: "metadata.name",
			},
			want: want{
				object: map[string]any{
					"metadata": map[string]any{},
				},
			},
		},
		"ObjectSingleField": {
			reason: "Deleting a field from a map should work.",
			data:   []byte(`{"metadata":{"name":"lame"}, "olala": {"omama": "koala"}}`),
			args: args{
				path: "metadata",
			},
			want: want{
				object: map[string]any{
					"olala": map[string]any{
						"omama": "koala",
					},
				},
			},
		},
		"ObjectLeafField": {
			reason: "Deleting a field that is deep in the tree from a map should work.",
			data:   []byte(`{"spec":{"some": {"more": "delete-me"}}}`),
			args: args{
				path: "spec.some.more",
			},
			want: want{
				object: map[string]any{
					"spec": map[string]any{
						"some": map[string]any{},
					},
				},
			},
		},
		"ObjectMidField": {
			reason: "Deleting a field that is in the middle of the tree from a map should work.",
			data:   []byte(`{"spec":{"some": {"more": "delete-me"}}}`),
			args: args{
				path: "spec.some",
			},
			want: want{
				object: map[string]any{
					"spec": map[string]any{},
				},
			},
		},
		"ObjectInArray": {
			reason: "Deleting a field that is in the middle of the tree from a map should work.",
			data:   []byte(`{"spec":[{"some": {"more": "delete-me"}}]}`),
			args: args{
				path: "spec[0].some.more",
			},
			want: want{
				object: map[string]any{
					"spec": []any{
						map[string]any{
							"some": map[string]any{},
						},
					},
				},
			},
		},
		"ArrayFirstElement": {
			reason: "Deleting the first element from an array should work",
			data:   []byte(`{"items":["a", "b"]}`),
			args: args{
				path: "items[0]",
			},
			want: want{
				object: map[string]any{
					"items": []any{
						"b",
					},
				},
			},
		},
		"ArrayLastElement": {
			reason: "Deleting the last element from an array should work",
			data:   []byte(`{"items":["a", "b"]}`),
			args: args{
				path: "items[1]",
			},
			want: want{
				object: map[string]any{
					"items": []any{
						"a",
					},
				},
			},
		},
		"ArrayMidElement": {
			reason: "Deleting an element that is neither first nor last from an array should work",
			data:   []byte(`{"items":["a", "b", "c"]}`),
			args: args{
				path: "items[1]",
			},
			want: want{
				object: map[string]any{
					"items": []any{
						"a",
						"c",
					},
				},
			},
		},
		"ArrayOnlyElements": {
			reason: "Deleting the only element from an array should work",
			data:   []byte(`{"items":["a"]}`),
			args: args{
				path: "items[0]",
			},
			want: want{
				object: map[string]any{
					"items": []any{},
				},
			},
		},
		"ArrayMultipleIndex": {
			reason: "Deleting an element from an array of array should work",
			data:   []byte(`{"items":[["a", "b"]]}`),
			args: args{
				path: "items[0][1]",
			},
			want: want{
				object: map[string]any{
					"items": []any{
						[]any{
							"a",
						},
					},
				},
			},
		},
		"ArrayNoElement": {
			reason: "Deleting an element from an empty array should work",
			data:   []byte(`{"items":[]}`),
			args: args{
				path: "items[0]",
			},
			want: want{
				object: map[string]any{
					"items": []any{},
				},
			},
		},
		"NonExistentPathInMap": {
			reason: "It should be no-op if the field does not exist already.",
			data:   []byte(`{"items":[]}`),
			args: args{
				path: "items[0].metadata",
			},
			want: want{
				object: map[string]any{
					"items": []any{},
				},
			},
		},
		"NonExistentPathInArray": {
			reason: "It should be no-op if the field does not exist already.",
			data:   []byte(`{"items":{"some": "other"}}`),
			args: args{
				path: "items.metadata[0]",
			},
			want: want{
				object: map[string]any{
					"items": map[string]any{
						"some": "other",
					},
				},
			},
		},
		"NonExistentElementInArray": {
			reason: "It should be no-op if the field does not exist already.",
			data:   []byte(`{"items":["some", "other"]}`),
			args: args{
				path: "items[5]",
			},
			want: want{
				object: map[string]any{
					"items": []any{
						"some", "other",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]any)
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			err := p.DeleteField(tc.args.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.DeleteField(%s): %s: -want error, +got error:\n%s", tc.args.path, tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.object, p.object); diff != "" {
				t.Fatalf("\np.DeleteField(%s): %s: -want, +got:\n%s", tc.args.path, tc.reason, diff)
			}
		})
	}
}
