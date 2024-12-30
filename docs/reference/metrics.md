# Metrics

Ella Core exposes a set of metrics that can be used to monitor the health of the system and the performance of the network. These metrics are exposed in Prometheus format.

## Default Go metrics

These metrics are used to monitor the performance of the Go runtime and garbage collector. These metrics start with the `go_` prefix.

## Custom metrics

These metrics are used to monitor the health of the system and the performance of the network. These metrics start with the `app_` prefix. The following custom metrics are exposed by Ella Core:

- `app_database_storage_bytes`: The total storage used by the database in bytes. This is the size of the database file on disk.
- `app_ip_addresses_allocated`: The total number of IP addresses currently allocated to subscribers.
- `app_ip_addresses_total`: The total number of IP addresses allocated to subscribers.
- `app_xdp_drop`: The total number of dropped packets.
- `app_xdp_pass`: The total number of passed packets.
- `app_xdp_tx`: The total number of transmitted packets.
- `app_xdp_redirect`: The total number of redirected packets.
- `app_pdu_sessions`: Number of PDU sessions currently in Ella.
