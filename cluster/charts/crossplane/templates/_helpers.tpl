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
Resolve the namespace in which core Crossplane creates package runtimes from
its existing command-line arguments. Empty unless core Crossplane is
explicitly passed a --namespace, in which case both it and the RBAC manager
default to their own namespace via POD_NAMESPACE.
*/}}
{{- define "crossplane.packageRuntimeNamespace" -}}
{{- $namespace := "" -}}
{{- $nextIsNamespace := false -}}
{{- range $value := .Values.args -}}
{{- $arg := toString $value -}}
{{- if $nextIsNamespace -}}
{{- $namespace = $arg -}}
{{- $nextIsNamespace = false -}}
{{- else if or (eq $arg "--namespace") (eq $arg "-n") -}}
{{- $nextIsNamespace = true -}}
{{- else if hasPrefix "--namespace=" $arg -}}
{{- $namespace = trimPrefix "--namespace=" $arg -}}
{{- else if hasPrefix "-n=" $arg -}}
{{- $namespace = trimPrefix "-n=" $arg -}}
{{- end -}}
{{- end -}}
{{- $namespace -}}
{{- end -}}

{{/*
Generate basic labels
*/}}
{{- define "crossplane.labels" }}
helm.sh/chart: {{ include "crossplane.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: cloud-infrastructure-controller
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

{{/*
Define ExternalSecretStoreEnabled Feature Flag
*/}}
{{- define "crossplane.externalSecretStoresEnabled" -}}
{{- if has "--enable-external-secret-stores" .Values.args -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}
