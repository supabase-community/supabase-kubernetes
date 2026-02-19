{{/*
Expand the name of the chart.
*/}}
{{- define "supabase.rest.name" -}}
{{- default (print .Chart.Name "-rest") .Values.deployment.rest.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase.rest.fullname" -}}
{{- if .Values.deployment.rest.fullnameOverride }}
{{- .Values.deployment.rest.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (print .Chart.Name "-rest") .Values.deployment.rest.nameOverride }}
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
{{- define "supabase.rest.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase.rest.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "supabase.rest.serviceAccountName" -}}
{{- if .Values.serviceAccount.rest.create }}
{{- default (include "supabase.rest.fullname" .) .Values.serviceAccount.rest.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.rest.name }}
{{- end }}
{{- end }}
