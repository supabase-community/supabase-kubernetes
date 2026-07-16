{{/*
Expand the name of the chart.
*/}}
{{- define "supabase-project.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase-project.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "supabase-project.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Namespace for generated references.
Always uses the Helm release namespace.
*/}}
{{- define "supabase-project.namespaceName" -}}
{{- .Release.Namespace }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "supabase-project.labels" -}}
helm.sh/chart: {{ include "supabase-project.chart" . }}
{{ include "supabase-project.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "supabase-project.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase-project.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Render WorkloadConfig fields shared by all components and SingleDatabase.
*/}}
{{- define "supabase-project.renderWorkloadConfig" -}}
{{- $out := "" -}}
{{- with .image }}{{ $out = printf "%simage: %s\n" $out (. | quote) }}{{ end -}}
{{- with .imagePullPolicy }}{{ $out = printf "%simagePullPolicy: %s\n" $out (. | quote) }}{{ end -}}
{{- with .affinity }}{{ $out = printf "%saffinity:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- with .nodeSelector }}{{ $out = printf "%snodeSelector:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- with .podAnnotations }}{{ $out = printf "%spodAnnotations:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- with .podLabels }}{{ $out = printf "%spodLabels:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- with .securityContext }}{{ $out = printf "%ssecurityContext:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- with .priorityClassName }}{{ $out = printf "%spriorityClassName: %s\n" $out (. | quote) }}{{ end -}}
{{- with .resources }}{{ $out = printf "%sresources:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- with .terminationGracePeriodSeconds }}{{ $out = printf "%sterminationGracePeriodSeconds: %d\n" $out (int64 .) }}{{ end -}}
{{- with .tolerations }}{{ $out = printf "%stolerations:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- $out -}}
{{- end }}

{{/*
Render ServiceSpec fields shared by all components and SingleDatabase.
*/}}
{{- define "supabase-project.renderService" -}}
{{- $out := "" -}}
{{- with .type }}{{ $out = printf "%stype: %s\n" $out (. | quote) }}{{ end -}}
{{- with .annotations }}{{ $out = printf "%sannotations:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- with .labels }}{{ $out = printf "%slabels:\n%s\n" $out (toYaml . | indent 2) }}{{ end -}}
{{- $out -}}
{{- end }}

{{/*
Render common component fields (enable, replicas, WorkloadConfig, service).
*/}}
{{- define "supabase-project.renderComponent" -}}
{{- $out := printf "enable: %t\n" .enable -}}
{{- with .replicas }}{{ $out = printf "%sreplicas: %d\n" $out (int .) }}{{ end -}}
{{- $workload := include "supabase-project.renderWorkloadConfig" . -}}
{{- if $workload }}{{ $out = printf "%s%s" $out $workload }}{{ end -}}
{{- with .service -}}
{{- $service := include "supabase-project.renderService" . -}}
{{- if $service }}{{ $out = printf "%sservice:\n%s" $out (trimSuffix "\n" (indent 2 $service)) }}{{ end -}}
{{- end -}}
{{- $out -}}
{{- end }}
