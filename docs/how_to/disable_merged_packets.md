---
description: Disable merged packets in TCX or generic XDP modes.
---

# Disable merged packets

When installing Ella Core in `tcx` or `xdp-generic` datapath mode, no merged packets should reach the data plane (see the [merged packets explanation](../explanation/user_plane_packet_processing_with_ebpf.md#why-merged-packets-must-be-disabled) for more info).

Create a systemd unit that disables them on the N3 and N6 interfaces at every boot, before Ella Core starts. With N3 and N6 on `ens4` and the Ella Core snap:

```shell
sudo tee /etc/systemd/system/ella-no-gro.service <<'EOF'
[Unit]
After=network-online.target
Before=snap.ella-core.cored.service
[Service]
Type=oneshot
ExecStart=/usr/sbin/ethtool -K ens4 gro off lro off rx-gro-hw off
[Install]
WantedBy=multi-user.target
EOF
sudo systemctl enable --now ella-no-gro
```

Add one `ExecStart` line per interface when N3 and N6 are different interfaces.

For a veth interface, disable segmentation on its peer instead: `ethtool -K <peer> tso off gso off`.
