{{/*
Expand the name of the chart.
*/}}
{{- define "pathfinder-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "pathfinder-operator.fullname" -}}
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
{{- define "pathfinder-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "pathfinder-operator.labels" -}}
helm.sh/chart: {{ include "pathfinder-operator.chart" . }}
{{ include "pathfinder-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "pathfinder-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pathfinder-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "pathfinder-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "pathfinder-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{- define "pathfinder-operator.tag" -}}
{{- if .Values.image.tag }}
{{- .Values.image.tag }}
{{- else if .Values.tracing.enabled }}
{{- "otel-" }}{{ .Values.version | default .Chart.AppVersion }}
{{- else }}
{{- .Values.version | default .Chart.AppVersion }}
{{- end }}
{{- end }}
