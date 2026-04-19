{{/*
Return the PVC name for a given workload.
If persistence.existingClaim is set, use it.
Otherwise generate the default name.
Usage: {{ include "supabase.pvc.name" (dict "root" $ "name" "alias") }}
*/}}
{{- define "supabase.pvc.name" -}}
{{- $root := .root -}}
{{- $name := .name -}}
{{- $persistence := index $root.Values.persistence $name -}}

{{- if $persistence.existingClaim -}}
{{- $persistence.existingClaim -}}
{{- else -}}
{{- printf "%s-%s" (include "supabase.fullname" $root) $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create a PVC for a given workload name.
Usage: {{ include "supabase.pvc" (dict "root" $ "name" "workload" "enabled": "true") }}
*/}}
{{- define "supabase.pvc" -}}
{{- $root := .root -}}
{{- $name := .name -}}
{{- $enabled := .enabled -}}
{{- $persistence := index $root.Values.persistence $name -}}

{{- if and $enabled $persistence.enabled (not $persistence.existingClaim) -}}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ include "supabase.pvc.name" (dict "root" $root "name" $name) }}
  labels:
    {{- include "supabase.labels" $root | nindent 4 }}
  {{- with $persistence.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if $persistence.storageClassName }}
  storageClassName: {{ $persistence.storageClassName }}
  {{- end }}
  accessModes:
    {{- range $persistence.accessModes }}
    - {{ . | quote }}
    {{- end }}
  resources:
    requests:
      storage: {{ $persistence.size | quote }}
{{- end }}
{{- end }}
