/*
Copyright 2023 The Crossplane Authors.

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

package composite

import (
	"errors"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
)

var (
	fuzzScheme = runtime.NewScheme()
)

func init() {
	if err := pkgmetav1alpha1.SchemeBuilder.AddToScheme(fuzzScheme); err != nil {
		panic(err)
	}
	if err := pkgmetav1.SchemeBuilder.AddToScheme(fuzzScheme); err != nil {
		panic(err)
	}
	if err := v1.SchemeBuilder.AddToScheme(fuzzScheme); err != nil {
		panic(err)
	}
}

// Adds a type to the patch
func addType(p *v1.Patch, i int) {
	chooseType := i % 5
	switch chooseType {
	case 0:
		p.Type = v1.PatchTypeFromCompositeFieldPath
	case 1:
		p.Type = v1.PatchTypePatchSet
	case 2:
		p.Type = v1.PatchTypeToCompositeFieldPath
	case 3:
		p.Type = v1.PatchTypeCombineFromComposite
	case 4:
		p.Type = v1.PatchTypeCombineToComposite
	}
}

func FuzzPatchApply(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		f := fuzz.NewConsumer(data)

		cp := &fake.Composite{}
		f.GenerateStruct(cp)

		cd := &fake.Composed{}
		f.GenerateStruct(cd)

		p := &v1.Patch{}
		f.GenerateStruct(p)

		typeIndex, err := f.GetInt()
		if err != nil {
			return
		}
		addType(p, typeIndex)

		_ = Apply(*p, cp, cd)
	})
}

// Adds a type to the transform
func addTransformType(t *v1.Transform, i int) error {
	chooseType := i % 4
	switch chooseType {
	case 0:
		t.Type = v1.TransformTypeMath
		if t.Math == nil {
			return errors.New("Incorrect configuration")
		}
	case 1:
		t.Type = v1.TransformTypeMap
		if t.Map == nil {
			return errors.New("Incorrect configuration")
		}
	case 2:
		t.Type = v1.TransformTypeMatch
		if t.Match == nil {
			return errors.New("Incorrect configuration")
		}
	case 3:
		t.Type = v1.TransformTypeString
		if t.String == nil {
			return errors.New("Incorrect configuration")
		}
	case 4:
		t.Type = v1.TransformTypeConvert
		if t.Convert == nil {
			return errors.New("Incorrect configuration")
		}
	}
	return nil
}

func FuzzTransform(f *testing.F) {
	f.Fuzz(func(tt *testing.T, data []byte) {
		f := fuzz.NewConsumer(data)

		t := &v1.Transform{}
		err := f.GenerateStruct(t)
		if err != nil {
			return
		}
		typeIndex, err := f.GetInt()
		if err != nil {
			return
		}
		err = addTransformType(t, typeIndex)
		if err != nil {
			return
		}

		i, err := f.GetString()
		if err != nil {
			return
		}

		_, _ = Resolve(*t, i)
	})
}

func YamlToUnstructured(yamlStr string) (*unstructured.Unstructured, error) {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yamlStr), &obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: obj}, nil
}
