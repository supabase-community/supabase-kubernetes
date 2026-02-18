{{/*
Return the Service name for a component
Usage: {{ include "supabase.service.name" (dict "root" $ "component" "analytics") }}
*/}}
{{- define "supabase.service.name" -}}
{{- include "supabase.component.fullname" (dict "root" .root "component" .component) -}}
{{- end }}

{{/*
Generic Service for a component
Usage: {{ include "supabase.service" (dict "root" $ "component" "analytics") }}
*/}}
{{- define "supabase.service" -}}
{{- $root := .root -}}
{{- $component := .component -}}
{{- $svc := index $root.Values.service $component -}}

apiVersion: v1
kind: Service
metadata:
  name: {{ include "supabase.component.fullname" (dict "root" $root "component" $component) }}
  labels:
    {{- include "supabase.labels" (dict "root" $root "component" $component) | nindent 4 }}
spec:
  type: {{ $svc.type }}
  ports:
    - name: {{ $svc.portName }}
      port: {{ $svc.port }}
      targetPort: {{ $svc.targetPort }}
      protocol: {{ $svc.protocol }}
  selector:
    {{- include "supabase.component.matchLabels" (dict "root" $root "component" $component) | nindent 4 }}
{{- end }}
