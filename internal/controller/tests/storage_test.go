package tests

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	storagedefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/storage"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	imageTransformationEnv = "ENABLE_IMAGE_TRANSFORMATION"
	imgproxyURLEnv         = "IMGPROXY_URL"
	localImgproxyURL       = "http://127.0.0.1:5001"
	externalImgproxyURL    = "http://external-imgproxy:8080"
)

func TestStorageImgproxyConfiguration(t *testing.T) {
	fromConfigMap := &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "image-config"}, Key: "enabled",
	}}
	for _, tc := range []struct {
		name      string
		config    []corev1.EnvVar
		wantEnv   corev1.EnvVar
		wantURL   *corev1.EnvVar
		wantCount int
	}{
		{name: "default", wantEnv: corev1.EnvVar{Name: imageTransformationEnv, Value: "true"},
			wantURL: &corev1.EnvVar{Name: imgproxyURLEnv, Value: localImgproxyURL}, wantCount: 2},
		{name: "explicitly enabled", config: []corev1.EnvVar{{Name: imageTransformationEnv, Value: "true"}},
			wantEnv: corev1.EnvVar{Name: imageTransformationEnv, Value: "true"},
			wantURL: &corev1.EnvVar{Name: imgproxyURLEnv, Value: localImgproxyURL}, wantCount: 2},
		{name: "disabled", config: []corev1.EnvVar{{Name: imageTransformationEnv, Value: "false"}},
			wantEnv: corev1.EnvVar{Name: imageTransformationEnv, Value: "false"}, wantCount: 1},
		{name: "external URL keeps sidecar", config: []corev1.EnvVar{{Name: imgproxyURLEnv, Value: externalImgproxyURL}},
			wantEnv: corev1.EnvVar{Name: imageTransformationEnv, Value: "true"},
			wantURL: &corev1.EnvVar{Name: imgproxyURLEnv, Value: externalImgproxyURL}, wantCount: 2},
		{name: "disabled preserves custom URL", config: []corev1.EnvVar{
			{Name: imageTransformationEnv, Value: "false"}, {Name: imgproxyURLEnv, Value: externalImgproxyURL}},
			wantEnv: corev1.EnvVar{Name: imageTransformationEnv, Value: "false"},
			wantURL: &corev1.EnvVar{Name: imgproxyURLEnv, Value: externalImgproxyURL}, wantCount: 1},
		{name: "runtime setting keeps sidecar", config: []corev1.EnvVar{{Name: imageTransformationEnv, ValueFrom: fromConfigMap}},
			wantEnv: corev1.EnvVar{Name: imageTransformationEnv, ValueFrom: fromConfigMap},
			wantURL: &corev1.EnvVar{Name: imgproxyURLEnv, Value: localImgproxyURL}, wantCount: 2},
		{name: "runtime URL keeps sidecar", config: []corev1.EnvVar{{Name: imgproxyURLEnv, ValueFrom: fromConfigMap}},
			wantEnv: corev1.EnvVar{Name: imageTransformationEnv, Value: "true"},
			wantURL: &corev1.EnvVar{Name: imgproxyURLEnv, ValueFrom: fromConfigMap}, wantCount: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			state := defaults.NewContext(&core.Project{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}})
			state.Spec.Storage = &core.StorageSpec{Config: tc.config}
			state.Names["Storage"] = "test"
			sts, err := storagedefaults.StorageStatefulSet(state, &core.ResolvedDatabase{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(tc.wantCount))
			env := sts.Spec.Template.Spec.Containers[0].Env
			g.Expect(env).To(ContainElement(tc.wantEnv))
			if tc.wantURL == nil {
				for _, variable := range env {
					g.Expect(variable.Name).NotTo(Equal(imgproxyURLEnv))
				}
			} else {
				g.Expect(env).To(ContainElement(*tc.wantURL))
			}
			if tc.wantCount == 2 {
				imgproxy := sts.Spec.Template.Spec.Containers[1]
				g.Expect(imgproxy.Name).To(Equal("imgproxy"))
				g.Expect(imgproxy.Image).To(Equal(defaults.DefaultImgproxyImage))
				g.Expect(imgproxy.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
				g.Expect(imgproxy.Ports).To(ContainElement(corev1.ContainerPort{Name: "http", ContainerPort: 5001, Protocol: corev1.ProtocolTCP}))
				g.Expect(imgproxy.Env).To(ContainElements(
					corev1.EnvVar{Name: "IMGPROXY_BIND", Value: ":5001"},
					corev1.EnvVar{Name: "IMGPROXY_LOCAL_FILESYSTEM_ROOT", Value: "/"},
					corev1.EnvVar{Name: "IMGPROXY_USE_ETAG", Value: "true"},
					corev1.EnvVar{Name: "IMGPROXY_AUTO_WEBP", Value: "true"},
					corev1.EnvVar{Name: "IMGPROXY_MAX_SRC_RESOLUTION", Value: "16.8"},
				))
				for _, probe := range []*corev1.Probe{imgproxy.LivenessProbe, imgproxy.ReadinessProbe} {
					g.Expect(probe).NotTo(BeNil())
					g.Expect(probe.Exec.Command).To(Equal([]string{"imgproxy", "health"}))
					g.Expect(probe.TimeoutSeconds).To(Equal(int32(5)))
					g.Expect(probe.PeriodSeconds).To(Equal(int32(5)))
					g.Expect(probe.FailureThreshold).To(Equal(int32(3)))
				}
				g.Expect(imgproxy.VolumeMounts).To(Equal(sts.Spec.Template.Spec.Containers[0].VolumeMounts))
				g.Expect(imgproxy.VolumeMounts).To(ContainElement(corev1.VolumeMount{
					Name: "storage-data", MountPath: "/var/lib/storage", SubPath: "storage-data",
				}))
			}
		})
	}
}

func TestStorageImgproxyOverlays(t *testing.T) {
	g := NewWithT(t)
	state := defaults.NewContext(&core.Project{})
	state.Spec.Storage = &core.StorageSpec{Pod: &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "storage", Env: []corev1.EnvVar{{Name: imageTransformationEnv, Value: "false"}}},
		{Name: "imgproxy", Image: "custom/imgproxy:test",
			Resources:      corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")}},
			Env:            []corev1.EnvVar{{Name: "IMGPROXY_MAX_SRC_RESOLUTION", Value: "32"}},
			ReadinessProbe: &corev1.Probe{TimeoutSeconds: 10}},
	}}}}
	sts, err := storagedefaults.StorageStatefulSet(state, &core.ResolvedDatabase{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(2))
	expectEnvValue(t, sts, imageTransformationEnv, "false")
	imgproxy := sts.Spec.Template.Spec.Containers[1]
	g.Expect(imgproxy.Image).To(Equal("custom/imgproxy:test"))
	g.Expect(imgproxy.Resources.Requests.Memory().String()).To(Equal("128Mi"))
	g.Expect(imgproxy.Env).To(ContainElement(corev1.EnvVar{Name: "IMGPROXY_MAX_SRC_RESOLUTION", Value: "32"}))
	g.Expect(imgproxy.ReadinessProbe.TimeoutSeconds).To(Equal(int32(10)))
	g.Expect(imgproxy.ReadinessProbe.Exec.Command).To(Equal([]string{"imgproxy", "health"}))

	state.Spec.Storage.Config = []corev1.EnvVar{{Name: imageTransformationEnv, Value: "false"}}
	sts, err = storagedefaults.StorageStatefulSet(state, &core.ResolvedDatabase{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(2))
	g.Expect(sts.Spec.Template.Spec.Containers[1]).To(Equal(state.Spec.Storage.Pod.Spec.Containers[1]))
}

func TestStorageImgproxyReconciliation(t *testing.T) {
	g := NewWithT(t)
	ctx, r, project, objects := componentFixture(t)
	storage := objects[4].(*core.Storage)
	key := client.ObjectKey{Namespace: project.Namespace, Name: "custom-storage-storage"}
	for _, enabled := range []bool{true, false, true} {
		storage.Spec.Config = nil
		wantCount := 2
		if !enabled {
			storage.Spec.Config = []corev1.EnvVar{{Name: imageTransformationEnv, Value: "false"}}
			wantCount = 1
		}
		g.Expect(r.Update(ctx, storage)).To(Succeed())
		runComponent(t, ctx, r, storage)
		sts := &appsv1.StatefulSet{}
		g.Expect(r.Get(ctx, key, sts)).To(Succeed())
		g.Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(wantCount))
		g.Expect(metav1.IsControlledBy(sts, storage)).To(BeTrue())
		before := sts.DeepCopy()
		runComponent(t, ctx, r, storage)
		g.Expect(r.Get(ctx, key, sts)).To(Succeed())
		g.Expect(sts.Spec).To(Equal(before.Spec))
		g.Expect(sts.ResourceVersion).To(Equal(before.ResourceVersion))
	}
}

func TestStorageLegacyImgproxyExample(t *testing.T) {
	const legacyExample = `
storage:
  config:
    - name: ENABLE_IMAGE_TRANSFORMATION
      value: "true"
    - name: IMGPROXY_URL
      value: http://127.0.0.1:5001
  pod:
    spec:
      containers:
        - name: imgproxy
          image: darthsim/imgproxy:v3.31.4
          ports:
            - name: http
              containerPort: 5001
              protocol: TCP
          env:
            - name: IMGPROXY_BIND
              value: ":5001"
            - name: IMGPROXY_LOCAL_FILESYSTEM_ROOT
              value: /
            - name: IMGPROXY_USE_ETAG
              value: "true"
            - name: IMGPROXY_AUTO_WEBP
              value: "true"
            - name: IMGPROXY_MAX_SRC_RESOLUTION
              value: "16.8"
          livenessProbe:
            exec:
              command: [imgproxy, health]
            timeoutSeconds: 5
            periodSeconds: 5
            failureThreshold: 3
          readinessProbe:
            exec:
              command: [imgproxy, health]
            timeoutSeconds: 5
            periodSeconds: 5
            failureThreshold: 3
          volumeMounts:
            - name: storage-data
              mountPath: /var/lib/storage
              subPath: storage-data
`
	g := NewWithT(t)
	var values struct {
		Storage core.StorageSpec `json:"storage"`
	}
	g.Expect(yaml.Unmarshal([]byte(legacyExample), &values)).To(Succeed())
	state := defaults.NewContext(&core.Project{})
	state.Spec.Storage = &values.Storage
	sts, err := storagedefaults.StorageStatefulSet(state, &core.ResolvedDatabase{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(2))
	expectEnvValue(t, sts, imageTransformationEnv, "true")
	expectEnvValue(t, sts, imgproxyURLEnv, localImgproxyURL)
	want := values.Storage.Pod.Spec.Containers[0].DeepCopy()
	want.ImagePullPolicy = corev1.PullIfNotPresent
	g.Expect(sts.Spec.Template.Spec.Containers[1]).To(Equal(*want))
}

var _ = Describe("Storage API", func() {
	It("requires a local immutable Project reference and permits zero replicas", func() {
		ns := "schema-" + rand.String(6)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		c := &core.Storage{ObjectMeta: metav1.ObjectMeta{Name: "arbitrary-name", Namespace: ns}}
		Expect(apierrors.IsInvalid(k8sClient.Create(ctx, c))).To(BeTrue())
		c.Spec.ProjectRef = corev1.LocalObjectReference{Name: "missing"}
		zero := int32(0)
		c.Spec.Replicas = &zero
		Expect(k8sClient.Create(ctx, c)).To(Succeed())
		defaults := &core.Storage{ObjectMeta: metav1.ObjectMeta{Name: "defaults", Namespace: ns}, Spec: c.Spec}
		defaults.Spec.Replicas = nil
		Expect(k8sClient.Create(ctx, defaults)).To(Succeed())
		Expect(defaults.Spec.Replicas).NotTo(BeNil())
		Expect(*defaults.Spec.Replicas).To(Equal(int32(1)))

		Expect(apierrors.IsInvalid(k8sClient.Patch(ctx, c, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"projectRef":{"name":"another-project"}}}`))))).To(BeTrue())
	})
})
