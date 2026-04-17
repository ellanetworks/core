---
description: Reference outlining configuration options.
---

# Configuration File

Ella is configured using a yaml formatted file.

Start Ella core with the `--config` flag to specify the path to the configuration file.

## Parameters

- `logging` (object): The logging configuration.
    - `system` (object): The system logging configuration.
        - `level` (string): The log level. Options are `trace`, `debug`, `info`, `warn`, `error`, and `fatal`.
        - `output` (string): The output for the logs. Options are `stdout` and `file`.
        - `path` (string): The path to the log file. This is only used if the output is set to `file`.
    - `audit` (object): The audit logging configuration.
        - `output` (string): The output for the logs. Options are `stdout` and `file`.
        - `path` (string): The path to the log file. This is only used if the output is set to `file`.
- `db` (object): The database configuration.
    - `path` (string): The path to the directory holding the database file (`ella.db`).
- `interfaces` (object): The network interfaces configuration.
    - `n2` (object): The configuration for the n2 interface. This interface should be connected to the radios.
        - `name` (string): The name of the network interface to listen on (optional: either name or address must be provided). When set, the server binds to all IP addresses configured on this interface. Link-local addresses (IPv6 link-local and IPv4 link-local) are automatically excluded.
        - `address` (string): The IP address to listen on. Supports both IPv4 and IPv6 addresses (optional: either name or address must be provided). When set, the server binds to this specific address.
        - `port` (int): The port to listen on.
    - `n3` (object): The configuration for the n3 interface. This interface should be connected to the radios.
        - `name` (string): The name of the network interface (optional: either name or address must be provided).
        - `address` (string): The address to listen on. Currently only IPv4 is supported (optional: either name or address must be provided).
    - `n6` (object): The configuration for the n6 interface. This interface should be connected to the internet.
        - `name` (string): The name of the network interface.
    - `api` (object): The configuration for the api interface.
        - `name` (string): The name of the network interface to listen on (optional: either name or address must be provided). When set, the server listens on all addresses (`0.0.0.0`) but uses `SO_BINDTODEVICE` to restrict incoming traffic to this interface. Use this when you want to bind to a device without pinning to a specific IP address.
        - `address` (string): The IP address to listen on. Supports both IPv4 and IPv6 addresses (optional: either name or address must be provided). When set, the server binds to this specific address.
        - `port` (int): The port to listen on.
        - `tls` (object): The TLS configuration (optional).
            - `cert` (string): The path to the TLS certificate file (optional).
            - `key` (string): The path to the TLS key file (optional).
- `xdp` (object): The XDP configuration.
    - `attach-mode` (string): The XDP attach mode. Options are `native` and `generic`. `native` is the most performant option and only works on supported drivers.
- `telemetry` (object): The telemetry configuration.
    - `enabled` (boolean): Whether telemetry is enabled or not. Default is `false`.
    - `otlp-endpoint` (string): The endpoint for the OpenTelemetry Protocol (OTLP) collector.
- `cluster` (object): The clustering configuration for high-availability deployments. When enabled, multiple Ella Core instances form a Raft consensus cluster.
    - `enabled` (boolean): Whether clustering is enabled. Default is `false`.
    - `node-id` (int): Unique Raft node identifier (1–63). Must be unique across all cluster members.
    - `bind-address` (string): The `host:port` that the Raft transport listens on for inter-node communication.
    - `advertise-api-address` (string): The full URL (including scheme) that other cluster members use to reach this node's API (e.g. `"https://10.0.0.1:5002"`). Used for leader proxy forwarding.
    - `bootstrap-expect` (int): Number of nodes expected for initial cluster bootstrap. Must be ≤ the number of entries in `peers`.
    - `peers` (list of strings): List of URLs of all initial cluster members. Each entry must be a valid URL with scheme and host, matching the `advertise-api-address` format. Must include this node's own `advertise-api-address`.
    - `join-token` (string): Shared secret used to authenticate nodes during cluster bootstrap. Must be at least 32 characters.
    - `join-timeout` (string): Maximum time to wait for peers during bootstrap (e.g. `"30s"`). Default is `"2m"`.
    - `propose-timeout` (string): Maximum time to wait for a Raft proposal to be committed (e.g. `"5s"`). Default is `"5s"`.
    - `snapshot-interval` (string): How often Raft takes a snapshot (e.g. `"2m"`). Uses the Raft library default when omitted.
    - `snapshot-threshold` (int): Minimum number of log entries before a snapshot is taken. Uses the Raft library default when omitted.
    - `initial-suffrage` (string): Initial suffrage state for this node when joining the cluster. Options are `"voter"` and `"nonvoter"`. Non-voters receive replicated data but do not participate in elections, useful for rolling upgrades. Default is `"voter"`.

!!! note
    When you use the Ella Core snap, the configuration file is located at `/var/snap/ella-core/common/config.yaml`. After modifying the configuration file, restart Ella Core with `sudo snap restart ella-core.cored` for the changes to take effect.

## Example

```yaml
logging:
  system:
    level: "info"
    output: "stdout"
  audit:
    output: "file"
    path: "/var/log/ella_system.log"
db:
  path: "/var/lib/ella-core"
interfaces:
  n2:
    address: "22.22.22.2"
    port: 38412
  n3:
    name: "ens5"
  n6:
    name: "ens3"
  api:
    address: "0.0.0.0"
    port: 5002
    tls:
      cert: "/etc/ella/cert.pem"
      key: "/etc/ella/key.pem"
xdp:
  attach-mode: "native"
telemetry:
  enabled: true
  otlp-endpoint: "localhost:4317"
```

## Clustering

To deploy Ella Core in a high-availability configuration, enable clustering on each node. All nodes must list each other in `peers` and use mTLS certificates signed by a shared cluster CA for peer authentication.

```yaml
cluster:
  enabled: true
  node-id: 1
  bind-address: "10.0.0.1:7000"
  advertise-address: "10.0.0.1:7000"
  bootstrap-expect: 3
  peers:
    - "10.0.0.1:7000"
    - "10.0.0.2:7000"
    - "10.0.0.3:7000"
  tls:
    ca: "/etc/ella/cluster-ca.pem"
    cert: "/etc/ella/node-1.pem"
    key: "/etc/ella/node-1-key.pem"
  join-timeout: "30s"
  propose-timeout: "5s"
  snapshot-interval: "2m"
  snapshot-threshold: 8192
```

- `bind-address` (string): The `host:port` address the cluster listener binds to. Carries all intra-cluster traffic (Raft consensus and cluster HTTP).
- `advertise-address` (string, optional): The `host:port` address peers use to reach this node. Defaults to `bind-address`. Must appear in the `peers` list.
- `peers` (list of strings): `host:port` cluster addresses of all nodes. Every node's `advertise-address` must appear in this list.
- `tls` (object): mTLS configuration for cluster communication. Required when clustering is enabled.
    - `ca` (string): Path to the CA bundle (PEM) trusted for peer certificates.
    - `cert` (string): Path to this node's leaf certificate (PEM). The leaf CN must be `ella-node-<node-id>`.
    - `key` (string): Path to the private key for the leaf certificate (PEM).

!!! note
    When clustering is enabled, write requests (POST, PUT, PATCH, DELETE) are automatically forwarded to the current Raft leader. Read requests are served by any node.

## IPv6 Support

Ella Core supports IPv6 addresses for the management interface (`api`), the radio interface (`n2`) and the GTPU interface (`n3`).

The following example demonstrates using an IPv6 address for those interfaces:

```yaml
interfaces:
  n2:
    address: "2001:db8::1"
    port: 38412
  n3:
    address: "2001:dba::1"
  n6:
    name: "ens3"
  api:
    address: "2001:db9::1"
    port: 5002
```

The following example demonstrates using all non link-local addresses for those interfaces:

```yaml
interfaces:
  n2:
    name: "ens5"
    port: 38412
  n3:
    address: "ens4"
  n6:
    name: "ens3"
  api:
    name: "ens0"
    port: 5002
```

!!! note
    IPv6 support is currently available for the management interface (`api`), radio interface (`n2`) and GTPU interface (`n3`). Only IPv4 is currently supported for the UE traffic and the data network interface (`n6`).

## GTP-U Transport over IPv6

Ella Core supports GTP-U tunnels over IPv6 for the N3 interface (between the core and the gNB). When a gNB advertises a dual-stack transport address (both IPv4 and IPv6) in the N2 signaling and Ella Core is configured for dual-stack, Ella Core always prefers IPv6 for the GTP-U data path.

To ensure Ella Core always uses IPv4 or IPv6 for GTP-U, specify an address of that family in the configuration file, or ensure only IPs of that family are configured on the interface.
