{{/*
Expand the name of the chart.
*/}}
{{- define "supabase.imgproxy.name" -}}
{{- default (print .Chart.Name "-imgproxy") .Values.imgproxy.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "supabase.imgproxy.fullname" -}}
{{- if .Values.imgproxy.fullnameOverride }}
{{- .Values.imgproxy.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (print .Chart.Name "-imgproxy") .Values.imgproxy.nameOverride }}
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
{{- define "supabase.imgproxy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supabase.imgproxy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "supabase.imgproxy.serviceAccountName" -}}
{{- if .Values.imgproxy.serviceAccount.create }}
{{- default (include "supabase.imgproxy.fullname" .) .Values.imgproxy.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.imgproxy.serviceAccount.name }}
{{- end }}
{{- end }}
