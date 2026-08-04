---
description: Disable merged packets in TCX or generic XDP modes.
---

# Disable merged packets

When installing Ella Core in `tcx` or `xdp-generic` datapath mode, no merged packets should reach the data plane (see the [merged packets explanation](../explanation/user_plane_packet_processing_with_ebpf.md#merged-packets) for more info). Apply this to both the N3 and the N6 interface. Depending on the deployment:

**Physical NIC**:

```
ethtool -K <interface> gro off lro off
```

**veth**:

```
ethtool -K <peer> tso off gso off
```

**virtio (VM)**:

```
ethtool -K <interface> gro off rx-gro-hw off
```
