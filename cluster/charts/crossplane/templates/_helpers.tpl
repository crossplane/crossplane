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
Generate basic labels for the given component. Expects a list of the root
context and the value of the app.kubernetes.io/component label.
*/}}
{{- define "crossplane.componentLabels" }}
{{- $root := index . 0 }}
helm.sh/chart: {{ include "crossplane.chart" $root }}
app.kubernetes.io/managed-by: {{ $root.Release.Service }}
app.kubernetes.io/component: {{ index . 1 }}
app.kubernetes.io/part-of: {{ template "crossplane.name" $root }}
app.kubernetes.io/name: {{ include "crossplane.name" $root }}
app.kubernetes.io/instance: {{ $root.Release.Name }}
{{- if $root.Chart.AppVersion }}
app.kubernetes.io/version: {{ $root.Chart.AppVersion | quote }}
{{- end }}
{{- if $root.Values.customLabels }}
{{ toYaml $root.Values.customLabels }}
{{- end }}
{{- end }}

{{/*
Generate basic labels for the Crossplane core controller.
*/}}
{{- define "crossplane.labels" -}}
{{ include "crossplane.componentLabels" (list . "cloud-infrastructure-controller") }}
{{- end }}

{{/*
Generate basic labels for the RBAC manager.
*/}}
{{- define "crossplane.rbacManagerLabels" -}}
{{ include "crossplane.componentLabels" (list . "rbac-manager") }}
{{- end }}
