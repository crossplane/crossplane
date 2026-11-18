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
Find the value of a --namespace or -n flag in a list of command-line arguments.
Empty if the flag isn't set.
*/}}
{{- define "crossplane.namespaceArg" -}}
{{- $namespace := "" -}}
{{- $nextIsNamespace := false -}}
{{- range $value := . -}}
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
{{- else if hasPrefix "-n" $arg -}}
{{- $namespace = trimPrefix "-n" $arg -}}
{{- end -}}
{{- end -}}
{{- $namespace -}}
{{- end -}}

{{/*
Resolve the namespace in which core Crossplane creates package runtimes the
way core does: an explicit --namespace argument, then a POD_NAMESPACE override
in extraEnvVarsCrossplane. Empty when core uses its own namespace, which the
RBAC manager also gets from POD_NAMESPACE. References to core's extra
environment variables are expanded here because the RBAC manager doesn't have
them.
*/}}
{{- define "crossplane.packageRuntimeNamespace" -}}
{{- $env := dict -}}
{{- range $key, $value := .Values.extraEnvVarsCrossplane -}}
{{- $_ := set $env ($key | replace "." "_") (toString $value) -}}
{{- end -}}
{{- $namespace := include "crossplane.namespaceArg" .Values.args -}}
{{- if and (not $namespace) (hasKey $env "POD_NAMESPACE") -}}
{{- $namespace = get $env "POD_NAMESPACE" -}}
{{- end -}}
{{- range $key, $value := $env -}}
{{- $namespace = replace (printf "$(%s)" $key) $value $namespace -}}
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
