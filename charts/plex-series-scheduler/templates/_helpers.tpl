{{- define "plex-series-scheduler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "plex-series-scheduler.fullname" -}}
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

{{- define "plex-series-scheduler.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "plex-series-scheduler.labels" -}}
helm.sh/chart: {{ include "plex-series-scheduler.chart" . }}
app.kubernetes.io/name: {{ include "plex-series-scheduler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "plex-series-scheduler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "plex-series-scheduler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "plex-series-scheduler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "plex-series-scheduler.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "plex-series-scheduler.configMapName" -}}
{{- printf "%s-config" (include "plex-series-scheduler.fullname" .) -}}
{{- end -}}

{{- define "plex-series-scheduler.commonEnv" -}}
- name: PLEX_TOKEN
  valueFrom:
    secretKeyRef:
      name: {{ .Values.plexTokenSecret.name }}
      key: {{ .Values.plexTokenSecret.key }}
{{- with .Values.extraEnv }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{- define "plex-series-scheduler.commonVolumes" -}}
- name: config
  configMap:
    name: {{ include "plex-series-scheduler.configMapName" . }}
{{- with .Values.extraVolumes }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{- define "plex-series-scheduler.commonVolumeMounts" -}}
- name: config
  mountPath: /config/{{ .Values.configFileName }}
  subPath: {{ .Values.configFileName }}
  readOnly: true
{{- with .Values.extraVolumeMounts }}
{{ toYaml . }}
{{- end }}
{{- end -}}
