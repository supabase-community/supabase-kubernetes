{{/*
Match labels for a component
Usage: {{ include "supabase.component.matchLabels" (dict "root" $ "component" "analytics") }}
*/}}
{{- define "supabase.component.matchLabels" -}}
{{- $root := .root -}}
{{- $component := .component -}}

app.kubernetes.io/name: {{ include "supabase.component.name" (dict "root" $root "component" $component) }}
app.kubernetes.io/instance: {{ $root.Release.Name }}
{{- end }}
