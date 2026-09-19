{{/*
Chart name.
*/}}
{{- define "gophprofile.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified app name.
*/}}
{{- define "gophprofile.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "gophprofile.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "gophprofile.labels" -}}
helm.sh/chart: {{ include "gophprofile.chart" . }}
{{ include "gophprofile.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "gophprofile.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gophprofile.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Component-scoped names/labels, e.g. "server", "worker", "postgresql".
*/}}
{{- define "gophprofile.componentName" -}}
{{- printf "%s-%s" (include "gophprofile.fullname" .context) .component -}}
{{- end -}}

{{- define "gophprofile.componentLabels" -}}
{{ include "gophprofile.labels" .context }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- define "gophprofile.componentSelectorLabels" -}}
{{ include "gophprofile.selectorLabels" .context }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- define "gophprofile.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{ default (include "gophprofile.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
{{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Name of the Secret holding app credentials (own Secret, or user-supplied existingSecret).
*/}}
{{- define "gophprofile.secretName" -}}
{{- default (printf "%s-app" (include "gophprofile.fullname" .)) .Values.secrets.existingSecret -}}
{{- end -}}

{{/*
Postgres/MinIO/Kafka/OTel/Loki/Jaeger host:port helpers, so the app's
ConfigMap and the infra Services stay in lock-step.
*/}}
{{- define "gophprofile.postgresHost" -}}
{{ include "gophprofile.fullname" . }}-postgresql
{{- end -}}

{{- define "gophprofile.minioHost" -}}
{{ include "gophprofile.fullname" . }}-minio
{{- end -}}

{{- define "gophprofile.kafkaBootstrap" -}}
{{ include "gophprofile.fullname" . }}-kafka:9092
{{- end -}}

{{- define "gophprofile.otelEndpoint" -}}
http://{{ include "gophprofile.fullname" . }}-otel-collector:4318
{{- end -}}
