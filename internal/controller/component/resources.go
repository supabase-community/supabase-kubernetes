package component

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Ensure[T client.Object](ctx context.Context, c client.Client, desired T, owner client.Object, mutate func(T, T) error, description string) error {
	result, err := reconciler.EnsureResource(ctx, c, desired, owner, mutate)
	if err != nil {
		return err
	}
	logger := log.FromContext(ctx).WithValues("name", desired.GetName(), "namespace", desired.GetNamespace())
	messages := map[reconciler.Result]string{
		reconciler.ResultCreated: "Created " + description,
		reconciler.ResultUpdated: "Updated " + description,
	}
	if message, changed := messages[result]; changed {
		logger.Info(message)
		return nil
	}
	logger.V(1).Info(description + " unchanged")
	return nil
}

func StampInputs(ctx context.Context, c client.Client, namespace string, pod *corev1.PodTemplateSpec) error {
	secrets, configs := podInputs(pod)
	inputs := map[string]any{}
	for name, optional := range secrets {
		secret := &corev1.Secret{}
		err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
		if optional && apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
		inputs["secret/"+name] = secret.Data
	}
	for name, optional := range configs {
		config := &corev1.ConfigMap{}
		err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, config)
		if optional && apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
		inputs["config/"+name] = []any{config.Data, config.BinaryData}
	}
	data, err := json.Marshal(inputs)
	if err != nil {
		return err
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations["core.supabase.io/inputs-hash"] = fmt.Sprintf("%x", sha256.Sum256(data))
	return nil
}

func DeploymentReady(ctx context.Context, c client.Client, key client.ObjectKey) error {
	workload := &appsv1.Deployment{}
	if err := c.Get(ctx, key, workload); err != nil {
		return err
	}
	if workload.Status.ObservedGeneration < workload.Generation || workload.Status.ReadyReplicas < *workload.Spec.Replicas || workload.Status.UpdatedReplicas < *workload.Spec.Replicas {
		return fmt.Errorf("waiting for Deployment rollout")
	}
	return nil
}

func StatefulSetReady(ctx context.Context, c client.Client, key client.ObjectKey) error {
	workload := &appsv1.StatefulSet{}
	if err := c.Get(ctx, key, workload); err != nil {
		return err
	}
	if workload.Status.ObservedGeneration < workload.Generation || workload.Status.ReadyReplicas < *workload.Spec.Replicas || workload.Status.CurrentRevision != workload.Status.UpdateRevision {
		return fmt.Errorf("waiting for StatefulSet rollout")
	}
	return nil
}

func podInputs(pod *corev1.PodTemplateSpec) (map[string]bool, map[string]bool) {
	secrets, configs := map[string]bool{}, map[string]bool{}
	for _, volume := range pod.Spec.Volumes {
		if volume.Secret != nil {
			secrets[volume.Secret.SecretName] = volume.Secret.Optional != nil && *volume.Secret.Optional
		}
		if volume.ConfigMap != nil {
			configs[volume.ConfigMap.Name] = volume.ConfigMap.Optional != nil && *volume.ConfigMap.Optional
		}
		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.Secret != nil {
					secrets[source.Secret.Name] = source.Secret.Optional != nil && *source.Secret.Optional
				}
				if source.ConfigMap != nil {
					configs[source.ConfigMap.Name] = source.ConfigMap.Optional != nil && *source.ConfigMap.Optional
				}
			}
		}
	}
	containers := append(append([]corev1.Container{}, pod.Spec.Containers...), pod.Spec.InitContainers...)
	for _, container := range containers {
		for _, env := range container.Env {
			if env.ValueFrom == nil {
				continue
			}
			if ref := env.ValueFrom.SecretKeyRef; ref != nil {
				secrets[ref.Name] = ref.Optional != nil && *ref.Optional
			}
			if ref := env.ValueFrom.ConfigMapKeyRef; ref != nil {
				configs[ref.Name] = ref.Optional != nil && *ref.Optional
			}
		}
		for _, env := range container.EnvFrom {
			if env.SecretRef != nil {
				secrets[env.SecretRef.Name] = env.SecretRef.Optional != nil && *env.SecretRef.Optional
			}
			if env.ConfigMapRef != nil {
				configs[env.ConfigMapRef.Name] = env.ConfigMapRef.Optional != nil && *env.ConfigMapRef.Optional
			}
		}
	}
	return secrets, configs
}
