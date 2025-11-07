---
description: How to use VLAN interfaces with Ella Core
---

Any interface used by Ella Core can be on a VLAN interface. First, configure the VLAN on the host system, then use the name of the VLAN interface in the configuration file:

```yaml
logging:
  system:
    level: "info"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "/var/snap/ella-core/common/data/core.db"
interfaces:
  n2:
    address: "10.3.0.2"
    port: 38412
  n3:
    name: "ens5.103"
  n6:
    name: "ens3.106"
  api:
    address: "0.0.0.0"
    port: 5002
    tls:
      cert: "/var/snap/ella-core/common/cert.pem"
      key: "/var/snap/ella-core/common/key.pem"
xdp:
  attach-mode: "native"
telemetry:
  enabled: false
```

