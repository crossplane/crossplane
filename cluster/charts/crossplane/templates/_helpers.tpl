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
