{{/*
High Availability helper templates
*/}}

{{/*
Get replica count for a service based on HA settings
*/}}
{{- define "supabase.ha.replicaCount" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $global := .global -}}
{{- if and $global.highAvailability.enabled $service.highAvailability.enabled -}}
{{- $service.highAvailability.minReplicas | default $global.highAvailability.minReplicas -}}
{{- else -}}
{{- $service.replicaCount | default 1 -}}
{{- end -}}
{{- end -}}

{{/*
Generate anti-affinity rules for HA services
*/}}
{{- define "supabase.ha.antiAffinity" -}}
{{- $service := .service -}}
{{- $serviceName := .serviceName -}}
{{- $global := .global -}}
{{- if and $global.highAvailability.enabled $service.highAvailability.enabled $service.highAvailability.antiAffinity.enabled -}}
podAntiAffinity:
  {{- if eq $service.highAvailability.antiAffinity.type "hard" }}
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchExpressions:
      - key: app.kubernetes.io/name
        operator: In
        values: [{{ $serviceName }}]
    topologyKey: kubernetes.io/hostname
  {{- else }}
  preferredDuringSchedulingIgnoredDuringExecution:
  - weight: 100
    podAffinityTerm:
      labelSelector:
        matchExpressions:
        - key: app.kubernetes.io/name
          operator: In
          values: [{{ $serviceName }}]
      topologyKey: kubernetes.io/hostname
  {{- end }}
  {{- if $global.multiZone.enabled }}
  - weight: 50
    podAffinityTerm:
      labelSelector:
        matchExpressions:
        - key: app.kubernetes.io/name
          operator: In
          values: [{{ $serviceName }}]
      topologyKey: topology.kubernetes.io/zone
  {{- end }}
{{- end -}}
{{- end -}}

{{/*
Generate HPA configuration for HA services
*/}}
{{- define "supabase.ha.hpa" -}}
{{- $service := .service -}}
{{- $global := .global -}}
{{- if and $global.highAvailability.enabled $service.highAvailability.enabled -}}
minReplicas: {{ $service.highAvailability.minReplicas | default $global.highAvailability.minReplicas }}
maxReplicas: {{ $global.highAvailability.maxReplicas }}
targetCPUUtilizationPercentage: {{ $global.highAvailability.targetCPUUtilization }}
{{- if $global.highAvailability.targetMemoryUtilization }}
targetMemoryUtilizationPercentage: {{ $global.highAvailability.targetMemoryUtilization }}
{{- end }}
{{- else }}
minReplicas: {{ $service.autoscaling.minReplicas }}
maxReplicas: {{ $service.autoscaling.maxReplicas }}
targetCPUUtilizationPercentage: {{ $service.autoscaling.targetCPUUtilizationPercentage }}
{{- if $service.autoscaling.targetMemoryUtilizationPercentage }}
targetMemoryUtilizationPercentage: {{ $service.autoscaling.targetMemoryUtilizationPercentage }}
{{- end }}
{{- end -}}
{{- end -}}

{{/*
Generate zone spread topology constraints
*/}}
{{- define "supabase.ha.topologySpreadConstraints" -}}
{{- $serviceName := .serviceName -}}
{{- $global := .global -}}
{{- if and $global.highAvailability.enabled $global.multiZone.enabled -}}
topologySpreadConstraints:
- maxSkew: 1
  topologyKey: topology.kubernetes.io/zone
  whenUnsatisfiable: {{ if $global.multiZone.preferZoneSpread }}ScheduleAnyway{{ else }}DoNotSchedule{{ end }}
  labelSelector:
    matchLabels:
      app.kubernetes.io/name: {{ $serviceName }}
{{- end -}}
{{- end -}}
