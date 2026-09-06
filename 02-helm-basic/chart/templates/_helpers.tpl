{{/*
Labels put on every object this chart creates. Defined once here and
pulled into each template with `include`, so adding a label later means
editing one place instead of every file under templates/.
*/}}
{{- define "chart.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels — kept separate from chart.labels since selectors are
immutable on a Deployment and must not pick up churny labels later.
*/}}
{{- define "chart.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
