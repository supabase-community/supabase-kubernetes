{{/*
Expand the name of a component.
Usage: {{ include "supabase.component.name" (dict "root" $ "component" "analytics") }}
*/}}
{{- define "supabase.component.name" -}}
{{- $root := .root -}}
{{- $component := .component -}}

{{- $defaultName := printf "%s-%s" $root.Chart.Name $component -}}
{{- $override := index $root.Values $component "nameOverride" -}}

{{- default $defaultName $override | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{/*
Create a default fully qualified name for a component.
Usage: {{ include "supabase.component.fullname" (dict "root" $ "component" "analytics") }}
*/}}
{{- define "supabase.component.fullname" -}}
{{- $root := .root -}}
{{- $component := .component -}}

{{- $componentValues := index $root.Values $component -}}
{{- $nameOverride := "" -}}
{{- $fullnameOverride := "" -}}

{{- if $componentValues -}}
  {{- $nameOverride = $componentValues.nameOverride -}}
  {{- $fullnameOverride = $componentValues.fullnameOverride -}}
{{- end -}}

{{- if $fullnameOverride -}}
  {{- $fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
  {{- $baseName := default (printf "%s-%s" $root.Chart.Name $component) $nameOverride -}}
  {{- if contains $baseName $root.Release.Name -}}
    {{- $root.Release.Name | trunc 63 | trimSuffix "-" -}}
  {{- else -}}
    {{- printf "%s-%s" $root.Release.Name $baseName | trunc 63 | trimSuffix "-" -}}
  {{- end -}}
{{- end -}}
{{- end }}
