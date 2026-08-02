{{- define "koment.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "koment.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "koment.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "koment.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "koment.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "koment.selectorLabels" -}}
app.kubernetes.io/name: {{ include "koment.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "koment.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}

{{/*
Only read-only surfaces are offered. ADR 0011 ties the no-authentication
posture to the surface staying read-only, and this chart ships no auth, so a
writable mode must not become reachable by setting a value.
*/}}
{{- define "koment.args" -}}
{{- $port := printf "0.0.0.0:%d" (int .Values.service.port) -}}
{{- $metrics := list -}}
{{- if .Values.metrics.enabled -}}
{{- $metrics = list "--metrics" (printf "0.0.0.0:%d" (int .Values.metrics.port)) -}}
{{- end -}}
{{- if eq .Values.mode "ui" -}}
{{ concat (list "ui" "--listen" $port) $metrics | toJson }}
{{- else if eq .Values.mode "mcp-http" -}}
{{ concat (list "mcp" "--http" $port) $metrics | toJson }}
{{- else if eq .Values.mode "mcp-streamable" -}}
{{ concat (list "mcp" "--streamable-http" $port) $metrics | toJson }}
{{- else -}}
{{- fail (printf "mode %q is not one of ui, mcp-http, mcp-streamable" .Values.mode) -}}
{{- end -}}
{{- end -}}
