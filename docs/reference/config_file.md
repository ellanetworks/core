---
description: Reference outlining configuration options.
---

# Configuration File

Ella is configured using a yaml formatted file. 

Start Ella core with the `--config` flag to specify the path to the configuration file.

## Parameters

- `log-level` (string): The log level for the application. Options are `debug`, `info`, `warning`, `error`, and `critical`.
- `db` (object): The database configuration.
    - `path` (string): The path to the database file.
- `interfaces` (object): The network interfaces configuration.
    - `n2` (object): The configuration for the n2 interface. This interface should be connected to the radios.
        - `name` (string): The name of the network interface. 
        - `address` (string): The IP address of the network interface. 
        - `port` (int): The port to listen on.
    - `n3` (object): The configuration for the n3 interface. This interface should be connected to the radios.
        - `name` (string): The name of the network interface.
        - `address` (string): The IP address of the network interface.
    - `n6` (object): The configuration for the n6 interface. This interface should be connected to the internet.
        - `name` (string): The name of the network interface.
    - `api` (object): The configuration for the api interface.
        - `name` (string): The name of the network interface.
        - `port` (int): The port to listen on.
        - `tls` (object): The TLS configuration.
            - `cert` (string): The path to the TLS certificate file.
            - `key` (string): The path to the TLS key file.
- `xdp` (object): The XDP configuration.
    - `attach-mode` (string): The XDP attach mode. Options are `native` and `generic`. `native` is the most performant option and only works on supported drivers.

## Example

```yaml
log-level: "debug"
db:
  path: "core.db"
interfaces:
  n2:
    name: "enp2s0"
    address: "127.0.0.1"
    port: 38412
  n3: 
    name: "enp3s0"
    address: "127.0.0.1"
  n6:
    name: "enp6s0"
  api:
    name: "enp0s8"
    port: 5002
    tls:
      cert: "/etc/ssl/certs/core.crt"
      key: "/etc/ssl/private/core.key"
xdp:
  attach-mode: "native"
```
