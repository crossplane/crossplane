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

package v1alpha1

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestFilterOutCondition(t *testing.T) {
	g := NewGomegaWithT(t)

	var empty []ProviderCondition
	validOnly := append(empty, *NewProviderCondition(Valid, corev1.ConditionTrue, "", ""))
	invalidOnly := append(empty, *NewProviderCondition(Invalid, corev1.ConditionTrue, "", ""))
	mixed := append(validOnly, invalidOnly...)
	mixedWithDuplicates := append(mixed, mixed...)

	// empty - any
	g.Expect(FilterOutProviderCondition(empty, Valid)).To(BeNil())

	// {valid} - invalid = {valid}
	g.Expect(FilterOutProviderCondition(validOnly, Invalid)).To(Equal(validOnly))
	// {valid} - valid = nil}
	g.Expect(FilterOutProviderCondition(validOnly, Valid)).To(BeNil())
	// {valid, invalid} - invalid = {valid}
	g.Expect(FilterOutProviderCondition(mixed, Invalid)).To(Equal(validOnly))
	// {valid, invalid} - valid = {invalid}
	g.Expect(FilterOutProviderCondition(mixed, Valid)).To(Equal(invalidOnly))

	// {valid,invalid,valid,invalid} - invalid = {valid,valid}
	c := FilterOutProviderCondition(mixedWithDuplicates, Invalid)
	g.Expect(c).To(Equal(append(validOnly, validOnly...)))
	// {valid,valid} - invalid = {valid, valid} (no change)
	c = FilterOutProviderCondition(c, Invalid)
	g.Expect(c).To(Equal(append(validOnly, validOnly...)))
	// {valid,valid} - valid = {nil}
	g.Expect(FilterOutProviderCondition(c, Valid)).To(BeNil())
}

func TestRemoveCondition(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	status.RemoveCondition(Valid)
	g.Expect(status.Conditions).To(BeNil())

	conditions := []ProviderCondition{*NewProviderCondition(Valid, corev1.ConditionTrue, "", "")}
	status.SetCondition(conditions[0])
	g.Expect(status.Conditions).To(Equal(conditions))
	status.RemoveCondition(Invalid)
	g.Expect(status.Conditions).To(Equal(conditions))
	status.RemoveCondition(Valid)
	g.Expect(status.Conditions).To(BeNil())
}

func TestGetConditions(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	c := status.GetCondition(Invalid)
	g.Expect(c).To(BeNil())

	st := time.Now()
	status.SetCondition(*NewProviderCondition(Valid, corev1.ConditionTrue, "", ""))

	g.Expect(status.Conditions).To(Not(BeNil()))

	c = status.GetCondition(Invalid)
	g.Expect(c).To(BeNil())

	c = status.GetCondition(Valid)
	g.Expect(c.Type).To(Equal(Valid))
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(c.LastTransitionTime.After(st)).To(BeTrue())
}

func TestSetConditions(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	valid := *NewProviderCondition(Valid, corev1.ConditionTrue, "", "")
	status.SetCondition(valid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{valid}))

	invalid := *NewProviderCondition(Invalid, corev1.ConditionFalse, "Invalid reason", "")
	status.SetCondition(invalid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{valid, invalid}))

	// new valid - diff message only - no change
	newValid := *NewProviderCondition(Valid, corev1.ConditionTrue, "", "bar")
	status.SetCondition(newValid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{valid, invalid}))

	// new valid - diff reason and message - change
	newValid.Reason = "foo"
	valid = newValid
	status.SetCondition(newValid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{invalid, valid}))

	// new valid - diff Status  - change
	newValid.Status = corev1.ConditionUnknown
	valid = newValid
	status.SetCondition(newValid)
	g.Expect(status.Conditions).To(Equal([]ProviderCondition{invalid, valid}))
}

func TestSetInvalid(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	ts := time.Now()
	status.SetInvalid("fail", "bye")
	i := status.GetCondition(Invalid)
	g.Expect(i).To(Not(BeNil()))
	g.Expect(i.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(i.Reason).To(Equal("fail"))
	g.Expect(i.Message).To(Equal("bye"))
	g.Expect(i.LastTransitionTime.After(ts)).To(BeTrue())
	v := status.GetCondition(Valid)
	g.Expect(v).To(BeNil())

	status.RemoveCondition(Invalid)
	g.Expect(status.Conditions).To(BeNil())

	valid := *NewProviderCondition(Valid, corev1.ConditionTrue, "", "")
	status.SetCondition(valid)

	ts = time.Now()
	status.SetInvalid("fail", "bye")
	i = status.GetCondition(Invalid)
	g.Expect(i).To(Not(BeNil()))
	g.Expect(i.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(i.Reason).To(Equal("fail"))
	g.Expect(i.Message).To(Equal("bye"))
	g.Expect(i.LastTransitionTime.After(ts)).To(BeTrue())
	v = status.GetCondition(Valid)
	g.Expect(v).To(Not(BeNil()))
	g.Expect(v.Status).To(Equal(corev1.ConditionFalse))
	g.Expect(v.LastTransitionTime.After(ts)).To(BeTrue())
}

func TestSetValid(t *testing.T) {
	g := NewGomegaWithT(t)
	status := &ProviderStatus{}
	g.Expect(status.Conditions).To(BeNil())

	ts := time.Now()

	status.SetValid("hello")
	g.Expect(len(status.Conditions)).To(Equal(1))
	v := status.GetCondition(Valid)
	g.Expect(v).To(Not(BeNil()))
	g.Expect(v.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(v.Reason).To(Equal(""))
	g.Expect(v.Message).To(Equal("hello"))
	g.Expect(v.LastTransitionTime.After(ts)).To(BeTrue())
	i := status.GetCondition(Invalid)
	g.Expect(i).To(BeNil())

	status.RemoveCondition(Valid)
	g.Expect(status.Conditions).To(BeNil())

	invalid := *NewProviderCondition(Invalid, corev1.ConditionTrue, "fail", "")
	status.SetCondition(invalid)

	ts = time.Now()
	status.SetValid("hello")
	v = status.GetCondition(Valid)
	g.Expect(v).To(Not(BeNil()))
	g.Expect(v.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(v.Reason).To(Equal(""))
	g.Expect(v.Message).To(Equal("hello"))
	g.Expect(v.LastTransitionTime.After(ts)).To(BeTrue())
	i = status.GetCondition(Invalid)
	g.Expect(i.Status).To(Equal(corev1.ConditionFalse))
}
