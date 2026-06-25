/*
Copyright 2026.

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

package reconciler

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Status helpers", func() {
	var obj *supabasev1alpha1.SingleDatabase

	BeforeEach(func() {
		obj = &supabasev1alpha1.SingleDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-db",
				Namespace:  "default",
				Generation: 42,
			},
		}
	})

	Describe("SetCondition", func() {
		It("should add a new condition when the slice is empty", func() {
			SetCondition(obj, "CustomType", metav1.ConditionTrue, "CustomReason", "custom message")

			Expect(obj.Status.Conditions).To(HaveLen(1))
			Expect(obj.Status.Conditions[0].Type).To(Equal("CustomType"))
			Expect(obj.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(obj.Status.Conditions[0].Reason).To(Equal("CustomReason"))
			Expect(obj.Status.Conditions[0].Message).To(Equal("custom message"))
			Expect(obj.Status.Conditions[0].ObservedGeneration).To(Equal(int64(42)))
		})

		It("should update an existing condition of the same type", func() {
			SetCondition(obj, "CustomType", metav1.ConditionFalse, "OldReason", "old message")
			SetCondition(obj, "CustomType", metav1.ConditionTrue, "NewReason", "new message")

			Expect(obj.Status.Conditions).To(HaveLen(1))
			Expect(obj.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(obj.Status.Conditions[0].Reason).To(Equal("NewReason"))
			Expect(obj.Status.Conditions[0].Message).To(Equal("new message"))
		})

		It("should preserve other conditions when adding or updating", func() {
			SetCondition(obj, "TypeA", metav1.ConditionTrue, "ReasonA", "message A")
			SetCondition(obj, "TypeB", metav1.ConditionFalse, "ReasonB", "message B")

			Expect(obj.Status.Conditions).To(HaveLen(2))
			Expect(obj.Status.Conditions[0].Type).To(Equal("TypeA"))
			Expect(obj.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(obj.Status.Conditions[1].Type).To(Equal("TypeB"))
			Expect(obj.Status.Conditions[1].Status).To(Equal(metav1.ConditionFalse))
		})
	})

	Describe("SetReady", func() {
		It("should set Ready condition to True with provided reason and message", func() {
			SetReady(obj, "AllGood", "everything is fine")

			Expect(obj.Status.Conditions).To(HaveLen(1))
			Expect(obj.Status.Conditions[0].Type).To(Equal(ConditionTypeReady))
			Expect(obj.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(obj.Status.Conditions[0].Reason).To(Equal("AllGood"))
			Expect(obj.Status.Conditions[0].Message).To(Equal("everything is fine"))
		})

		It("should update existing Ready condition from False to True", func() {
			SetNotReady(obj, "NotReady", "not ready yet")
			SetReady(obj, "NowReady", "ready now")

			Expect(obj.Status.Conditions).To(HaveLen(1))
			Expect(obj.Status.Conditions[0].Type).To(Equal(ConditionTypeReady))
			Expect(obj.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(obj.Status.Conditions[0].Reason).To(Equal("NowReady"))
		})
	})

	Describe("SetNotReady", func() {
		It("should set Ready condition to False with provided reason and message", func() {
			SetNotReady(obj, "SomethingFailed", "something went wrong")

			Expect(obj.Status.Conditions).To(HaveLen(1))
			Expect(obj.Status.Conditions[0].Type).To(Equal(ConditionTypeReady))
			Expect(obj.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(obj.Status.Conditions[0].Reason).To(Equal("SomethingFailed"))
			Expect(obj.Status.Conditions[0].Message).To(Equal("something went wrong"))
		})

		It("should update existing Ready condition from True to False", func() {
			SetReady(obj, "WasReady", "was ready")
			SetNotReady(obj, "NoLongerReady", "no longer ready")

			Expect(obj.Status.Conditions).To(HaveLen(1))
			Expect(obj.Status.Conditions[0].Type).To(Equal(ConditionTypeReady))
			Expect(obj.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(obj.Status.Conditions[0].Reason).To(Equal("NoLongerReady"))
		})
	})
})
