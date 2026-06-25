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

package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/function"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

var _ = Describe("Function Controller", func() {
	const (
		defaultTimeout = 10 * time.Second
		defaultPolling = 150 * time.Millisecond
	)

	var ns string

	BeforeEach(func() {
		ns = "fn-test-" + rand.String(6)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		})).To(Succeed())
	})

	newFunction := func(name string, source map[string]string) *supabasev1alpha1.Function {
		return &supabasev1alpha1.Function{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: supabasev1alpha1.FunctionSpec{
				ProjectRef:   "demo",
				FunctionName: "hello",
				Source:       source,
			},
		}
	}

	Context("when a Function is created", func() {
		It("creates a ConfigMap with the source files and reports Ready=True", func() {
			fn := newFunction("hello", map[string]string{
				"index.ts": "Deno.serve(() => new Response(\"hello\"))",
			})
			Expect(k8sClient.Create(ctx, fn)).To(Succeed())

			cmKey := types.NamespacedName{
				Name:      function.FunctionConfigMapName(fn),
				Namespace: ns,
			}
			cm := &corev1.ConfigMap{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, cmKey, cm)).To(Succeed())
				g.Expect(cm.Data).To(HaveKeyWithValue("index.ts", "Deno.serve(() => new Response(\"hello\"))"))
				g.Expect(cm.OwnerReferences).To(HaveLen(1))
				g.Expect(cm.OwnerReferences[0].Kind).To(Equal("Function"))
				g.Expect(cm.OwnerReferences[0].Name).To(Equal(fn.Name))
				g.Expect(cm.OwnerReferences[0].Controller).NotTo(BeNil())
				g.Expect(*cm.OwnerReferences[0].Controller).To(BeTrue())
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Function{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: fn.Name, Namespace: ns}, got)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(got.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
				cond := meta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond.Reason).To(Equal("ReconcileSucceeded"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})

	Context("when a Function's source is updated", func() {
		It("updates the ConfigMap data", func() {
			fn := newFunction("hello", map[string]string{
				"index.ts": "first",
			})
			Expect(k8sClient.Create(ctx, fn)).To(Succeed())

			cmKey := types.NamespacedName{
				Name:      function.FunctionConfigMapName(fn),
				Namespace: ns,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, cmKey, &corev1.ConfigMap{})
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Function{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: fn.Name, Namespace: ns}, got)).To(Succeed())
				got.Spec.Source = map[string]string{
					"index.ts":  "second",
					"helper.ts": "export const x = 1",
				}
				g.Expect(k8sClient.Update(ctx, got)).To(Succeed())
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, cmKey, cm)).To(Succeed())
				g.Expect(cm.Data).To(HaveKeyWithValue("index.ts", "second"))
				g.Expect(cm.Data).To(HaveKeyWithValue("helper.ts", "export const x = 1"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})
})
