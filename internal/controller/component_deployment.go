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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// ComponentWorkloadParams holds parameters for building a component workload.
type ComponentWorkloadParams struct {
	Component            string
	Image                string
	Port                 int32
	Command              []string
	Args                 []string
	Env                  []corev1.EnvVar
	Resources            corev1.ResourceRequirements
	Probes               *platformv1alpha1.ComponentProbes
	Replicas             *int32
	VolumeMounts         []corev1.VolumeMount
	Volumes              []corev1.Volume
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
	UseStatefulSet       bool
}

func componentLabels(project *platformv1alpha1.Project, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "supabase",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

func buildPodTemplateSpec(project *platformv1alpha1.Project, params ComponentWorkloadParams) corev1.PodTemplateSpec {
	labels := componentLabels(project, params.Component)
	container := corev1.Container{
		Name:         params.Component,
		Image:        params.Image,
		Command:      params.Command,
		Args:         params.Args,
		Ports:        []corev1.ContainerPort{{Name: "http", ContainerPort: params.Port, Protocol: corev1.ProtocolTCP}},
		Env:          params.Env,
		Resources:    params.Resources,
		VolumeMounts: params.VolumeMounts,
	}
	if params.Probes != nil {
		container.StartupProbe = params.Probes.Startup
		container.ReadinessProbe = params.Probes.Readiness
		container.LivenessProbe = params.Probes.Liveness
	}
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{container}, Volumes: params.Volumes},
	}
}

// BuildDeployment creates a Deployment for a Supabase component.
func BuildDeployment(project *platformv1alpha1.Project, params ComponentWorkloadParams) *appsv1.Deployment {
	labels := componentLabels(project, params.Component)
	replicas := derefInt32(params.Replicas, 1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: componentServiceName(project.Name, params.Component), Namespace: project.Namespace, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: buildPodTemplateSpec(project, params),
		},
	}
}

// BuildStatefulSet creates a StatefulSet for a Supabase component.
func BuildStatefulSet(project *platformv1alpha1.Project, params ComponentWorkloadParams) *appsv1.StatefulSet {
	labels := componentLabels(project, params.Component)
	replicas := derefInt32(params.Replicas, 1)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: componentServiceName(project.Name, params.Component), Namespace: project.Namespace, Labels: labels},
		Spec: appsv1.StatefulSetSpec{
			Replicas:             &replicas,
			Selector:             &metav1.LabelSelector{MatchLabels: labels},
			ServiceName:          componentServiceName(project.Name, params.Component),
			Template:             buildPodTemplateSpec(project, params),
			VolumeClaimTemplates: params.VolumeClaimTemplates,
		},
	}
}

func StudioWorkloadParams(project *platformv1alpha1.Project) (ComponentWorkloadParams, error) {
	studioSpec := &platformv1alpha1.StudioSpec{}
	if project.Spec.Studio != nil {
		studioSpec = project.Spec.Studio
	}

	image, err := resolveComponentImage(project, componentStudio, studioSpec.Image)
	if err != nil {
		return ComponentWorkloadParams{}, fmt.Errorf("resolving image for %s: %w", componentStudio, err)
	}

	params := ComponentWorkloadParams{
		Component:      "studio",
		Image:          image,
		Port:           3000,
		Env:            StudioEnvVars(project),
		Resources:      studioSpec.Resources,
		Probes:         studioSpec.Probes,
		Replicas:       studioSpec.Replicas,
		UseStatefulSet: true,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "snippets", MountPath: "/var/lib/studio/snippets"},
			{Name: "functions-main", MountPath: "/var/lib/studio/functions/main/index.ts", SubPath: "index.ts"},
		},
		Volumes: []corev1.Volume{{
			Name: "functions-main",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: functionsCodeConfigMapName(project.Name)},
				},
			},
		}},
	}
	if studioSpec.Snippets != nil && studioSpec.Snippets.VolumeClaimTemplate != nil {
		tpl := studioSpec.Snippets.VolumeClaimTemplate
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "snippets"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      tpl.AccessModes,
				StorageClassName: tpl.StorageClassName,
				Resources:        tpl.Resources,
			},
		}
		params.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{pvc}
	} else {
		params.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "snippets"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")}},
			},
		}}
	}
	return params, nil
}

func attachStudioFunctions(params *ComponentWorkloadParams, functions []platformv1alpha1.Function) {
	for i := range functions {
		function := &functions[i]
		sourceVolumeName := fmt.Sprintf("function-src-%d", i)
		workVolumeName := fmt.Sprintf("function-work-%d", i)
		params.Volumes = append(params.Volumes, corev1.Volume{
			Name: sourceVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: supabaseFunctionCodeConfigMapName(function)},
				},
			},
		})
		params.Volumes = append(params.Volumes, corev1.Volume{
			Name: workVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		params.VolumeMounts = append(params.VolumeMounts, corev1.VolumeMount{
			Name:      workVolumeName,
			MountPath: "/var/lib/studio/functions/" + function.Spec.FunctionName,
		})
		for _, fileName := range sortedSourceFileNames(function.Spec.Source) {
			params.VolumeMounts = append(params.VolumeMounts, corev1.VolumeMount{
				Name:      sourceVolumeName,
				MountPath: "/var/lib/studio/functions/" + function.Spec.FunctionName + "/" + fileName,
				SubPath:   fileName,
			})
		}
	}
}

func AuthWorkloadParams(project *platformv1alpha1.Project) (ComponentWorkloadParams, error) {
	authSpec := &platformv1alpha1.AuthSpec{}
	if project.Spec.Auth != nil {
		authSpec = project.Spec.Auth
	}

	image, err := resolveComponentImage(project, componentAuth, authSpec.Image)
	if err != nil {
		return ComponentWorkloadParams{}, fmt.Errorf("resolving image for %s: %w", componentAuth, err)
	}
	return ComponentWorkloadParams{Component: "auth", Image: image, Port: 9999, Env: AuthEnvVars(project), Resources: authSpec.Resources, Probes: authSpec.Probes, Replicas: authSpec.Replicas}, nil
}

func RestWorkloadParams(project *platformv1alpha1.Project) (ComponentWorkloadParams, error) {
	restSpec := &platformv1alpha1.RestSpec{}
	if project.Spec.Rest != nil {
		restSpec = project.Spec.Rest
	}

	image, err := resolveComponentImage(project, componentRest, restSpec.Image)
	if err != nil {
		return ComponentWorkloadParams{}, fmt.Errorf("resolving image for %s: %w", componentRest, err)
	}
	return ComponentWorkloadParams{Component: "rest", Image: image, Port: 3000, Env: RestEnvVars(project), Resources: restSpec.Resources, Probes: restSpec.Probes, Replicas: restSpec.Replicas}, nil
}

func RealtimeWorkloadParams(project *platformv1alpha1.Project) (ComponentWorkloadParams, error) {
	realtimeSpec := &platformv1alpha1.RealtimeSpec{}
	if project.Spec.Realtime != nil {
		realtimeSpec = project.Spec.Realtime
	}

	image, err := resolveComponentImage(project, componentRealtime, realtimeSpec.Image)
	if err != nil {
		return ComponentWorkloadParams{}, fmt.Errorf("resolving image for %s: %w", componentRealtime, err)
	}
	return ComponentWorkloadParams{Component: "realtime", Image: image, Port: 4000, Env: RealtimeEnvVars(project), Resources: realtimeSpec.Resources, Probes: realtimeSpec.Probes, Replicas: realtimeSpec.Replicas}, nil
}

func StorageWorkloadParams(project *platformv1alpha1.Project) (ComponentWorkloadParams, error) {
	storageSpec := &platformv1alpha1.StorageSpec{}
	if project.Spec.Storage != nil {
		storageSpec = project.Spec.Storage
	}

	image, err := resolveComponentImage(project, componentStorage, storageSpec.Image)
	if err != nil {
		return ComponentWorkloadParams{}, fmt.Errorf("resolving image for %s: %w", componentStorage, err)
	}

	params := ComponentWorkloadParams{Component: "storage", Image: image, Port: 5000, Env: StorageEnvVars(project), Resources: storageSpec.Resources, Probes: storageSpec.Probes, Replicas: storageSpec.Replicas}
	if derefString(storageSpec.Backend, "file") == "file" {
		params.UseStatefulSet = true
		params.VolumeMounts = []corev1.VolumeMount{{Name: "storage-data", MountPath: "/var/lib/storage"}}
		if storageSpec.File != nil && storageSpec.File.VolumeClaimTemplate != nil {
			tpl := storageSpec.File.VolumeClaimTemplate
			params.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "storage-data"},
				Spec:       corev1.PersistentVolumeClaimSpec{AccessModes: tpl.AccessModes, StorageClassName: tpl.StorageClassName, Resources: tpl.Resources},
			}}
		} else {
			params.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "storage-data"},
				Spec:       corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("50Gi")}}},
			}}
		}
	}
	return params, nil
}

func MetaWorkloadParams(project *platformv1alpha1.Project) (ComponentWorkloadParams, error) {
	metaSpec := &platformv1alpha1.MetaSpec{}
	if project.Spec.Meta != nil {
		metaSpec = project.Spec.Meta
	}

	image, err := resolveComponentImage(project, componentMeta, metaSpec.Image)
	if err != nil {
		return ComponentWorkloadParams{}, fmt.Errorf("resolving image for %s: %w", componentMeta, err)
	}
	return ComponentWorkloadParams{Component: "meta", Image: image, Port: 8080, Env: MetaEnvVars(project), Resources: metaSpec.Resources, Probes: metaSpec.Probes, Replicas: metaSpec.Replicas}, nil
}

func FunctionsWorkloadParams(project *platformv1alpha1.Project, functions []platformv1alpha1.Function) (ComponentWorkloadParams, error) {
	functionsSpec := &platformv1alpha1.FunctionsSpec{}
	if project.Spec.Functions != nil {
		functionsSpec = project.Spec.Functions
	}

	image, err := resolveComponentImage(project, componentFunctions, functionsSpec.Image)
	if err != nil {
		return ComponentWorkloadParams{}, fmt.Errorf("resolving image for %s: %w", componentFunctions, err)
	}

	functionsEnvVars, err := FunctionsEnvVars(project, functions)
	if err != nil {
		return ComponentWorkloadParams{}, fmt.Errorf("building env vars for %s: %w", componentFunctions, err)
	}

	params := ComponentWorkloadParams{
		Component: "functions",
		Image:     image,
		Port:      9000,
		Args:      []string{"start", "--main-service", "/home/deno/functions/main"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "functions-main",
			MountPath: "/home/deno/functions/main/index.ts",
			SubPath:   "index.ts",
		}},
		Volumes: []corev1.Volume{{
			Name: "functions-main",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: functionsCodeConfigMapName(project.Name)},
				},
			},
		}},
		Env:       functionsEnvVars,
		Resources: functionsSpec.Resources,
		Probes:    functionsSpec.Probes,
		Replicas:  functionsSpec.Replicas,
	}

	for i := range functions {
		function := &functions[i]
		sourceVolumeName := fmt.Sprintf("function-src-%d", i)
		workVolumeName := fmt.Sprintf("function-work-%d", i)
		params.Volumes = append(params.Volumes, corev1.Volume{
			Name: sourceVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: supabaseFunctionCodeConfigMapName(function)},
				},
			},
		})
		params.Volumes = append(params.Volumes, corev1.Volume{
			Name: workVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		params.VolumeMounts = append(params.VolumeMounts, corev1.VolumeMount{
			Name:      workVolumeName,
			MountPath: "/home/deno/functions/" + function.Spec.FunctionName,
		})
		for _, fileName := range sortedSourceFileNames(function.Spec.Source) {
			params.VolumeMounts = append(params.VolumeMounts, corev1.VolumeMount{
				Name:      sourceVolumeName,
				MountPath: "/home/deno/functions/" + function.Spec.FunctionName + "/" + fileName,
				SubPath:   fileName,
			})
		}
	}

	return params, nil
}

func functionsCodeConfigMapName(projectName string) string {
	return projectName + "-function-main"
}

func listProjectFunctions(functions []platformv1alpha1.Function, projectName string) []platformv1alpha1.Function {
	projectFunctions := make([]platformv1alpha1.Function, 0)
	for i := range functions {
		if functions[i].Spec.ProjectRef.Name != projectName {
			continue
		}
		if functions[i].Spec.FunctionName == "main" {
			continue
		}
		if err := validateFunctionSource(functions[i].Spec.Source); err != nil {
			continue
		}
		projectFunctions = append(projectFunctions, functions[i])
	}
	sort.Slice(projectFunctions, func(i, j int) bool {
		return projectFunctions[i].Name < projectFunctions[j].Name
	})

	result := make([]platformv1alpha1.Function, 0, len(projectFunctions))
	seen := map[string]struct{}{}
	for i := range projectFunctions {
		if _, found := seen[projectFunctions[i].Spec.FunctionName]; found {
			continue
		}
		seen[projectFunctions[i].Spec.FunctionName] = struct{}{}
		result = append(result, projectFunctions[i])
	}
	return result
}

func sortedSourceFileNames(source map[string]string) []string {
	fileNames := make([]string, 0, len(source))
	for fileName := range source {
		fileNames = append(fileNames, fileName)
	}
	sort.Strings(fileNames)
	return fileNames
}

func functionsMainServiceSource() string {
	return "import * as jose from 'https://deno.land/x/jose@v4.14.4/index.ts'\n\n" +
		"console.log('main function started')\n\n" +
		"const JWT_SECRET = Deno.env.get('JWT_SECRET')\n" +
		"const SUPABASE_URL = Deno.env.get('SUPABASE_URL')\n" +
		"const FUNCTIONS_NO_VERIFY_JWT_RAW = Deno.env.get('FUNCTIONS_NO_VERIFY_JWT') ?? '[]'\n\n" +
		"let FUNCTIONS_NO_VERIFY_JWT: string[] = []\n" +
		"try {\n" +
		"  const parsed = JSON.parse(FUNCTIONS_NO_VERIFY_JWT_RAW)\n" +
		"  if (Array.isArray(parsed)) {\n" +
		"    FUNCTIONS_NO_VERIFY_JWT = parsed\n" +
		"  }\n" +
		"} catch (e) {\n" +
		"  console.error('Invalid FUNCTIONS_NO_VERIFY_JWT JSON:', e)\n" +
		"}\n\n" +
		"function shouldVerifyJWT(functionName: string): boolean {\n" +
		"  return !FUNCTIONS_NO_VERIFY_JWT.includes(functionName)\n" +
		"}\n\n" +
		"let SUPABASE_JWT_KEYS: ReturnType<typeof jose.createRemoteJWKSet> | null = null\n" +
		"if (SUPABASE_URL) {\n" +
		"  try {\n" +
		"    SUPABASE_JWT_KEYS = jose.createRemoteJWKSet(\n" +
		"      new URL('/auth/v1/.well-known/jwks.json', SUPABASE_URL),\n" +
		"    )\n" +
		"  } catch (e) {\n" +
		"    console.error('Failed to fetch JWKS from SUPABASE_URL:', e)\n" +
		"  }\n" +
		"}\n\n" +
		"function getAuthToken(req: Request) {\n" +
		"  const authHeader = req.headers.get('authorization')\n" +
		"  if (!authHeader) {\n" +
		"    throw new Error('Missing authorization header')\n" +
		"  }\n" +
		"  const [bearer, token] = authHeader.split(' ')\n" +
		"  if (bearer !== 'Bearer') {\n" +
		"    throw new Error(\"Auth header is not 'Bearer {token}'\")\n" +
		"  }\n" +
		"  return token\n" +
		"}\n\n" +
		"async function isValidLegacyJWT(jwt: string): Promise<boolean> {\n" +
		"  if (!JWT_SECRET) {\n" +
		"    console.error('JWT_SECRET not available for HS256 token verification')\n" +
		"    return false\n" +
		"  }\n" +
		"  const encoder = new TextEncoder();\n" +
		"  const secretKey = encoder.encode(JWT_SECRET)\n" +
		"  try {\n" +
		"    await jose.jwtVerify(jwt, secretKey)\n" +
		"  } catch (e) {\n" +
		"    console.error('Symmetric Legacy JWT verification error', e);\n" +
		"    return false\n" +
		"  }\n" +
		"  return true\n" +
		"}\n\n" +
		"async function isValidJWT(jwt: string): Promise<boolean> {\n" +
		"  if (!SUPABASE_JWT_KEYS) {\n" +
		"    console.error('JWKS not available for ES256/RS256 token verification')\n" +
		"    return false\n" +
		"  }\n\n" +
		"  try {\n" +
		"    await jose.jwtVerify(jwt, SUPABASE_JWT_KEYS)\n" +
		"  } catch (e) {\n" +
		"    console.error('Asymmetric JWT verification error', e);\n" +
		"    return false\n" +
		"  }\n\n" +
		"  return true\n" +
		"}\n\n" +
		"async function isValidHybridJWT(jwt: string): Promise<boolean> {\n" +
		"  const { alg: jwtAlgorithm } = jose.decodeProtectedHeader(jwt)\n\n" +
		"  if (jwtAlgorithm === 'HS256') {\n" +
		"    console.log(`Legacy token type detected, attempting ${jwtAlgorithm} verification.`)\n" +
		"    return await isValidLegacyJWT(jwt)\n" +
		"  }\n\n" +
		"  if (jwtAlgorithm === 'ES256' || jwtAlgorithm === 'RS256') {\n" +
		"    return await isValidJWT(jwt)\n" +
		"  }\n\n" +
		"  return false\n" +
		"}\n\n" +
		"Deno.serve(async (req: Request) => {\n" +
		"  const url = new URL(req.url)\n" +
		"  const { pathname } = url\n" +
		"  const pathParts = pathname.split('/')\n" +
		"  const serviceName = pathParts[1]\n\n" +
		"  if (!serviceName || serviceName === '') {\n" +
		"    const error = { msg: 'missing function name in request' }\n" +
		"    return new Response(JSON.stringify(error), {\n" +
		"      status: 400,\n" +
		"      headers: { 'Content-Type': 'application/json' },\n" +
		"    })\n" +
		"  }\n\n" +
		"  const verifyJWT = shouldVerifyJWT(serviceName)\n\n" +
		"  if (req.method !== 'OPTIONS' && verifyJWT) {\n" +
		"    try {\n" +
		"      const token = getAuthToken(req)\n" +
		"      const isValidJWT = await isValidHybridJWT(token);\n\n" +
		"      if (!isValidJWT) {\n" +
		"        return new Response(JSON.stringify({ msg: 'Invalid JWT' }), {\n" +
		"          status: 401,\n" +
		"          headers: { 'Content-Type': 'application/json' },\n" +
		"        })\n" +
		"      }\n" +
		"    } catch (e) {\n" +
		"      console.error(e)\n" +
		"      return new Response(JSON.stringify({ msg: e.toString() }), {\n" +
		"        status: 401,\n" +
		"        headers: { 'Content-Type': 'application/json' },\n" +
		"      })\n" +
		"    }\n" +
		"  }\n\n" +
		"  const servicePath = `/home/deno/functions/${serviceName}`\n" +
		"  console.error(`serving the request with ${servicePath}`)\n\n" +
		"  const memoryLimitMb = 150\n" +
		"  const workerTimeoutMs = 1 * 60 * 1000\n" +
		"  const noModuleCache = false\n" +
		"  const importMapPath = null\n" +
		"  const envVarsObj = Deno.env.toObject()\n" +
		"  const envVars = Object.keys(envVarsObj).map((k) => [k, envVarsObj[k]])\n\n" +
		"  try {\n" +
		"    const worker = await EdgeRuntime.userWorkers.create({\n" +
		"      servicePath,\n" +
		"      memoryLimitMb,\n" +
		"      workerTimeoutMs,\n" +
		"      noModuleCache,\n" +
		"      importMapPath,\n" +
		"      envVars,\n" +
		"    })\n" +
		"    return await worker.fetch(req)\n" +
		"  } catch (e) {\n" +
		"    const error = { msg: e.toString() }\n" +
		"    return new Response(JSON.stringify(error), {\n" +
		"      status: 500,\n" +
		"      headers: { 'Content-Type': 'application/json' },\n" +
		"    })\n" +
		"  }\n" +
		"})\n"
}

func (r *ProjectReconciler) ensureFunctionsCodeConfigMap(ctx context.Context, project *platformv1alpha1.Project) error {
	name := functionsCodeConfigMapName(project.Name)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: project.Namespace}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(project, cm, r.Scheme); err != nil {
			return err
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data["index.ts"] = functionsMainServiceSource()
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensuring functions main service ConfigMap: %w", err)
	}

	return nil
}

func computeSecretHash(secret *corev1.Secret) string {
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write(secret.Data[k])
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (r *ProjectReconciler) computeEnvSecretHash(ctx context.Context, namespace string, envs []corev1.EnvVar) (string, error) {
	h := sha256.New()
	for _, e := range envs {
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			continue
		}
		ref := e.ValueFrom.SecretKeyRef
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return "", err
		}
		h.Write([]byte(ref.Name))
		h.Write([]byte(ref.Key))
		h.Write([]byte(computeSecretHash(secret)))
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

func mergeAnnotations(existing, additional map[string]string) map[string]string {
	if existing == nil {
		existing = make(map[string]string)
	}
	maps.Copy(existing, additional)
	return existing
}

// EnsureComponent creates or updates the workload (Deployment or StatefulSet) and Service.
func (r *ProjectReconciler) EnsureComponent(ctx context.Context, project *platformv1alpha1.Project, params ComponentWorkloadParams, svcParams ComponentServiceParams) error {
	logger := log.FromContext(ctx).WithValues("component", params.Component)

	desiredSvc := BuildComponentService(project, svcParams)
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: desiredSvc.Name, Namespace: desiredSvc.Namespace}}
	result, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if err := controllerutil.SetControllerReference(project, svc, r.Scheme); err != nil {
			return err
		}
		svc.Labels = desiredSvc.Labels
		svc.Spec.Type = desiredSvc.Spec.Type
		svc.Spec.Selector = desiredSvc.Spec.Selector
		svc.Spec.Ports = desiredSvc.Spec.Ports
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensuring Service for %s: %w", params.Component, err)
	}
	logger.Info("Ensured Service", "result", result)

	hash, err := r.computeEnvSecretHash(ctx, project.Namespace, params.Env)
	if err != nil {
		return fmt.Errorf("computing secret hash for %s: %w", params.Component, err)
	}

	if params.UseStatefulSet {
		desired := BuildStatefulSet(project, params)
		desired.Spec.Template.Annotations = mergeAnnotations(desired.Spec.Template.Annotations, map[string]string{
			"supabase.io/secret-hash": hash,
		})
		sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
		result, err := ctrl.CreateOrUpdate(ctx, r.Client, sts, func() error {
			if err := controllerutil.SetControllerReference(project, sts, r.Scheme); err != nil {
				return err
			}
			sts.Labels = desired.Labels
			sts.Spec.Replicas = desired.Spec.Replicas
			sts.Spec.Selector = desired.Spec.Selector
			sts.Spec.ServiceName = desired.Spec.ServiceName
			sts.Spec.Template = desired.Spec.Template
			sts.Spec.VolumeClaimTemplates = desired.Spec.VolumeClaimTemplates
			return nil
		})
		if err != nil {
			return fmt.Errorf("ensuring StatefulSet for %s: %w", params.Component, err)
		}
		logger.Info("Ensured StatefulSet", "result", result)
		return nil
	}

	desired := BuildDeployment(project, params)
	desired.Spec.Template.Annotations = mergeAnnotations(desired.Spec.Template.Annotations, map[string]string{
		"supabase.io/secret-hash": hash,
	})
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	result, err = ctrl.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		if err := controllerutil.SetControllerReference(project, deploy, r.Scheme); err != nil {
			return err
		}
		deploy.Labels = desired.Labels
		deploy.Spec.Replicas = desired.Spec.Replicas
		deploy.Spec.Selector = desired.Spec.Selector
		deploy.Spec.Template = desired.Spec.Template
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensuring Deployment for %s: %w", params.Component, err)
	}
	logger.Info("Ensured Deployment", "result", result)
	return nil
}

func (r *ProjectReconciler) DeleteComponent(ctx context.Context, project *platformv1alpha1.Project, params ComponentWorkloadParams, svcParams ComponentServiceParams) error {
	logger := log.FromContext(ctx).WithValues("component", params.Component)

	svcName := componentServiceName(project.Name, svcParams.Component)
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: project.Namespace}}
	if err := r.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting Service for %s: %w", params.Component, err)
	}
	logger.Info("Deleted Service", "name", svcName)

	if params.UseStatefulSet {
		stsName := componentServiceName(project.Name, params.Component)
		sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: stsName, Namespace: project.Namespace}}
		if err := r.Delete(ctx, sts); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting StatefulSet for %s: %w", params.Component, err)
		}
		logger.Info("Deleted StatefulSet", "name", stsName)
		return nil
	}

	deployName := componentServiceName(project.Name, params.Component)
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deployName, Namespace: project.Namespace}}
	if err := r.Delete(ctx, deploy); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting Deployment for %s: %w", params.Component, err)
	}
	logger.Info("Deleted Deployment", "name", deployName)
	return nil
}

type componentDef struct {
	enabled   bool
	params    ComponentWorkloadParams
	svcParams ComponentServiceParams
}

// EnsureAllComponents ensures all enabled Supabase components are deployed.
func (r *ProjectReconciler) EnsureAllComponents(ctx context.Context, project *platformv1alpha1.Project) error {
	functions := []platformv1alpha1.Function{}
	functionsEnabled := project.Spec.Functions == nil || derefBool(project.Spec.Functions.Enabled, true)
	if functionsEnabled {
		if err := r.ensureFunctionsCodeConfigMap(ctx, project); err != nil {
			return err
		}

		functionList := &platformv1alpha1.FunctionList{}
		if err := r.List(ctx, functionList, client.InNamespace(project.Namespace)); err != nil {
			return fmt.Errorf("listing Functions: %w", err)
		}
		functions = listProjectFunctions(functionList.Items, project.Name)
		for i := range functions {
			if err := r.ensureFunctionCodeConfigMapForProject(ctx, &functions[i]); err != nil {
				return err
			}
		}
	} else {
		cmName := functionsCodeConfigMapName(project.Name)
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: project.Namespace}}
		if err := r.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting functions code ConfigMap: %w", err)
		}
	}

	components := []componentDef{}
	studioParams, err := StudioWorkloadParams(project)
	if err != nil {
		return err
	}
	attachStudioFunctions(&studioParams, functions)
	components = append(components, componentDef{enabled: project.Spec.Studio == nil || derefBool(project.Spec.Studio.Enabled, true), params: studioParams, svcParams: ComponentServiceParams{Component: "studio", Port: 3000}})

	authParams, err := AuthWorkloadParams(project)
	if err != nil {
		return err
	}
	components = append(components, componentDef{enabled: project.Spec.Auth == nil || derefBool(project.Spec.Auth.Enabled, true), params: authParams, svcParams: ComponentServiceParams{Component: "auth", Port: 9999}})

	restParams, err := RestWorkloadParams(project)
	if err != nil {
		return err
	}
	components = append(components, componentDef{enabled: project.Spec.Rest == nil || derefBool(project.Spec.Rest.Enabled, true), params: restParams, svcParams: ComponentServiceParams{Component: "rest", Port: 3000}})

	realtimeParams, err := RealtimeWorkloadParams(project)
	if err != nil {
		return err
	}
	components = append(components, componentDef{enabled: project.Spec.Realtime == nil || derefBool(project.Spec.Realtime.Enabled, true), params: realtimeParams, svcParams: ComponentServiceParams{Component: "realtime", Port: 4000}})

	storageParams, err := StorageWorkloadParams(project)
	if err != nil {
		return err
	}
	components = append(components, componentDef{enabled: project.Spec.Storage == nil || derefBool(project.Spec.Storage.Enabled, true), params: storageParams, svcParams: ComponentServiceParams{Component: "storage", Port: 5000}})

	metaParams, err := MetaWorkloadParams(project)
	if err != nil {
		return err
	}
	components = append(components, componentDef{enabled: project.Spec.Meta == nil || derefBool(project.Spec.Meta.Enabled, true), params: metaParams, svcParams: ComponentServiceParams{Component: "meta", Port: 8080}})

	functionsParams, err := FunctionsWorkloadParams(project, functions)
	if err != nil {
		return err
	}
	components = append(components, componentDef{enabled: functionsEnabled, params: functionsParams, svcParams: ComponentServiceParams{Component: "functions", Port: 9000}})

	for _, comp := range components {
		if !comp.enabled {
			if err := r.DeleteComponent(ctx, project, comp.params, comp.svcParams); err != nil {
				return err
			}
			continue
		}
		if err := r.EnsureComponent(ctx, project, comp.params, comp.svcParams); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProjectReconciler) ensureFunctionCodeConfigMapForProject(ctx context.Context, function *platformv1alpha1.Function) error {
	name := supabaseFunctionCodeConfigMapName(function)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: function.Namespace}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(function, cm, r.Scheme); err != nil {
			return err
		}
		cm.Labels = map[string]string{
			"app.kubernetes.io/name":       "supabase",
			"app.kubernetes.io/managed-by": "supabase-operator",
			"app.kubernetes.io/component":  "functions-source",
			"supabase.project":             function.Spec.ProjectRef.Name,
			"supabase.function":            function.Spec.FunctionName,
		}
		cm.Data = sortedSource(function.Spec.Source)
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensuring Function ConfigMap for project reconcile: %w", err)
	}

	return nil
}
