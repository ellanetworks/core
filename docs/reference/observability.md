---
description: Reference for observability features in Ella Core.
---

# Observability

Ella Core supports four observability pillars: Metrics, Logs, Traces, and Profiles.

## 1. Metrics

Ella Core exposes [Prometheus](https://prometheus.io/) metrics to monitor the health of an Ella Core instance.

Please refer to the [metrics API documentation](api/metrics.md) for more information on accessing metrics in Ella Core.

### Default Go metrics

These metrics are used to monitor the health of the Go runtime and garbage collector. These metrics start with the `go_` prefix.

### Custom metrics

These metrics are used to monitor the health of the system and the performance of the network. These metrics start with the `app_` prefix. The following custom metrics are exposed by Ella Core:

| Metric | Description    | Type  |
| ------------------- | --------- | --------- |
| app_connected_radios            | Number of radios currently connected to Ella Core                  | Gauge   |
| app_ngap_messages_total | Total number of received NGAP message per type | Counter |
| app_registered_subscribers      | Number of subscribers currently registered in Ella Core            | Gauge   |
| app_registration_attempts_total | Total number of UE registration attempts by type and result | Counter |
| app_pdu_sessions_total | Number of PDU sessions currently in Ella Core. | Gauge |
| app_pdu_session_establishment_attempts_total | Total PDU session establishment attempts by result | Counter |
| app_ip_addresses_allocated_total | The total number of IP addresses currently allocated to subscribers. | Gauge |
| app_ip_addresses_total | The total number of IP addresses available for subscribers. | Gauge |
| app_xdp_action_total | The total number of packets, with labels for the interface (n3, n6), and action taken. | Counter |
| app_uplink_bytes | The total number of bytes transmitted in the uplink direction (N3 -> N6). This value includes the Ethernet header. | Counter |
| app_downlink_bytes | The total number of bytes transmitted in the downlink direction (N6 -> N3). This value includes the Ethernet header. | Counter |
| app_api_requests_total                | Total number of HTTP requests by method, endpoint, and status code | Counter |
| app_api_request_duration_seconds      | HTTP request duration histogram in seconds    | Histogram |
| app_api_authentication_attempts_total | Total number of authentication attempts by type and result         | Counter |
| app_database_storage_bytes | The total storage used by the database in bytes. This is the size of the database file on disk. | Gauge |
| app_database_queries_total | Total number of database queries by table and operation | Counter |
| app_database_query_duration_seconds | Duration of database queries | Histogram |

## 2. Logs

Ella Core produces three types of logs:

 - **System Logs**: General operational information about the system.
 - **Audit Logs**: Logs of user actions for security and compliance. You can view audit logs and manage their retention via the [API](api/audit_logs.md) and the Web UI. 
 - **Radio Logs**: Logs related to NGAP messages. You can view radio logs and manage their retention via the [API](api/radios.md) and the Web UI. 

All logs are output in **JSON format** with structured fields for easy parsing and ingestion into log aggregation systems like Loki, Elasticsearch, or Splunk.

For more information on configuring logging in Ella Core, refer to the [Configuration File](config_file.md) documentation.

!!! Note
    Ella Core does not assist with log rotation; we recommend using a log rotation tool to manage log files.


## 3. Traces

Ella Core supports distributed tracing using [OpenTelemetry](https://opentelemetry.io/). Traces are exported via [OTLP (gRPC)](https://opentelemetry.io/docs/specs/otlp/) to any compatible backend such as Jaeger, Tempo, or Honeycomb.

Traces are collected for the following components:

 - **NGAP**: Traces for NGAP message handling between gNodeBs and Ella Core.
 - **API**: Traces for HTTP requests to the REST API.

For more information on configuring tracing in Ella Core, refer to the [Configuration File](config_file.md) documentation.

## 4. Profiles

Ella Core exposes the [http/pprof](https://pkg.go.dev/net/http/pprof) API for CPU and memory profiling analysis. This allows users to collect and analyze profiles of Ella Core using visualization tools like [pprof](https://pkg.go.dev/net/http/pprof) or [pyroscope](https://grafana.com/oss/pyroscope/).

For more information on accessing the pprof API in Ella Core, refer to the [pprof API documentation](api/pprof.md).

## Dashboards

Ella Core ships with [Grafana](https://grafana.com/) dashboards that you can import using the Dashboard IDs provided below.

### Network Health 

This dashboard uses Prometheus metrics to provide real-time visibility into all aspects of your 5G private network deployment, from radio connectivity and subscriber sessions to system performance and data plane throughput.

<figure markdown="span">
  ![Network Health Dashboard](../images/dashboard_network_health.png){ width="800" }
  <figcaption>Grafana dashboard for Network Health.</figcaption>
</figure>

- Data Sources: Prometheus
- Dashboard ID: 24751
- View online: [grafana.com/grafana/dashboards/24751/](https://grafana.com/grafana/dashboards/24751/)

### Deep Dive (for developers)

This dashboard uses metrics, logs, traces, and profiles to provide deep insights into the internal workings of Ella Core. It is intended for developers and advanced users who want to understand the performance and behavior of Ella Core at a granular level. We recommend running Grafana Alloy to collect all signals ([example configuration file](https://github.com/ellanetworks/core/tree/main/grafana)).

<figure markdown="span">
  ![Deep Dive Dashboard](../images/dashboard_deep_dive.png){ width="800" }
  <figcaption>Grafana dashboard for Deep Dive.</figcaption>
</figure>

- Data Sources: Mimir, Loki, Tempo, Pyroscope
- Dashboard ID: 24770
- View online: [grafana.com/grafana/dashboards/24770/](https://grafana.com/grafana/dashboards/24770/)
