{{/*
Expand the name of the chart.
*/}}
{{- define "supabase.vector.name" -}}
{{- default (print .Chart.Name "-vector") .Values.deployment.vector.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase.vector.fullname" -}}
{{- if .Values.deployment.vector.fullnameOverride }}
{{- .Values.deployment.vector.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (print .Chart.Name "-vector") .Values.deployment.vector.nameOverride }}
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
{{- define "supabase.vector.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase.vector.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "supabase.vector.serviceAccountName" -}}
{{- if .Values.serviceAccount.vector.create }}
{{- default (include "supabase.vector.fullname" .) .Values.serviceAccount.vector.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.vector.name }}
{{- end }}
{{- end }}
