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

{{/*
Render a standardized DB host env var.
Usage: include "supabase.db.hostEnv" (dict "root" . "name" "DB_HOST")
*/}}
{{- define "supabase.db.hostEnv" -}}
{{- $root := .root -}}
- name: {{ .name }}
  {{- if $root.Values.deployment.db.enabled }}
  value: {{ include "supabase.db.fullname" $root | quote }}
  {{- else if and $root.Values.secret.db.secretRef (hasKey (default (dict) $root.Values.secret.db.secretRefKey) "host") }}
  valueFrom:
    secretKeyRef:
      name: {{ include "supabase.secret.db.name" $root }}
      key: {{ include "supabase.secret.db.key.host" $root }}
  {{- else if $root.Values.secret.db.host }}
  value: {{ $root.Values.secret.db.host | quote }}
  {{- else }}
  value: {{ required "secret.db.host must be set when deployment.db.enabled is false" $root.Values.secret.db.host | quote }}
  {{- end }}
{{- end }}

{{/*
Render a standardized DB port env var.
Usage: include "supabase.db.portEnv" (dict "root" . "name" "DB_PORT")
*/}}
{{- define "supabase.db.portEnv" -}}
{{- $root := .root -}}
- name: {{ .name }}
  {{- if and (not $root.Values.deployment.db.enabled) $root.Values.secret.db.secretRef (hasKey (default (dict) $root.Values.secret.db.secretRefKey) "port") }}
  valueFrom:
    secretKeyRef:
      name: {{ include "supabase.secret.db.name" $root }}
      key: {{ include "supabase.secret.db.key.port" $root }}
  {{- else if and (not $root.Values.deployment.db.enabled) $root.Values.secret.db.port }}
  value: {{ $root.Values.secret.db.port | quote }}
  {{- else if $root.Values.deployment.db.enabled }}
  value: {{ $root.Values.service.db.port | quote }}
  {{- else }}
  value: {{ required "secret.db.port must be set when deployment.db.enabled is false" $root.Values.secret.db.port | quote }}
  {{- end }}
{{- end }}
