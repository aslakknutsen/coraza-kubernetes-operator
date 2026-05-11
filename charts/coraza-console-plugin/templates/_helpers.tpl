{{/*
Expand the name of the chart.
*/}}
{{- define "coraza-console-plugin.name" -}}
{{- default (default .Chart.Name .Release.Name) .Values.plugin.name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "coraza-console-plugin.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "coraza-console-plugin.labels" -}}
helm.sh/chart: {{ include "coraza-console-plugin.chart" . }}
{{ include "coraza-console-plugin.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "coraza-console-plugin.selectorLabels" -}}
app: {{ include "coraza-console-plugin.name" . }}
app.kubernetes.io/name: {{ include "coraza-console-plugin.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: {{ include "coraza-console-plugin.name" . }}
{{- end }}

{{/*
Create the name of the certificate secret
*/}}
{{- define "coraza-console-plugin.certificateSecret" -}}
{{ default (printf "%s-cert" (include "coraza-console-plugin.name" .)) .Values.plugin.certificateSecretName }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "coraza-console-plugin.serviceAccountName" -}}
{{- if .Values.plugin.serviceAccount.create }}
{{- default (include "coraza-console-plugin.name" .) .Values.plugin.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.plugin.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the patcher
*/}}
{{- define "coraza-console-plugin.patcherName" -}}
{{- printf "%s-patcher" (include "coraza-console-plugin.name" .) }}
{{- end }}

{{/*
Create the name of the patcher service account
*/}}
{{- define "coraza-console-plugin.patcherServiceAccountName" -}}
{{- if .Values.plugin.patcherServiceAccount.create }}
{{- default (printf "%s-patcher" (include "coraza-console-plugin.name" .)) .Values.plugin.patcherServiceAccount.name }}
{{- else }}
{{- default "default" .Values.plugin.patcherServiceAccount.name }}
{{- end }}
{{- end }}
