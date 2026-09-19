# Observability

Ella Core supports four observability pillars: Metrics, Logs, Traces, and Profiles.

## 1. Metrics

Ella Core exposes [Prometheus](https://prometheus.io/) metrics to monitor the health of an Ella Core instance.

Please refer to the [metrics API documentation](https://docs.ellanetworks.com/reference/api/metrics/index.md) for more information on accessing metrics in Ella Core.

### Default Go metrics

These metrics are used to monitor the health of the Go runtime and garbage collector. These metrics start with the `go_` prefix.

### Custom metrics

These metrics are used to monitor the health of the system and the performance of your private network. These metrics start with the `app_` prefix. The following custom metrics are exposed by Ella Core:

| Metric                                   | Description                                                                                                                      | Type      |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | --------- |
| app_connected_radios                     | Number of radios currently connected to Ella Core, labeled by `rat`.                                                             | Gauge     |
| app_signaling_messages_total             | Total radio signaling messages, labeled by `rat`, `direction`, and `type`.                                                       | Counter   |
| app_registered_subscribers               | Number of subscribers currently registered in Ella Core, labeled by `rat`.                                                       | Gauge     |
| app_registration_attempts_total          | Total UE registration (5G) and attach/tracking-area-update (4G) attempts, labeled by `rat`, `type`, and `result`.                | Counter   |
| app_sessions                             | Number of active sessions currently in Ella Core, labeled by `rat`.                                                              | Gauge     |
| app_session_establishment_attempts_total | Total session establishment attempts, labeled by `rat` and `result`.                                                             | Counter   |
| app_ip_addresses_allocated               | The total number of IP addresses currently allocated to subscribers.                                                             | Gauge     |
| app_ip_addresses                         | The total number of IP addresses available for subscribers.                                                                      | Gauge     |
| app_upf_datapath_forward_total           | Packets the data plane forwarded, labeled by `direction` and `action`.                                                           | Counter   |
| app_upf_datapath_drop_total              | Packets the data plane did not forward, labeled by `direction` and reason.                                                       | Counter   |
| app_upf_datapath_fib_lookup_total        | FIB lookup outcomes in the data plane labeled by `direction` and `result`.                                                       | Counter   |
| app_upf_bytes_total                      | Total number of bytes transmitted through the data plane, labeled by `direction`. This value includes the Ethernet header.       | Counter   |
| app_upf_bpf_map_entries                  | Entries currently installed in a data plane BPF map, labeled by `map`. Divide by `app_upf_bpf_map_max_entries` for a fill ratio. | Gauge     |
| app_upf_bpf_map_max_entries              | Capacity of a data plane BPF map, labeled by `map`.                                                                              | Gauge     |
| app_upf_bpf_map_memory_bytes             | Kernel memory locked by a data plane BPF map, labeled by `map`.                                                                  | Gauge     |
| app_upf_nat_evictions_total              | Conntrack entries the data plane found evicted under load and re-created, labeled by the `direction`.                            | Counter   |
| app_upf_dl_buffer_capture_attempts_total | Downlink packets for an idle UE the data plane offered to the buffer, labeled by `result`.                                       | Counter   |
| app_upf_dl_buffer_evictions_total        | Buffered downlink packets discarded before re-injection, labeled by `reason`.                                                    | Counter   |
| app_upf_ringbuf_events_lost_total        | Events the data plane raised but could not place in a ring buffer, labeled by `map`.                                             | Counter   |
| app_api_requests_total                   | Total number of HTTP requests by method, endpoint, and status code                                                               | Counter   |
| app_api_request_duration_seconds         | HTTP request duration histogram in seconds                                                                                       | Histogram |
| app_api_authentication_attempts_total    | Total number of authentication attempts by type and result                                                                       | Counter   |
| app_database_storage_bytes               | The total storage used by the database in bytes. This is the size of the database file on disk.                                  | Gauge     |
| app_database_queries_total               | Total number of database queries by table and operation                                                                          | Counter   |
| app_database_query_duration_seconds      | Duration of database queries                                                                                                     | Histogram |
| app_raft_changeset_bytes_total           | SQLite changeset bytes applied through the Raft FSM. Emitted only when clustering is enabled.                                    | Counter   |

Note

When clustering is enabled, Ella Core also exports the full [hashicorp/raft](https://github.com/hashicorp/raft) metrics suite (prefix `raft_`).

## 2. Logs

Ella Core produces three types of logs:

- **System Logs**: General operational information about the system.
- **Audit Logs**: Logs of user actions for security and compliance. You can view audit logs and manage their retention via the [API](https://docs.ellanetworks.com/reference/api/audit_logs/index.md) and the Web UI.
- **Radio Logs**: Logs related to NGAP (5G) and S1AP (4G) messages. You can view radio logs and manage their retention via the [API](https://docs.ellanetworks.com/reference/api/radios/index.md) and the Web UI.

All logs are output in **JSON format** with structured fields for easy parsing and ingestion into log aggregation systems like Loki, Elasticsearch, or Splunk.

For more information on configuring logging in Ella Core, refer to the [Configuration File](https://docs.ellanetworks.com/reference/config_file/index.md) documentation.

Note

Ella Core does not assist with log rotation; we recommend using a log rotation tool to manage log files.

## 3. Traces

Ella Core supports distributed tracing using [OpenTelemetry](https://opentelemetry.io/). Traces are exported via [OTLP (gRPC)](https://opentelemetry.io/docs/specs/otlp/) to any compatible backend such as Jaeger, Tempo, or Honeycomb.

Traces are collected for the following components:

- **NGAP/S1AP**: Traces for NGAP (5G) and S1AP (4G) message handling between radios and Ella Core.
- **API**: Traces for HTTP requests to the REST API.

For more information on configuring tracing in Ella Core, refer to the [Configuration File](https://docs.ellanetworks.com/reference/config_file/index.md) documentation.

## 4. Profiles

Ella Core exposes the [http/pprof](https://pkg.go.dev/net/http/pprof) API for CPU and memory profiling analysis. This allows users to collect and analyze profiles of Ella Core using visualization tools like [pprof](https://pkg.go.dev/net/http/pprof) or [pyroscope](https://grafana.com/oss/pyroscope/).

For more information on accessing the pprof API in Ella Core, refer to the [pprof API documentation](https://docs.ellanetworks.com/reference/api/pprof/index.md).

## Alert Rules

Ella Core ships with pre-configured [Grafana alert rules](https://github.com/ellanetworks/core/tree/main/observability/grafana/alerting/alerts.yml) that detect the most important failure scenarios.

### Network Health

#### No Radios Connected

Critical. No radios connected for 2 minutes.

Check that the radio is powered on, reachable from the core, and configured with this core's address and PLMN.

#### High Registration Failure Rate

Critical. More than 10% of registration attempts rejected over 5 minutes.

Break down `app_registration_attempts_total` by `result`, then check the subscriber's credentials, profile, and policy.

#### High PDU Session Failure Rate

Critical. More than 10% of session establishments rejected over 5 minutes.

Break down `app_session_establishment_attempts_total` by `result`, then check the subscriber's policy, its data network, and the IP pool.

#### IP Address Pool Near Exhaustion

Warning. More than 90% of the data network IP pool is allocated.

Enlarge the data network's IP pool, or remove subscribers that no longer need addresses. New PDU sessions fail once the pool is full.

### Data Plane Health

#### High Data Plane Packet Drop Rate

Warning. More than 10 packets/s dropped by the data plane for 5 minutes.

Break down `app_upf_datapath_drop_total` by `reason` and `direction` to localise the drop.

#### No Data Plane Traffic

Critical. Radios connected but zero uplink and downlink throughput for 10 minutes.

Confirm a subscriber is actually sending traffic, then check the data network and route configuration.

#### Data Plane Aborted Actions Detected

Critical. Any drop with an `internal_` reason for 2 minutes.

An `internal_` reason means Ella Core's data plane failed rather than dropping traffic by policy. Break down `app_upf_datapath_drop_total` by `reason` and report it.

#### Flow Reports Being Dropped

Warning. Any flow report dropped in the last 5 minutes.

Data usage reporting is incomplete while this persists. Shorten the flow report retention policy, or report this if it continues at normal traffic levels.

### API Health

#### High API Error Rate

Warning. More than 5% of API responses are 5xx over 5 minutes.

Break down `app_api_requests_total` by `endpoint` and `status` to identify the failing endpoint.

#### High API Latency (P99)

Warning. P99 API response time exceeds 2 seconds over 5 minutes.

Check database query latency and CPU usage on the Deep Dive dashboard.

#### Authentication Failure Spike

Warning. More than 25% of API authentication attempts fail over 5 minutes.

Review the audit log to identify the source and whether the credentials are still valid.

### Infrastructure Health

#### Instance Down

Critical. The scrape target is unreachable for more than 20% of the last 10 minutes.

Check that the Ella Core process is running and reachable from the metrics collector. Note that this only proves the collector can reach the core, not that radios can.

#### High Memory Usage

Warning. Resident memory is forecast to exceed 1 GiB within 48 hours, based on the last 6 hours of growth, sustained for 30 minutes.

Compare heap profiles in Pyroscope to find what is retaining memory. Tune the 1 GiB ceiling to your deployment size, and the 48 hour horizon to how much warning you want.

#### High Goroutine Count

Warning. Goroutine count is forecast to exceed 10,000 within 48 hours, based on the last hour of growth, sustained for 30 minutes.

This indicates a resource leak in Ella Core. Capture a goroutine profile from Pyroscope and report it.

#### High Database Query Latency (P99)

Warning. P99 database query latency exceeds 500ms over 5 minutes.

Check the database size and the query rate on the Deep Dive dashboard.

#### Large Database Size

Warning. Database file exceeds 1 GiB for 10 minutes.

Shorten the retention policy for network logs, audit logs, or flow reports.

## Dashboards

Ella Core ships with [Grafana](https://grafana.com/) dashboards that you can import using the Dashboard IDs provided below.

### Network Health

This dashboard uses Prometheus metrics to provide real-time visibility into your mobile private network.

Grafana dashboard for Network Health.

- Data Sources: Prometheus
- Dashboard ID: 24751
- View online: [grafana.com/grafana/dashboards/24751/](https://grafana.com/grafana/dashboards/24751/)

### Deep Dive (for developers)

This dashboard uses metrics, logs, traces, and profiles to provide deep insights into the internal workings of Ella Core. It is intended for developers and advanced users who want to understand the performance and internal behavior of Ella Core. We recommend running Grafana Alloy to collect all signals ([example configuration file](https://github.com/ellanetworks/core/tree/main/observability/alloy)). A complete [example observability stack](https://github.com/ellanetworks/core/tree/main/observability) (Grafana, Mimir, Loki, Tempo, Pyroscope) is provided as a Docker Compose setup.

Grafana dashboard for Deep Dive.

- Data Sources: Mimir, Loki, Tempo, Pyroscope
- Dashboard ID: 24770
- View online: [grafana.com/grafana/dashboards/24770/](https://grafana.com/grafana/dashboards/24770/)
