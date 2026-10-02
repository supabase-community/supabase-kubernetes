{{/*
Expand the name of the chart.
*/}}
{{- define "supabase-project.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Match internal/helper.ResourceName for the operator-managed Envoy Service.
*/}}
{{- define "supabase-project.envoyServiceName" -}}
{{- $original := printf "%s-envoy" (include "supabase-project.fullname" .) -}}
{{- $normalized := replace "." "-" $original -}}
{{- if regexMatch "^[0-9]" $normalized -}}
{{- $normalized = printf "c-%s" $normalized -}}
{{- end -}}
{{- if and (le (len $normalized) 63) (eq $normalized $original) -}}
{{- $normalized -}}
{{- else -}}
{{- printf "%s-%s" (regexReplaceAll "-+$" (trunc 46 $normalized) "") (trunc 16 (sha256sum $original)) -}}
{{- end -}}
{{- end -}}

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
Namespace for generated references.
Always uses the Helm release namespace.
*/}}
{{- define "supabase-project.namespaceName" -}}
{{- .Release.Namespace }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "supabase-project.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
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
