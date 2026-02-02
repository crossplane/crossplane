{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "crossplane.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "crossplane.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{/*
This helper allows setting app.kubernetes.io/component dynamically.
instead of hardcoding it for all Deployments.
Usage:
  {{- include "crossplane.componentLabel" "rbac-manager" | nindent 4 }}
*/}}
{{- define "crossplane.componentLabel" -}}
app.kubernetes.io/component: {{ . | quote }}
{{- end -}}
{{/*
Generate basic labels
*/}}
{{- define "crossplane.labels" }}
helm.sh/chart: {{ include "crossplane.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: {{ template "crossplane.name" . }}
app.kubernetes.io/name: {{ include "crossplane.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- if .Values.customLabels }}
{{ toYaml .Values.customLabels }}
{{- end }}
{{- end }}
