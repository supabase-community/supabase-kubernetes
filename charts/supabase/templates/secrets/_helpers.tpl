{{/*
Expand the name of the JWT secret.
*/}}
{{- define "supabase.secret.jwt" -}}
{{- printf "%s-jwt" (include "supabase.fullname" .) }}
{{- end -}}

{{/*
Expand the name of the SMTP secret.
*/}}
{{- define "supabase.secret.smtp" -}}
{{- printf "%s-smtp" (include "supabase.fullname" .) }}
{{- end -}}

{{/*
Expand the name of the dashboard secret.
*/}}
{{- define "supabase.secret.dashboard" -}}
{{- printf "%s-dashboard" (include "supabase.fullname" .) }}
{{- end -}}

{{/*
Expand the name of the database secret.
*/}}
{{- define "supabase.secret.db" -}}
{{- printf "%s-db" (include "supabase.fullname" .) }}
{{- end -}}

{{/*
Expand the name of the analytics secret.
*/}}
{{- define "supabase.secret.analytics" -}}
{{- printf "%s-analytics" (include "supabase.fullname" .) }}
{{- end -}}

{{/*
Expand the name of the s3 secret.
*/}}
{{- define "supabase.secret.s3" -}}
{{- printf "%s-s3" (include "supabase.fullname" .) }}
{{- end -}}
