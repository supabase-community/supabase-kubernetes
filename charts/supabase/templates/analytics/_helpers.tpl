{{/*
Expand the name of the chart.
*/}}
{{- define "supabase.analytics.name" -}}
{{- default (print .Chart.Name "-analytics") .Values.analytics.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase.analytics.fullname" -}}
{{- if .Values.analytics.fullnameOverride }}
{{- .Values.analytics.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (print .Chart.Name "-analytics") .Values.analytics.nameOverride }}
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
{{- define "supabase.analytics.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase.analytics.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "supabase.analytics.serviceAccountName" -}}
{{- if .Values.analytics.serviceAccount.create }}
{{- default (include "supabase.analytics.fullname" .) .Values.analytics.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.analytics.serviceAccount.name }}
{{- end }}
{{- end }}
