{{/*
Expand the name of the chart.
*/}}
{{- define "agentlens.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "agentlens.fullname" -}}
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
Create chart label.
*/}}
{{- define "agentlens.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "agentlens.labels" -}}
helm.sh/chart: {{ include "agentlens.chart" . }}
{{ include "agentlens.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "agentlens.selectorLabels" -}}
app.kubernetes.io/name: {{ include "agentlens.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "agentlens.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "agentlens.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Multi-replica guard: SQLite is single-writer only.
*/}}
{{- if and (gt (int .Values.replicaCount) 1) (eq .Values.database.dialect "sqlite") }}
{{- fail "replicaCount > 1 is not supported with SQLite (single-writer). Set database.dialect=postgres to scale horizontally." }}
{{- end }}
