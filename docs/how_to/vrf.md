---
description: How to run Ella Core interfaces inside VRFs
---

Any interface used by Ella Core can be in a VRF.

## Use netplan to configure N3 and N6 in a user-plane VRF

Edit your netplan configuration:

```yaml
network:
  vrfs:
    up-vrf:
      table: 1001
      interfaces:
        - enp4s0f0
        - enp4s0f1
      routes:
        - to: default
          via: 10.6.0.1
          metric: 100
        # High metric unreachable route to prevent lookups in other tables
        - to: 0.0.0.0/0
          type: unreachable
          metric: 9000
  ethernets:
    enp2s0:
      addresses:
        - 10.1.0.10/24
      nameservers:
        addresses:
          - 1.1.1.1
      routes:
        - to: default
          via: 10.1.0.1
    enp4s0f0:
      addresses:
        - 10.6.0.3/24
    enp4s0f1:
      addresses:
        - 10.3.0.3/24

  version: 2
```

!!! warning "N3 and N6 must share a VRF"
    The N3 and N6 interfaces must be assigned to the **same** VRF. Ella Core will refuse to start otherwise.

## Configure Ella Core

Use the interfaces directly in the configuration file:

```yaml
interfaces:
  n2:
    address: "10.1.0.10"
  n3:
    name: "enp4s0f1"
  n6:
    name: "enp4s0f0"
  api:
    address: "10.1.0.10"
    port: 5002
```
