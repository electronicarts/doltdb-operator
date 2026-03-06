{{/*
Overlays will be merged with controller-gen generated yaml.
*/}}


{{- define "doltdb-operator.clusterRoleOverlay" -}}
metadata:
  name: {{ include "doltdb-operator.fullname" . }}
  labels:
    {{- include "doltdb-operator.labels" . | nindent 4 }}
{{- end }}

{{- define "doltdb-operator.crdOverlay" -}}
metadata:
  labels:
    {{- include "doltdb-operator.labels" . | nindent 4 }}
{{- if .Values.keepCrds }}
  annotations:
    helm.sh/resource-policy: keep
{{- end }}
{{- end }}
