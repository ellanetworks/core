---
description: Reference outlining configuration options.
---

# Configuration File

Ella is configured using a yaml formatted file.

Start Ella core with the `--config` flag to specify the path to the configuration file.

## Parameters

- `logging` (object): The logging configuration.
    - `system` (object): The system logging configuration.
        - `level` (string): The log level. Options are `debug`, `info`, `warn`, `error`, and `fatal`.
        - `output` (string): The output for the logs. Options are `stdout` and `file`.
        - `path` (string): The path to the log file. Only used if the output is set to `file`.
    - `audit` (object): The audit logging configuration.
        - `output` (string): The output for the logs. Options are `stdout` and `file`.
        - `path` (string): The path to the log file. Only used if the output is set to `file`.
- `db` (object): The database configuration.
    - `path` (string): The path to the SQLite database file.
- `interfaces` (object): The network interfaces configuration.
    - `n2` (object): The configuration for the n2 interface (N2 in 5G, S1-MME in 4G). This is the SCTP control-plane interface towards radios.
        - `name` (string): The name of the network interface to listen on (optional: either name or address must be provided). When set, the server binds to all IP addresses configured on this interface.
        - `address` (string): The IP address to listen on (optional: either name or address must be provided). When set, the server binds to this specific address.
        - `ngap-port` (int, optional): The SCTP port for the 5G N2 / NGAP listener. Default `38412`.
        - `s1ap-port` (int, optional): The SCTP port for the 4G S1-MME / S1AP listener. Default `36412`.
        - `port` (int, optional): Deprecated alias for `ngap-port`. Cannot be set together with `ngap-port`.
    - `n3` (object): The configuration for the n3 interface (N3 in 5G, S1-U in 4G). This is the user plane interface towards radios.
        - `name` (string): The name of the network interface (optional: either name or address must be provided).
        - `address` (string): The address to listen on (optional: either name or address must be provided).
    - `n6` (object): The configuration for the n6 interface (N6 in 5G, SGi in 4G). This interface should be connected to the internet. It carries the `default` network instance.
        - `name` (string): The name of the network interface.
    - `network-instances` (list, optional): Additional N6 network instances, each with its own interface. Sessions use the network instance set in the `network_instance` field of their slice and data network (see [Slices](api/slices.md#list-the-data-networks-of-a-slice)).
        - `name` (string): The network instance name. `default` is reserved for `interfaces.n6`.
        - `n6` (object): The N6 interface of the network instance.
            - `name` (string): The name of the network interface. It can be a VLAN or a veth, in any VRF, but not the n3 or n6 interface or the interface of another network instance. It must exist when Ella Core starts.
    - `api` (object): The configuration for the api interface.
        - `name` (string): The name of the network interface to listen on (optional: either name or address must be provided). When set, the server listens on all addresses (`0.0.0.0`) but uses `SO_BINDTODEVICE` to restrict incoming traffic to this interface. Use this when you want to bind to a device without pinning to a specific IP address.
        - `address` (string): The IP address to listen on (optional: either name or address must be provided). When set, the server binds to this specific address.
        - `port` (int): The port to listen on.
        - `tls` (object): The TLS configuration (optional).
            - `cert` (string): The path to the TLS certificate file (optional).
            - `key` (string): The path to the TLS key file (optional).
    - `diameter` (object, optional): The configuration for the Diameter interface.
        - `name` (string): The name of the network interface to listen on (optional: either name or address must be provided).
        - `address` (string): The IP address to listen on (optional: either name or address must be provided).
        - `port` (int, optional): The port to listen on. Default `3868`.
- `datapath` (object): The datapath configuration (optional). When omitted, the datapath attaches at the XDP hook in driver mode where the network interface supports it, and at the TCX hook otherwise.
    - `attach-mode` (string): The kernel hook the datapath attaches to (optional): `xdp-native`, `tcx`, or `xdp-generic`. See [the eBPF attach mode explanation](../explanation/user_plane_packet_processing_with_ebpf.md).
- `xdp` (object, deprecated): Replaced by `datapath`. Cannot be set together with `datapath`.
    - `attach-mode` (string): `native` is equivalent to `datapath.attach-mode: xdp-native`, `generic` to `xdp-generic`.
- `telemetry` (object): The telemetry configuration.
    - `enabled` (boolean): Whether telemetry is enabled or not. Default is `false`.
    - `otlp-endpoint` (string): The endpoint for the OpenTelemetry Protocol (OTLP) collector.
- `cluster` (object): Clustering configuration for high-availability deployments. See [Clustering](#clustering).
    - `enabled` (boolean): Enables HA mode. When `false`, Ella Core runs standalone.
    - `node-id` (int, 1–63, optional, **deprecated**): Node identity. Deprecated, the node ID is now generated automatically.
    - `bind-address` (string): `host:port` the cluster listener binds to. Carries Raft consensus and cluster HTTP over mTLS.
    - `advertise-address` (string, optional): `host:port` peers use to reach this node. Host may be an IP or DNS name. Defaults to `bind-address`. When `bind-address` has no host or an unspecified host (`0.0.0.0`, `::`), defaults to the source address of the default route and the `bind-address` port.
    - `join` (list of strings, optional): The `host` or `host:port` of the other nodes, on the `bind-address` port when the port is left out. A host name resolves to all its addresses. A node with no cluster state forms or joins a cluster through these nodes. Ignored on a node that has cluster state.
    - `bootstrap-expect` (int, optional): The number of nodes that form a new cluster together. 2 or more. Requires `join`. Without it, a node only joins an existing cluster.
    - `secret-file` (string): The path to the cluster secret: standard base64 values (with padding) of at least 32 bytes, one per line. The first line signs, every line verifies. Required with `join`.
    - `peers`, `join-token`, `initial-suffrage` (**removed**): Ignored. Use `join` or the API or UI instead.
    - `join-timeout` (duration string, optional): How long a joining node keeps trying to reach a formed peer before it gives up and exits. Defaults to `2m`.
    - `propose-timeout` (duration string, optional): Maximum wait for a Raft commit before the API returns 503.
    - `snapshot-interval` (duration string, optional): Minimum interval between automatic Raft snapshots.
    - `snapshot-threshold` (int, optional): Minimum number of applied log entries between automatic snapshots.
    - `trailing-logs` (int, optional): Number of Raft log entries retained after a snapshot, so a lagging follower can catch up by log replay instead of a full snapshot install. Defaults to the Raft library's default, currently `10240`.

!!! note
    When you use the Ella Core snap, the configuration file is located at `/var/snap/ella-core/common/core.yaml`. After modifying the configuration file, restart Ella Core with `sudo snap restart ella-core.cored` for the changes to take effect.

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
  path: "/var/lib/ella-core/ella.db"
interfaces:
  n2:
    address: "22.22.22.2"
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
datapath:
  attach-mode: "xdp-native"
telemetry:
  enabled: true
  otlp-endpoint: "localhost:4317"
```

## Clustering

Enable clustering on each node to deploy Ella Core in a high-availability configuration. See [Deploy a High Availability Cluster](../how_to/deploy_ha_cluster.md) for the walkthrough.

```yaml
cluster:
  bind-address: "10.0.0.1:7000"
```

To form the cluster from the configuration file, use the same block on every node:

```yaml
cluster:
  bind-address: ":7000"
  join: ["10.0.0.1", "10.0.0.2", "10.0.0.3"]
  bootstrap-expect: 3
  secret-file: "/var/snap/ella-core/common/cluster.secret"
```
