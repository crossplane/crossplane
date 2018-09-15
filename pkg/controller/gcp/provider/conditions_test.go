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

package provider

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	. "github.com/upbound/conductor/pkg/apis/gcp/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestFilterOutCondition(t *testing.T) {
	g := NewGomegaWithT(t)

	empty := []ProviderCondition{}
	validOnly := append(empty, *newCondition(Valid, corev1.ConditionTrue, "", ""))
	invalidOnly := append(empty, *newCondition(Invalid, corev1.ConditionTrue, "", ""))
	mixed := append(validOnly, invalidOnly...)
	mixedWithDuplicates := append(mixed, mixed...)

	// empty - any
	g.Expect(filterOutCondition(empty, Valid)).To(BeNil())

	// {valid} - invalid = {valid}
	g.Expect(filterOutCondition(validOnly, Invalid)).To(Equal(validOnly))
	// {valid} - valid = nil}
	g.Expect(filterOutCondition(validOnly, Valid)).To(BeNil())
	// {valid, invalid} - invalid = {valid}
	g.Expect(filterOutCondition(mixed, Invalid)).To(Equal(validOnly))
	// {valid, invalid} - valid = {invalid}
	g.Expect(filterOutCondition(mixed, Valid)).To(Equal(invalidOnly))

	// {valid,invalid,valid,invalid} - invalid = {valid,valid}
	c := filterOutCondition(mixedWithDuplicates, Invalid)
	g.Expect(c).To(Equal(append(validOnly, validOnly...)))
	// {valid,valid} - invalid = {valid, valid} (no change)
	c = filterOutCondition(c, Invalid)
	g.Expect(c).To(Equal(append(validOnly, validOnly...)))
	// {valid,valid} - valid = {nil}
	g.Expect(filterOutCondition(c, Valid)).To(BeNil())
}

func TestRemoveCondition(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	removeCondition(status, Valid)
	g.Expect(status.Conditions).To(BeNil())

	conditions := []ProviderCondition{*newCondition(Valid, corev1.ConditionTrue, "", "")}
	setCondition(status, conditions[0])
	g.Expect(status.Conditions).To(Equal(conditions))
	removeCondition(status, Invalid)
	g.Expect(status.Conditions).To(Equal(conditions))
	removeCondition(status, Valid)
	g.Expect(status.Conditions).To(BeNil())
}

func TestGetConditions(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	c := getCondition(*status, Invalid)
	g.Expect(c).To(BeNil())

	st := time.Now()
	setCondition(status, *newCondition(Valid, corev1.ConditionTrue, "", ""))

	g.Expect(status.Conditions).To(Not(BeNil()))

	c = getCondition(*status, Invalid)
	g.Expect(c).To(BeNil())

	c = getCondition(*status, Valid)
	g.Expect(c.Type).To(Equal(Valid))
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.LastTransitionTime.After(st)).To(BeTrue())
}

func TestSetConditions(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	valid := *newCondition(Valid, corev1.ConditionTrue, "", "")
	setCondition(status, valid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{valid}))

	invalid := *newCondition(Invalid, corev1.ConditionFalse, "Invalid reason", "")
	setCondition(status, invalid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{valid, invalid}))

	// new valid - diff message only - no change
	newValid := *newCondition(Valid, corev1.ConditionTrue, "", "bar")
	setCondition(status, newValid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{valid, invalid}))

	// new valid - diff reason and message - change
	newValid.Reason = "foo"
	valid = newValid
	setCondition(status, newValid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{invalid, valid}))

	// new valid - diff Status  - change
	newValid.Status = corev1.ConditionUnknown
	valid = newValid
	setCondition(status, newValid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{invalid, valid}))
}

func TestSetInvalid(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	ts := time.Now()
	setInvalid(status, "fail", "bye")
	i := getCondition(*status, Invalid)
	g.Expect(i).To(Not(BeNil()))
	g.Expect(i.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(i.Reason).To(Equal("fail"))
	g.Expect(i.Message).To(Equal("bye"))
	g.Expect(i.LastTransitionTime.After(ts)).To(BeTrue())
	v := getCondition(*status, Valid)
	g.Expect(v).To(BeNil())

	removeCondition(status, Invalid)
	g.Expect(status.Conditions).To(BeNil())

	valid := *newCondition(Valid, corev1.ConditionTrue, "", "")
	setCondition(status, valid)

	ts = time.Now()
	setInvalid(status, "fail", "bye")
	i = getCondition(*status, Invalid)
	g.Expect(i).To(Not(BeNil()))
	g.Expect(i.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(i.Reason).To(Equal("fail"))
	g.Expect(i.Message).To(Equal("bye"))
	g.Expect(i.LastTransitionTime.After(ts)).To(BeTrue())
	v = getCondition(*status, Valid)
	g.Expect(v).To(Not(BeNil()))
	g.Expect(v.Status).To(Equal(corev1.ConditionFalse))
	g.Expect(v.LastTransitionTime.After(ts)).To(BeTrue())
}

func TestSetValid(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	ts := time.Now()

	setValid(status, "hello")
	g.Expect(len(status.Conditions)).To(Equal(1))
	v := getCondition(*status, Valid)
	g.Expect(v).To(Not(BeNil()))
	g.Expect(v.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(v.Reason).To(Equal(""))
	g.Expect(v.Message).To(Equal("hello"))
	g.Expect(v.LastTransitionTime.After(ts)).To(BeTrue())
	i := getCondition(*status, Invalid)
	g.Expect(i).To(BeNil())

	removeCondition(status, Valid)
	g.Expect(status.Conditions).To(BeNil())

	invalid := *newCondition(Invalid, corev1.ConditionTrue, "fail", "")
	setCondition(status, invalid)

	ts = time.Now()
	setValid(status, "hello")
	v = getCondition(*status, Valid)
	g.Expect(v).To(Not(BeNil()))
	g.Expect(v.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(v.Reason).To(Equal(""))
	g.Expect(v.Message).To(Equal("hello"))
	g.Expect(v.LastTransitionTime.After(ts)).To(BeTrue())
	i = getCondition(*status, Invalid)
	g.Expect(i.Status).To(Equal(corev1.ConditionFalse))
}
