{{/*
Expand the name of the chart.
*/}}
{{- define "supabase.db.name" -}}
{{- default (print .Chart.Name "-db") .Values.deployment.db.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase.db.fullname" -}}
{{- if .Values.deployment.db.fullnameOverride }}
{{- .Values.deployment.db.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (print .Chart.Name "-db") .Values.deployment.db.nameOverride }}
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
{{- define "supabase.db.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase.db.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "supabase.db.serviceAccountName" -}}
{{- if .Values.serviceAccount.db.create }}
{{- default (include "supabase.db.fullname" .) .Values.serviceAccount.db.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.db.name }}
{{- end }}
{{- end }}
