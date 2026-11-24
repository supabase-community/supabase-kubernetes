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

package tests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	externalcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/externaldatabase"
	"github.com/supabase-community/supabase-kubernetes/internal/database"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
)

var _ = Describe("ExternalDatabase Controller", func() {
	var ns string
	var db *core.ExternalDatabase
	var secret *corev1.Secret
	const timeout = 20 * time.Second
	const polling = 100 * time.Millisecond
	BeforeEach(func() {
		ns = "external-test-" + rand.String(6)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		db = &core.ExternalDatabase{
			ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: ns},
			Spec:       core.ExternalDatabaseSpec{Host: "postgres.example.com", Port: 5433, DBName: "supabase", User: "postgres", PasswordRef: core.SecretKeyRef{Name: "credentials", Key: "password"}},
		}
		secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: ns}, Data: map[string][]byte{"password": []byte("existing-password")}}
	})
	waitReady := func(ready bool, reason string) {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), db)).To(Succeed())
			condition := meta.FindStatusCondition(db.Status.Conditions, "Ready")
			g.Expect(condition).NotTo(BeNil())
			g.Expect(meta.IsStatusConditionTrue(db.Status.Conditions, "Ready")).To(Equal(ready))
			g.Expect(condition.Reason).To(Equal(reason))
			g.Expect(condition.ObservedGeneration).To(Equal(db.Generation))
			if ready {
				g.Expect(db.Status.ResolvedDatabase).To(Equal(&core.ResolvedDatabase{Host: db.Spec.Host, Port: db.Spec.Port, DBName: db.Spec.DBName, User: db.Spec.User, PasswordRef: db.Spec.PasswordRef}))
			} else {
				g.Expect(db.Status.ResolvedDatabase).To(BeNil())
			}
		}, timeout, polling).Should(Succeed())
	}
	It("resolves Secret changes and reference changes without provisioning resources", func() {
		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		waitReady(false, "SecretNotFound")
		secret.Data = map[string][]byte{"other": []byte("value")}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		waitReady(false, "SecretKeyNotFound")
		secret.Data["password"] = []byte("existing-password")
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())
		waitReady(true, "ReconcileSucceeded")

		// An explicit repeat must not write an unchanged status or the source Secret.
		version := db.ResourceVersion
		r := &externalcontroller.Reconciler{Client: k8sClient}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(db)})
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), db)).To(Succeed())
		Expect(db.ResourceVersion).To(Equal(version))
		original := secret.DeepCopy()
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
		Expect(secret).To(Equal(original))

		for _, list := range []client.ObjectList{&appsv1.StatefulSetList{}, &appsv1.DeploymentList{}, &batchv1.JobList{}, &corev1.ServiceList{}, &corev1.PersistentVolumeClaimList{}, &corev1.ConfigMapList{}} {
			Expect(k8sClient.List(ctx, list, client.InNamespace(ns))).To(Succeed())
			items, err := meta.ExtractList(list)
			Expect(err).NotTo(HaveOccurred())
			Expect(items).To(BeEmpty())
		}
		secrets := &corev1.SecretList{}
		Expect(k8sClient.List(ctx, secrets, client.InNamespace(ns))).To(Succeed())
		Expect(secrets.Items).To(HaveLen(1))

		delete(secret.Data, "password")
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())
		waitReady(false, "SecretKeyNotFound")
		secret.Data["password"] = []byte("rotated-password")
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())
		waitReady(true, "ReconcileSucceeded")
		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		waitReady(false, "SecretNotFound")
		secret.ResourceVersion = ""
		secret.UID = ""
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		waitReady(true, "ReconcileSucceeded")

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), db)).To(Succeed())
			db.Spec.Host = "new.example.com"
			db.Spec.PasswordRef.Name = "new-credentials"
			g.Expect(k8sClient.Update(ctx, db)).To(Succeed())
		}, timeout, polling).Should(Succeed())
		waitReady(false, "SecretNotFound")
		replacement := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "new-credentials", Namespace: ns}, Data: secret.Data}
		Expect(k8sClient.Create(ctx, replacement)).To(Succeed())
		waitReady(true, "ReconcileSucceeded")
		Expect(k8sClient.Delete(ctx, db)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(replacement), replacement)).To(Succeed())
		Expect(replacement.OwnerReferences).To(BeEmpty())
		Expect(replacement.Data).To(Equal(secret.Data))
	})

	It("keeps Project migrations and synchronization and supplies components with external connection parameters", func() {
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		waitReady(true, "ReconcileSucceeded")
		proj := &core.Project{ObjectMeta: metav1.ObjectMeta{Name: "project", Namespace: ns}, Spec: core.ProjectSpec{PublicURL: "http://localhost:8000", DatabaseRef: core.DatabaseRef{Kind: "ExternalDatabase", Name: db.Name}}}
		Expect(k8sClient.Create(ctx, proj)).To(Succeed())
		migration := &core.Migration{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: "project-migration-1"}, migration)).To(Succeed())
			g.Expect(migration.Spec.DatabaseRef).To(Equal(proj.Spec.DatabaseRef))
			g.Expect(migration.Spec.Migrations).NotTo(BeEmpty())
		}, timeout, polling).Should(Succeed())
		for _, name := range []string{migration.Name + "-job", projectdefaults.SyncJWTJobName(projectdefaults.NewContext(proj)), projectdefaults.SyncPasswordJobName(projectdefaults.NewContext(proj))} {
			Eventually(func(g Gomega) {
				job := &batchv1.Job{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, job)).To(Succeed())
				if name == migration.Name+"-job" {
					g.Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElements(
						corev1.EnvVar{Name: "PGHOST", Value: db.Spec.Host}, corev1.EnvVar{Name: "PGPORT", Value: "5433"},
						corev1.EnvVar{Name: "PGDATABASE", Value: db.Spec.DBName}, corev1.EnvVar{Name: "PGUSER", Value: db.Spec.User},
						corev1.EnvVar{Name: "PGPASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name}, Key: "password"}}},
					))
				}
				job.Status.Succeeded = 1
				g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
			}, timeout, polling).Should(Succeed())
		}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(proj), proj)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(proj.Status.Conditions, "Ready")).To(BeTrue())
		}, timeout, polling).Should(Succeed())
		rest := &core.Rest{ObjectMeta: metav1.ObjectMeta{Name: "rest", Namespace: ns}, Spec: core.RestSpec{ProjectRef: corev1.LocalObjectReference{Name: proj.Name}}}
		Expect(k8sClient.Create(ctx, rest)).To(Succeed())
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: "rest-rest"}, deployment)).To(Succeed())
			g.Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PGRST_DB_URI", Value: "postgres://authenticator:$(PGRST_DB_PASSWORD)@postgres.example.com:5433/supabase"}))
		}, timeout, polling).Should(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(migration), migration)).To(Succeed())
		appliedAt := migration.Status.AppliedAt.DeepCopy()
		appliedHash := migration.Status.AppliedHash
		appliedJob := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: migration.Name + "-job"}, appliedJob)).To(Succeed())
		appliedJobUID := appliedJob.UID
		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		waitReady(false, "SecretNotFound")
		Eventually(func(g Gomega) {
			_, ready, err := database.ResolveRef(ctx, k8sClient, proj.Spec.DatabaseRef, ns)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ready).To(BeFalse())
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(proj), proj)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(proj.Status.Conditions, "Ready")).To(BeFalse())
		}, timeout, polling).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(migration), migration)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(migration.Status.Conditions, "Ready")).To(BeFalse())
		}, timeout, polling).Should(Succeed())
		secret.ResourceVersion = ""
		secret.UID = ""
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		waitReady(true, "ReconcileSucceeded")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(migration), migration)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(migration.Status.Conditions, "Ready")).To(BeTrue())
			g.Expect(migration.Status.AppliedAt).To(Equal(appliedAt))
			g.Expect(migration.Status.AppliedHash).To(Equal(appliedHash))
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(appliedJob), appliedJob)).To(Succeed())
			g.Expect(appliedJob.UID).To(Equal(appliedJobUID))
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(proj), proj)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(proj.Status.Conditions, "Ready")).To(BeTrue())
		}, timeout, polling).Should(Succeed())

	})
})
