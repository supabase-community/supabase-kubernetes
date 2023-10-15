{{/*
Expand the name of the chart.
*/}}
{{- define "supabase.functions.name" -}}
{{- default (print .Chart.Name "-functions") .Values.functions.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase.functions.fullname" -}}
{{- if .Values.functions.fullnameOverride }}
{{- .Values.functions.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (print .Chart.Name "-functions") .Values.functions.nameOverride }}
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
{{- define "supabase.functions.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase.functions.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "supabase.functions.serviceAccountName" -}}
{{- if .Values.functions.serviceAccount.create }}
{{- default (include "supabase.functions.fullname" .) .Values.functions.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.functions.serviceAccount.name }}
{{- end }}
{{- end }}
