{{/*
Expand the name of the chart.
*/}}
{{- define "supabase.realtime.name" -}}
{{- default (print .Chart.Name "-realtime") .Values.deployment.realtime.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase.realtime.fullname" -}}
{{- if .Values.deployment.realtime.fullnameOverride }}
{{- .Values.deployment.realtime.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (print .Chart.Name "-realtime") .Values.deployment.realtime.nameOverride }}
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
{{- define "supabase.realtime.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase.realtime.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "supabase.realtime.serviceAccountName" -}}
{{- if .Values.serviceAccount.realtime.create }}
{{- default (include "supabase.realtime.fullname" .) .Values.serviceAccount.realtime.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.realtime.name }}
{{- end }}
{{- end }}
