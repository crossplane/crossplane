/*
Copyright 2018 The Conductor Authors.

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

package util

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRemoveFinalizer(t *testing.T) {
	g := NewGomegaWithT(t)

	finalizer := "finalizer.test.io"

	type finalizers struct {
		input    []string
		expected []string
	}

	finalizerTests := []finalizers{
		{[]string{}, []string{}},
		{[]string{"foo.bar"}, []string{"foo.bar"}},
		{[]string{finalizer}, []string{}},
		{[]string{finalizer, "foo.bar"}, []string{"foo.bar"}},
		{[]string{finalizer, "foo.bar", finalizer, "fooz.booz"}, []string{"foo.bar", "fooz.booz"}},
	}

	om := &v1.ObjectMeta{}

	for _, tt := range finalizerTests {
		om.Finalizers = tt.input
		RemoveFinalizer(om, finalizer)
		g.Expect(om.Finalizers).To(Equal(tt.expected))
	}
}

func TestAddFinalizer(t *testing.T) {
	g := NewGomegaWithT(t)

	finalizer := "finalizer.test.io"

	type finalizers struct {
		input    []string
		expected []string
	}

	finalizerTests := []finalizers{
		{[]string{}, []string{finalizer}},
		{[]string{"foo.bar"}, []string{"foo.bar", finalizer}},
		{[]string{finalizer, "foo.bar"}, []string{finalizer, "foo.bar"}},
		{[]string{"foo.bar", finalizer}, []string{"foo.bar", finalizer}},
	}

	om := &v1.ObjectMeta{}

	for _, tt := range finalizerTests {
		om.Finalizers = tt.input
		AddFinalizer(om, finalizer)
		g.Expect(om.Finalizers).To(Equal(tt.expected))
	}
}

func TestHasFinalizer(t *testing.T) {
	g := NewGomegaWithT(t)

	finalizer := "finalizer.test.io"

	type finalizers struct {
		input    []string
		expected bool
	}

	finalizerTests := []finalizers{
		{[]string{}, false},
		{[]string{"foo.bar"}, false},
		{[]string{finalizer, "foo.bar"}, true},
		{[]string{"foo.bar", finalizer}, true},
	}

	om := &v1.ObjectMeta{}

	for _, tt := range finalizerTests {
		om.Finalizers = tt.input
		g.Expect(HasFinalizer(om, finalizer)).To(Equal(tt.expected))
	}
}
