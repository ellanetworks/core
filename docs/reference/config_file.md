# Configuration File

The configuration file is a YAML file that contains the configuration of the core. Start Ella core with the `--config` flag to specify the path to the configuration file.


## Configuration Options

```yaml
log-level: "debug"
db:
  path: "core.db"
interfaces: 
  n3: 
    name: "enp3s0"
    address: "127.0.0.1"
  n6:
    name: "enp6s0"
  api:
    name: "enp0s8"
    port: 5000
    tls:
      cert: "/etc/ssl/certs/core.crt"
      key: "/etc/ssl/private/core.key"
```