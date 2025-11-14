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
    - `path` (string): The path to the database file.
- `interfaces` (object): The network interfaces configuration.
    - `n2` (object): The configuration for the n2 interface. This interface should be connected to the radios.
        - `name` (string): The name of the network interface (optional: either name or address must be provided).
        - `address` (string): The address to listen on (optional: either name or address must be provided).
        - `port` (int): The port to listen on.
    - `n3` (object): The configuration for the n3 interface. This interface should be connected to the radios.
        - `name` (string): The name of the network interface (optional: either name or address must be provided).
        - `address` (string): The address to listen on (optional: either name or address must be provided).
        - `external-address` (string): The IP address advertised to the gNodeB to build the GTP-U tunnel (optional). If not set, the `address` field will be used. This field is useful when deploying Ella Core behind a proxy or NAT.
    - `n6` (object): The configuration for the n6 interface. This interface should be connected to the internet.
        - `name` (string): The name of the network interface.
    - `api` (object): The configuration for the api interface.
        - `name` (string): The name of the network interface (optional: either name or address must be provided).
        - `address` (string): The address to listen on (optional: either name or address must be provided).
        - `port` (int): The port to listen on.
        - `tls` (object): The TLS configuration (optional).
            - `cert` (string): The path to the TLS certificate file (optional).
            - `key` (string): The path to the TLS key file (optional).
- `xdp` (object): The XDP configuration.
    - `attach-mode` (string): The XDP attach mode. Options are `native` and `generic`. `native` is the most performant option and only works on supported drivers.
- `telemetry` (object): The telemetry configuration.
    - `enabled` (boolean): Whether telemetry is enabled or not. Default is `false`.
    - `otlp-endpoint` (string): The endpoint for the OpenTelemetry Protocol (OTLP) collector.

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
  path: "core.db"
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
