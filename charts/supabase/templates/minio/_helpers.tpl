{{/*
Expand the name of the chart.
*/}}
{{- define "supabase.minio.name" -}}
{{- default (print .Chart.Name "-minio") .Values.minio.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase.minio.fullname" -}}
{{- if .Values.minio.fullnameOverride }}
{{- .Values.minio.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (print .Chart.Name "-minio") .Values.minio.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "supabase.minio.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase.minio.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "supabase.minio.serviceAccountName" -}}
{{- if .Values.minio.serviceAccount.create }}
{{- default (include "supabase.minio.fullname" .) .Values.minio.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.minio.serviceAccount.name }}
{{- end }}
{{- end }}
