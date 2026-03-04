{{/*
Expand the name of the chart.
*/}}
{{- define "nexus.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "nexus.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "nexus.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "nexus.labels" -}}
helm.sh/chart: {{ include "nexus.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: nexus
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end }}

{{/*
nexus-core labels
*/}}
{{- define "nexus.core.labels" -}}
{{ include "nexus.labels" . }}
app.kubernetes.io/name: {{ include "nexus.name" . }}-core
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: core
{{- end }}

{{/*
nexus-core selector labels
*/}}
{{- define "nexus.core.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nexus.name" . }}-core
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
nexus-ai-operators labels
*/}}
{{- define "nexus.operator.labels" -}}
{{ include "nexus.labels" . }}
app.kubernetes.io/name: {{ include "nexus.name" . }}-ai-operators
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: operator
{{- end }}

{{/*
nexus-ai-operators selector labels
*/}}
{{- define "nexus.operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nexus.name" . }}-ai-operators
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
nexus-tower labels
*/}}
{{- define "nexus.tower.labels" -}}
{{ include "nexus.labels" . }}
app.kubernetes.io/name: {{ include "nexus.name" . }}-tower
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: tower
{{- end }}

{{/*
nexus-tower selector labels
*/}}
{{- define "nexus.tower.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nexus.name" . }}-tower
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
postgres labels
*/}}
{{- define "nexus.postgres.labels" -}}
{{ include "nexus.labels" . }}
app.kubernetes.io/name: {{ include "nexus.name" . }}-postgres
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: postgres
{{- end }}

{{/*
postgres selector labels
*/}}
{{- define "nexus.postgres.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nexus.name" . }}-postgres
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
redis labels
*/}}
{{- define "nexus.redis.labels" -}}
{{ include "nexus.labels" . }}
app.kubernetes.io/name: {{ include "nexus.name" . }}-redis
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: redis
{{- end }}

{{/*
redis selector labels
*/}}
{{- define "nexus.redis.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nexus.name" . }}-redis
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Image helpers - construct the full image reference for each component.
*/}}
{{- define "nexus.core.image" -}}
{{- $registry := .Values.global.imageRegistry -}}
{{- $repo := .Values.core.image.repository -}}
{{- $tag := .Values.core.image.tag | default .Chart.AppVersion -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repo $tag }}
{{- else }}
{{- printf "%s:%s" $repo $tag }}
{{- end }}
{{- end }}

{{- define "nexus.operator.image" -}}
{{- $registry := .Values.global.imageRegistry -}}
{{- $repo := .Values.operator.image.repository -}}
{{- $tag := .Values.operator.image.tag | default .Chart.AppVersion -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repo $tag }}
{{- else }}
{{- printf "%s:%s" $repo $tag }}
{{- end }}
{{- end }}

{{- define "nexus.tower.image" -}}
{{- $registry := .Values.global.imageRegistry -}}
{{- $repo := .Values.tower.image.repository -}}
{{- $tag := .Values.tower.image.tag | default .Chart.AppVersion -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repo $tag }}
{{- else }}
{{- printf "%s:%s" $repo $tag }}
{{- end }}
{{- end }}

{{/*
ConfigMap and Secret resource names
*/}}
{{- define "nexus.configmapName" -}}
{{ include "nexus.fullname" . }}-config
{{- end }}

{{- define "nexus.secretName" -}}
{{ include "nexus.fullname" . }}-secret
{{- end }}

{{/*
Service names used for inter-component communication
*/}}
{{- define "nexus.core.serviceName" -}}
{{ include "nexus.fullname" . }}-core
{{- end }}

{{- define "nexus.operator.serviceName" -}}
{{ include "nexus.fullname" . }}-ai-operators
{{- end }}

{{- define "nexus.tower.serviceName" -}}
{{ include "nexus.fullname" . }}-tower
{{- end }}

{{- define "nexus.postgres.serviceName" -}}
{{ include "nexus.fullname" . }}-postgres
{{- end }}

{{- define "nexus.redis.serviceName" -}}
{{ include "nexus.fullname" . }}-redis
{{- end }}

{{/*
imagePullSecrets helper
*/}}
{{- define "nexus.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}
