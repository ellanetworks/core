{{- define "ella-core.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ella-core.fullname" -}}
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

{{- define "ella-core.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ella-core.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ella-core.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "ella-core.labels" -}}
helm.sh/chart: {{ include "ella-core.chart" . }}
{{ include "ella-core.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/component: core
app.kubernetes.io/part-of: ella-networks
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "ella-core.ha" -}}
{{- if gt (len .Values.nodes) 1 }}true{{ end }}
{{- end }}

{{- define "ella-core.nodeName" -}}
{{- if include "ella-core.ha" .root }}
{{- printf "%s-%d" (include "ella-core.fullname" .root) .ordinal | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- include "ella-core.fullname" .root }}
{{- end }}
{{- end }}

{{- define "ella-core.headlessServiceName" -}}
{{- printf "%s-headless" (include "ella-core.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ella-core.networkAttachmentName" -}}
{{- default (printf "%s-ran" (include "ella-core.fullname" .)) .Values.ran.networkAttachment.name }}
{{- end }}

{{- define "ella-core.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ella-core.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "ella-core.image" -}}
{{- $tag := printf "%s%s" (default .Chart.AppVersion .Values.image.tag) .Values.image.tagSuffix }}
{{- if .Values.image.digest }}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest }}
{{- else }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}
{{- end }}

{{- define "ella-core.openshiftSCC" -}}
{{- $mode := toString .Values.openshift.securityContextConstraints.create }}
{{- if or (eq $mode "true") (and (eq $mode "auto") (.Capabilities.APIVersions.Has "security.openshift.io/v1/SecurityContextConstraints")) }}true{{ end }}
{{- end }}

{{- define "ella-core.config" -}}
{{- $root := .root }}
{{- $base := dict
  "db" (dict "path" "/core/data/ella.db")
  "interfaces" (dict
    "n2" (dict "name" $root.Values.ran.interface)
    "n3" (dict "name" $root.Values.ran.interface)
    "n6" (dict "name" "eth0")
    "api" (dict "address" "0.0.0.0" "port" 5002))
}}
{{- if include "ella-core.ha" $root }}
{{- $advertise := printf "%s-0.%s.%s.svc.%s:7000" .name (include "ella-core.headlessServiceName" $root) $root.Release.Namespace $root.Values.clusterDomain }}
{{- $_ := set $base "cluster" (dict "bind-address" "0.0.0.0:7000" "advertise-address" $advertise) }}
{{- end }}
{{- if $root.Values.api.tls.secretName }}
{{- $_ := set $base.interfaces.api "tls" (dict "cert" "/tls/api/tls.crt" "key" "/tls/api/tls.key") }}
{{- end }}
{{- toYaml (mustMergeOverwrite $base (deepCopy $root.Values.config)) }}
{{- end }}

{{- define "ella-core.apiScheme" -}}
{{- if .Values.api.tls.secretName }}HTTPS{{ else }}HTTP{{ end }}
{{- end }}
