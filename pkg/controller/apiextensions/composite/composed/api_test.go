/*
Copyright 2020 The Crossplane Authors.

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

package composed

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

func TestConfigure(t *testing.T) {

	tmpl, _ := json.Marshal(&fake.Managed{})

	type args struct {
		cp resource.Composite
		cd resource.Composed
		t  v1alpha1.ComposedTemplate
	}
	type want struct {
		cd  resource.Composed
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"InvalidTemplate": {
			reason: "Invalid template should not be accepted",
			args: args{
				cd: &fake.Composed{},
				t:  v1alpha1.ComposedTemplate{Base: runtime.RawExtension{Raw: []byte("olala")}},
			},
			want: want{
				cd:  &fake.Composed{},
				err: errors.Wrap(errors.New("invalid character 'o' looking for beginning of value"), errUnmarshal),
			},
		},
		"Success": {
			reason: "Configuration should result in the right object with correct generateName",
			args: args{
				cp: &fake.Composite{ObjectMeta: metav1.ObjectMeta{Name: "cp"}},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
				t:  v1alpha1.ComposedTemplate{Base: runtime.RawExtension{Raw: tmpl}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd", GenerateName: "cp-"}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &DefaultConfigurator{}
			err := c.Configure(tc.args.cp, tc.args.cd, tc.args.t)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cd, tc.args.cd); diff != "" {
				t.Errorf("\n%s\nConfigure(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
