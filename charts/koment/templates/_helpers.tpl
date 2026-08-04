{{- define "koment.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "koment.testImage" -}}
{{- if .Values.tests.image.digest -}}
{{- printf "%s@%s" .Values.tests.image.repository .Values.tests.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.tests.image.repository .Values.tests.image.tag -}}
{{- end -}}
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
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}
{{- end -}}

{{- define "koment.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "koment.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- required "serviceAccount.name is required when creation is disabled" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "koment.args" -}}
{{- $args := list "serve" "--config" "/config/repositories.yaml" "--listen" (printf "0.0.0.0:%d" (int .Values.service.port)) "--sync-interval" .Values.syncInterval -}}
{{- if .Values.metrics.enabled -}}
{{- $args = concat $args (list "--metrics" (printf "0.0.0.0:%d" (int .Values.metrics.port))) -}}
{{- end -}}
{{- if .Values.github.existingSecret -}}
{{- $args = concat $args (list "--github-token-file" (printf "/secrets/github/%s" .Values.github.tokenKey)) -}}
{{- end -}}
{{- if .Values.auth.existingSecret -}}
{{- $args = concat $args (list "--credentials-file" (printf "/secrets/auth/%s" .Values.auth.credentialsKey)) -}}
{{- end -}}
{{- if .Values.auth.trustedProxies -}}
{{- $args = concat $args (list "--trusted-proxies" (join "," .Values.auth.trustedProxies)) -}}
{{- end -}}
{{- if .Values.auth.humanWrites -}}
{{- $args = append $args "--human-writes" -}}
{{- end -}}
{{- $args | toJson -}}
{{- end -}}
