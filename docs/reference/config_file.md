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
        - `path` (string): The path to the log file. This is only used if the output is set to `file`.
    - `audit` (object): The audit logging configuration.
        - `output` (string): The output for the logs. Options are `stdout` and `file`.
        - `path` (string): The path to the log file. This is only used if the output is set to `file`.
- `db` (object): The database configuration.
    - `path` (string): The path to the SQLite database file. The parent directory must already exist. The file should be named `ella.db`, as backup and restore expect that name.
- `interfaces` (object): The network interfaces configuration.
    - `n2` (object): The configuration for the n2 interface (N2 in 5G, S1-MME in 4G). This is the control-plane interface to the radios; the same interface serves both 5G gNBs and 4G eNBs over SCTP.
        - `name` (string): The name of the network interface to listen on (optional: either name or address must be provided). When set, the server binds to all IP addresses configured on this interface. Link-local addresses (IPv6 link-local and IPv4 link-local) are automatically excluded.
        - `address` (string): The IP address to listen on. Supports both IPv4 and IPv6 addresses (optional: either name or address must be provided). When set, the server binds to this specific address.
        - `ngap-port` (int, optional): The SCTP port for the 5G N2 / NGAP listener. Default `38412`.
        - `s1ap-port` (int, optional): The SCTP port for the 4G S1-MME / S1AP listener. Default `36412`.
        - `port` (int, optional): Deprecated alias for `ngap-port`. Cannot be set together with `ngap-port`.
    - `n3` (object): The configuration for the n3 interface (N3 in 5G, S1-U in 4G). This interface should be connected to the radios.
        - `name` (string): The name of the network interface (optional: either name or address must be provided). Ella Core advertises one address per family found on the interface; see [GTP-U transport addresses](#gtp-u-transport-addresses).
        - `address` (string): The address to listen on. Supports both IPv4 and IPv6 (optional: either name or address must be provided). When set, Ella Core advertises only this address and only its family.
    - `n6` (object): The configuration for the n6 interface (N6 in 5G, SGi in 4G). This interface should be connected to the internet.
        - `name` (string): The name of the network interface.
    - `api` (object): The configuration for the api interface.
        - `name` (string): The name of the network interface to listen on (optional: either name or address must be provided). When set, the server listens on all addresses (`0.0.0.0`) but uses `SO_BINDTODEVICE` to restrict incoming traffic to this interface. Use this when you want to bind to a device without pinning to a specific IP address.
        - `address` (string): The IP address to listen on. Supports both IPv4 and IPv6 addresses (optional: either name or address must be provided). When set, the server binds to this specific address.
        - `port` (int): The port to listen on.
        - `tls` (object): The TLS configuration (optional).
            - `cert` (string): The path to the TLS certificate file (optional).
            - `key` (string): The path to the TLS key file (optional).
- `datapath` (object, optional): The datapath configuration. When omitted, the datapath attaches at the XDP hook in driver mode where the network interface supports it, and at the TCX hook otherwise.
    - `attach-mode` (string, optional): The kernel hook the datapath attaches to: `xdp-native`, `tcx`, or `xdp-generic`. See [the eBPF attach mode explanation](../explanation/user_plane_packet_processing_with_ebpf.md).
- `xdp` (object, deprecated): Replaced by `datapath`. Cannot be set together with `datapath`.
    - `attach-mode` (string): `native` is equivalent to `datapath.attach-mode: xdp-native`, `generic` to `xdp-generic`.
- `telemetry` (object): The telemetry configuration.
    - `enabled` (boolean): Whether telemetry is enabled or not. Default is `false`.
    - `otlp-endpoint` (string): The endpoint for the OpenTelemetry Protocol (OTLP) collector.
- `cluster` (object): Clustering configuration for high-availability deployments. See [Clustering](#clustering) for the walkthrough.
    - `enabled` (boolean): Enables HA mode. When `false`, Ella Core runs as a standalone single-server instance.
    - `node-id` (int, 1–63): Unique per node. Baked into this node's self-signed cluster certificate (SPIFFE URI) and the GUTIs it issues.
    - `bind-address` (string): `host:port` the cluster listener binds to. Carries Raft consensus and cluster HTTP over mTLS.
    - `advertise-address` (string, optional): `host:port` peers use to reach this node. Host may be an IP or DNS name. Defaults to `bind-address`. Must appear in `peers` and must not use an unspecified IP.
    - `peers` (list of strings): `host:port` of every node in the cluster. Host may be an IP or DNS name. Must include this node's own `advertise-address` (or `bind-address` if `advertise-address` is unset) as the same string.
    - `join-token` (string, optional): Single-use token minted on the cluster leader via `POST /api/v1/cluster/pki/join-tokens`. Required on the first boot of a node joining an existing cluster; consumed and ignored on subsequent starts. Its presence also tells the daemon that this node is a joiner, not the founder.
    - `initial-suffrage` (string, optional): `voter` or `nonvoter`. Defaults to `voter`.
    - `join-timeout` (duration string, optional): Wait for cluster formation after which discovery starts logging that it is slow. Discovery keeps retrying past it.
    - `propose-timeout` (duration string, optional): Maximum wait for a Raft commit before the API returns 503.
    - `snapshot-interval` (duration string, optional): Minimum interval between automatic Raft snapshots.
    - `snapshot-threshold` (int, optional): Minimum number of applied log entries between automatic snapshots.
    - `trailing-logs` (int, optional): Number of Raft log entries retained after a snapshot so a briefly-disconnected follower can catch up by log replay instead of receiving a full snapshot. Defaults to `10240`, which is correct for the vast majority of deployments. Lower it only if the Raft log is growing unboundedly under sustained write load; raise it only if followers repeatedly fall behind and trigger snapshot installs. Setting it too low on a cluster with a large database can put followers in a loop where they keep downloading snapshots and never converge.

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
  enabled: true
  node-id: 1
  bind-address: "10.0.0.1:7000"
  peers:
    - "10.0.0.1:7000"
    - "10.0.0.2:7000"
    - "10.0.0.3:7000"
```

!!! note
    Write requests (POST, PUT, PATCH, DELETE) are automatically forwarded to the current Raft leader; reads are served by any node.

## IPv6 Support

Ella Core supports IPv6 addresses for the management interface (`api`), the radio interface (`n2`) and the GTPU interface (`n3`).

The following example demonstrates using an IPv6 address for those interfaces:

```yaml
interfaces:
  n2:
    address: "2001:db8::1"
  n3:
    address: "2001:db8::1"
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
  n3:
    address: "ens4"
  n6:
    name: "ens3"
  api:
    name: "ens0"
    port: 5002
```

## GTP-U Transport over IPv6

Ella Core supports GTP-U tunnels over IPv6 for the N3 / S1-U interface (between the core and the radio). When a radio advertises a dual-stack transport address (both IPv4 and IPv6) in the N2 / S1-MME signaling and Ella Core is configured for dual-stack, Ella Core always prefers IPv6 for the GTP-U data path.

## GTP-U transport addresses

The address Ella Core advertises to radios as the GTP-U endpoint is taken from the `n3` configuration:

- **`address` is set**: Ella Core advertises exactly that address, and only its family. Configuring an IPv4 address means radios are never offered an IPv6 endpoint, and vice versa.
- **`name` is set**: Ella Core discovers the interface's addresses and advertises one per family. A dual-stack interface therefore yields a dual-stack transport address.

Some radios only accept an IPv4-only transport address. Support for the dual-stack form was added in 3GPP TS 36.414 v12.1.0 (February 2015), so radios predating it may reject a session setup that carries one. Set `address` to your IPv4 address to advertise IPv4 only.

When discovering addresses from an interface, Ella Core skips addresses the kernel considers unfit as a stable endpoint: deprecated addresses, tentative and duplicate-address-detection failures, IPv4 secondary aliases, and IPv6 temporary (RFC 4941 privacy) addresses. Among the remaining addresses it prefers permanent ones, then the longest remaining lifetime. Ella Core watches the interface and re-advertises when its addresses change.

!!! note
    An address Ella Core discovers can change or expire outside its control, for example when a DHCP lease or a router advertisement is renewed with a different prefix. Set `address` to pin the endpoint.
