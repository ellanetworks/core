---
description: Reference of the metrics exposed in Prometheus format.
---

# Metrics

Ella Core exposes a set of metrics that can be used to monitor the health of the system and the performance of the network.

## Default Go metrics

These metrics are used to monitor the performance of the Go runtime and garbage collector. These metrics start with the `go_` prefix.

## Custom metrics

These metrics are used to monitor the health of the system and the performance of the network. These metrics start with the `app_` prefix. The following custom metrics are exposed by Ella Core:

- `app_database_storage_bytes`: The total storage used by the database in bytes. This is the size of the database file on disk.
- `app_ip_addresses_allocated_total`: The total number of IP addresses currently allocated to subscribers.
- `app_ip_addresses_total`: The total number of IP addresses available for subscribers.
- `app_pdu_sessions_total`: Number of PDU sessions currently in Ella Core.
- `app_uplink_bytes`: The total number of bytes transmitted in the uplink direction (N3 -> N6). This value includes the Ethernet header. 
- `app_downlink_bytes`: The total number of bytes transmitted in the downlink direction (N6 -> N3). This value includes the Ethernet header.
- `app_xdp_action_total`: The total number of packets, with labels for the interface (n3, n6), and action taken.
