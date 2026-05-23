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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var _ = Describe("ExternalDatabase Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-external-db"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			extDB := &platformv1alpha1.ExternalDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, extDB)
			if err == nil {
				return
			}
			resource := &platformv1alpha1.ExternalDatabase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.ExternalDatabaseSpec{
					Host: "postgres.test.svc",
					PasswordRef: platformv1alpha1.SecretKeyRef{
						Name: "test-db-secret",
						Key:  "password",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &platformv1alpha1.ExternalDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should set Ready condition on a valid resource", func() {
			controllerReconciler := &ExternalDatabaseReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			extDB := &platformv1alpha1.ExternalDatabase{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, extDB)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(extDB.Status.Conditions, ConditionTypeReady)).To(BeTrue())
		})

	})
})
