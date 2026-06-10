# Observability

Ella Core supports four observability pillars: Metrics, Logs, Traces, and Profiles.

## 1. Metrics

Ella Core exposes [Prometheus](https://prometheus.io/) metrics to monitor the health of an Ella Core instance.

Please refer to the [metrics API documentation](https://docs.ellanetworks.com/reference/api/metrics/index.md) for more information on accessing metrics in Ella Core.

### Default Go metrics

These metrics are used to monitor the health of the Go runtime and garbage collector. These metrics start with the `go_` prefix.

### Custom metrics

These metrics are used to monitor the health of the system and the performance of the network. These metrics start with the `app_` prefix. The following custom metrics are exposed by Ella Core:

| Metric                                       | Description                                                                                                                                                                                                                                 | Type      |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| app_connected_radios                         | Number of radios currently connected to Ella Core                                                                                                                                                                                           | Gauge     |
| app_ngap_messages_total                      | Total number of received NGAP message per type                                                                                                                                                                                              | Counter   |
| app_registered_subscribers                   | Number of subscribers currently registered in Ella Core                                                                                                                                                                                     | Gauge     |
| app_registration_attempts_total              | Total number of subscriber registration attempts by type and result                                                                                                                                                                         | Counter   |
| app_pdu_sessions_total                       | Number of PDU sessions currently in Ella Core.                                                                                                                                                                                              | Gauge     |
| app_pdu_session_establishment_attempts_total | Total PDU session establishment attempts by result                                                                                                                                                                                          | Counter   |
| app_ip_addresses_allocated_total             | The total number of IP addresses currently allocated to subscribers.                                                                                                                                                                        | Gauge     |
| app_ip_addresses_total                       | The total number of IP addresses available for subscribers.                                                                                                                                                                                 | Gauge     |
| app_xdp_action_total                         | The total number of packets, with labels for the interface (n3, n6), and action taken.                                                                                                                                                      | Counter   |
| app_xdp_fib_lookup_total                     | FIB lookup outcomes in the XDP data plane, with labels for interface (n3, n6) and result matching kernel return codes (success, no_neigh, blackhole, unreachable, prohibit, no_src_addr, frag_needed, not_fwded, fwd_disabled, unsupp_lwt). | Counter   |
| app_xdp_ifindex_mismatch_total               | Packets dropped because the FIB-resolved interface did not match the expected N3/N6 interface, with label for interface (n3, n6).                                                                                                           | Counter   |
| app_uplink_bytes                             | The total number of bytes transmitted in the uplink direction (N3 -> N6). This value includes the Ethernet header.                                                                                                                          | Counter   |
| app_downlink_bytes                           | The total number of bytes transmitted in the downlink direction (N6 -> N3). This value includes the Ethernet header.                                                                                                                        | Counter   |
| app_api_requests_total                       | Total number of HTTP requests by method, endpoint, and status code                                                                                                                                                                          | Counter   |
| app_api_request_duration_seconds             | HTTP request duration histogram in seconds                                                                                                                                                                                                  | Histogram |
| app_api_authentication_attempts_total        | Total number of authentication attempts by type and result                                                                                                                                                                                  | Counter   |
| app_database_storage_bytes                   | The total storage used by the database in bytes. This is the size of the database file on disk.                                                                                                                                             | Gauge     |
| app_database_queries_total                   | Total number of database queries by table and operation                                                                                                                                                                                     | Counter   |
| app_database_query_duration_seconds          | Duration of database queries                                                                                                                                                                                                                | Histogram |
| app_raft_changeset_bytes_total               | SQLite changeset bytes applied through the Raft FSM. Emitted only when clustering is enabled.                                                                                                                                               | Counter   |

Note

When clustering is enabled, Ella Core also exports the full upstream [hashicorp/raft](https://github.com/hashicorp/raft) metrics suite (prefix `raft_`). These cover cluster state, leadership, replication, FSM apply latency, and snapshotting. The most useful ones for HA monitoring are:

- `raft_state_leader`, `raft_state_follower`, `raft_state_candidate` — counters incremented on each state transition. Rate indicates leadership flapping.
- `raft_leader_lastContact` — time since the leader last heard from a majority of peers (leader-only). Stale values indicate leader isolation.
- `raft_peers` — number of servers in the cluster configuration.
- `raft_fsm_apply` — FSM apply latency histogram. Covers the changeset apply path.
- `raft_replication_appendEntries_rpc`, `raft_replication_heartbeat` — per-peer replication latency, labeled by `peer_id`. Slow or absent values indicate an unhealthy follower.
- `raft_transition_heartbeat_timeout`, `raft_transition_leader_lease_timeout` — counters for failure-driven transitions.
- `raft_oldestLogAge` — age of the oldest retained log entry. Growing unbounded indicates snapshot/compaction is stuck.
- `raft_commitTime`, `raft_commitNumLogs` — commit latency and batch size on the leader.

## 2. Logs

Ella Core produces three types of logs:

- **System Logs**: General operational information about the system.
- **Audit Logs**: Logs of user actions for security and compliance. You can view audit logs and manage their retention via the [API](https://docs.ellanetworks.com/reference/api/audit_logs/index.md) and the Web UI.
- **Radio Logs**: Logs related to NGAP messages. You can view radio logs and manage their retention via the [API](https://docs.ellanetworks.com/reference/api/radios/index.md) and the Web UI.

All logs are output in **JSON format** with structured fields for easy parsing and ingestion into log aggregation systems like Loki, Elasticsearch, or Splunk.

For more information on configuring logging in Ella Core, refer to the [Configuration File](https://docs.ellanetworks.com/reference/config_file/index.md) documentation.

Note

Ella Core does not assist with log rotation; we recommend using a log rotation tool to manage log files.

## 3. Traces

Ella Core supports distributed tracing using [OpenTelemetry](https://opentelemetry.io/). Traces are exported via [OTLP (gRPC)](https://opentelemetry.io/docs/specs/otlp/) to any compatible backend such as Jaeger, Tempo, or Honeycomb.

Traces are collected for the following components:

- **NGAP**: Traces for NGAP message handling between gNodeBs and Ella Core.
- **API**: Traces for HTTP requests to the REST API.

For more information on configuring tracing in Ella Core, refer to the [Configuration File](https://docs.ellanetworks.com/reference/config_file/index.md) documentation.

## 4. Profiles

Ella Core exposes the [http/pprof](https://pkg.go.dev/net/http/pprof) API for CPU and memory profiling analysis. This allows users to collect and analyze profiles of Ella Core using visualization tools like [pprof](https://pkg.go.dev/net/http/pprof) or [pyroscope](https://grafana.com/oss/pyroscope/).

For more information on accessing the pprof API in Ella Core, refer to the [pprof API documentation](https://docs.ellanetworks.com/reference/api/pprof/index.md).

## Alert Rules

Ella Core ships with pre-configured [Grafana alert rules](https://github.com/ellanetworks/core/tree/main/observability/grafana/alerting/alerts.yml) that detect the most important failure scenarios.

### Network Health

| Alert                           | Severity | Condition                                                           |
| ------------------------------- | -------- | ------------------------------------------------------------------- |
| No Radios Connected             | Critical | No radios connected for 2 minutes                                   |
| High Registration Failure Rate  | Critical | More than 10% of subscriber registrations rejected over 5 minutes   |
| High PDU Session Failure Rate   | Critical | More than 10% of PDU session establishments rejected over 5 minutes |
| IP Address Pool Near Exhaustion | Warning  | More than 90% of the data network IP pool is allocated              |

### Data Plane Health

| Alert                     | Severity | Condition                                                            |
| ------------------------- | -------- | -------------------------------------------------------------------- |
| High XDP Packet Drop Rate | Warning  | More than 10 packets/s dropped by XDP for 5 minutes                  |
| No Data Plane Traffic     | Critical | Radios connected but zero throughput for 10 minutes                  |
| XDP Aborted Actions       | Critical | Any XDP_ABORTED events for 2 minutes (indicates eBPF program errors) |

### API Health

| Alert                        | Severity | Condition                                                        |
| ---------------------------- | -------- | ---------------------------------------------------------------- |
| High API Error Rate          | Warning  | More than 5% of API responses are 5xx errors over 5 minutes      |
| High API Latency             | Warning  | P99 API response time exceeds 2 seconds over 5 minutes           |
| Authentication Failure Spike | Warning  | More than 25% of API authentication attempts fail over 5 minutes |

### Infrastructure Health

| Alert                       | Severity | Condition                                               |
| --------------------------- | -------- | ------------------------------------------------------- |
| Instance Down               | Critical | Ella Core instance is unreachable                       |
| High Memory Usage           | Warning  | Process memory exceeds 1 GiB for 5 minutes              |
| High Goroutine Count        | Warning  | More than 10,000 goroutines for 5 minutes               |
| High Database Query Latency | Warning  | P99 database query latency exceeds 500ms over 5 minutes |
| Large Database Size         | Warning  | Database file exceeds 1 GiB for 10 minutes              |

## Dashboards

Ella Core ships with [Grafana](https://grafana.com/) dashboards that you can import using the Dashboard IDs provided below.

### Network Health

This dashboard uses Prometheus metrics to provide real-time visibility into all aspects of your 5G private network deployment, from radio connectivity and subscriber sessions to system performance and data plane throughput.

Grafana dashboard for Network Health.

- Data Sources: Prometheus
- Dashboard ID: 24751
- View online: [grafana.com/grafana/dashboards/24751/](https://grafana.com/grafana/dashboards/24751/)

### Deep Dive (for developers)

This dashboard uses metrics, logs, traces, and profiles to provide deep insights into the internal workings of Ella Core. It is intended for developers and advanced users who want to understand the performance and behavior of Ella Core at a granular level. We recommend running Grafana Alloy to collect all signals ([example configuration file](https://github.com/ellanetworks/core/tree/main/observability/alloy)). A complete [example observability stack](https://github.com/ellanetworks/core/tree/main/observability) (Grafana, Mimir, Loki, Tempo, Pyroscope) is provided as a Docker Compose setup.

Grafana dashboard for Deep Dive.

- Data Sources: Mimir, Loki, Tempo, Pyroscope
- Dashboard ID: 24770
- View online: [grafana.com/grafana/dashboards/24770/](https://grafana.com/grafana/dashboards/24770/)
