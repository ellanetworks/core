---
description: RESTful API reference for getting metrics in Prometheus format.
---

# Metrics

## Get metrics

This path returns the metrics of Ella core in Prometheus format. For more information about metrics exposed by Ella core, see the [Observability Reference](../observability.md).

| Method | Path              |
| ------ | ----------------- |
| GET    | `/api/v1/metrics` |

### Parameters

None

### Sample Response

```text
# HELP app_database_storage_bytes The total storage used by the database in bytes. This is the size of the database file on disk.
# TYPE app_database_storage_bytes gauge
app_database_storage_bytes 28672
# HELP app_ip_addresses_allocated_total The total number of IP addresses currently allocated to subscribers
# TYPE app_ip_addresses_allocated_total gauge
app_ip_addresses_allocated_total 0
# HELP app_ip_addresses_total The total number of IP addresses available for subscribers
# TYPE app_ip_addresses_total gauge
app_ip_addresses_total 65792
# HELP go_gc_duration_seconds A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 2.2352e-05
go_gc_duration_seconds{quantile="0.25"} 3.29e-05
go_gc_duration_seconds{quantile="0.5"} 5.5266e-05
go_gc_duration_seconds{quantile="0.75"} 0.000135436
go_gc_duration_seconds{quantile="1"} 0.000371761
go_gc_duration_seconds_sum 0.011946775
go_gc_duration_seconds_count 134
# HELP go_gc_gogc_percent Heap size target percentage configured by the user, otherwise 100. This value is set by the GOGC environment variable, and the runtime/debug.SetGCPercent function. Sourced from /gc/gogc:percent
# TYPE go_gc_gogc_percent gauge
go_gc_gogc_percent 100
...
```
