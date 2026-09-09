---
description: Step-by-step instructions to deploy Ella Core alongside OCUDU for an all-in-one mobile network.
---

# Co-host with OCUDU

Ella Core can be hosted with radio software like [OCUDU](https://ocudu.org/) (previously known as srsRAN) to operate an all-in-one private mobile network. This guide provides step-by-step instructions to deploy Ella Core alongside OCUDU using a Linux network namespace.

<figure markdown="span">
  ![Ella Core and OCUDU](../images/ocudu.svg){ width="800" }
  <figcaption>Co-host Ella Core with OCUDU</figcaption>
</figure>

## Pre-requisites

To follow this guide, you will need:

- Ubuntu Server 24.04
- A host with a network interface
- An OCUDU-compatible SDR

The instructions below were written for a Raspberry Pi 5 running Ubuntu 24.04 as the host and the Ettus Research B205-mini as the SDR. Please adapt the interface names and SDR configuration as needed for your setup.

!!! tip
    OCUDU requires some performance tuning for stable operation, especially on resource-constrained hosts like a Raspberry Pi. We recommend the following optimizations for OCUDU performance:
    
    - Use Ubuntu Real-Time (with [pro client](https://documentation.ubuntu.com/pro-client/en/latest/howtoguides/enable_realtime_kernel/#enable-and-install-automatically))
    - Use a high real-time scheduling priority (with [chrt](https://man7.org/linux/man-pages/man1/chrt.1.html))
    - Set scaling governor to Performance (with [OCUDU performance script](https://gitlab.com/ocudu/ocudu/-/blob/dev/scripts/ocudu_performance?ref_type=heads))
    - Disable DRM KMS polling (with [OCUDU performance script](https://gitlab.com/ocudu/ocudu/-/blob/dev/scripts/ocudu_performance?ref_type=heads))

## 1. Install Ella Core and OCUDU

Install Ella Core using the [How-to Install guide](install.md) and install OCUDU using the [official documentation](https://ocudu.gitlab.io/ocudu_docs/user_manual/installation/).

## 2. Create a network namespace for N3

Create a linux network namespace `n3ns` for the N3 interface between OCUDU and Ella Core. We use a systemd service so the namespace and veth pair are recreated automatically on boot.

The `n3ns.service` below calls `/usr/local/bin/ocudu_performance -y`, but this script is not installed by OCUDU. Install it manually from `scripts/ocudu_performance` in the OCUDU git repository:

```shell
sudo curl -o /usr/local/bin/ocudu_performance https://gitlab.com/ocudu/ocudu/-/raw/dev/scripts/ocudu_performance
sudo chmod +x /usr/local/bin/ocudu_performance
```

If you already cloned the OCUDU repository, copy it from your local checkout instead:

```shell
sudo install -m 0755 scripts/ocudu_performance /usr/local/bin/ocudu_performance
```

Create `/etc/systemd/system/n3ns.service`:

```ini
[Unit]
Description=N3 Network Setup
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
RemainAfterExit=true

ExecStartPre=-ip netns delete n3ns
ExecStart=ip netns add n3ns
ExecStart=ip link add n3-upf-veth type veth peer name n3-ran-veth
ExecStart=ip link set n3-ran-veth netns n3ns
ExecStart=ip addr add 10.202.0.3/24 dev n3-upf-veth
ExecStart=ip -n n3ns addr add 10.202.0.5/24 dev n3-ran-veth
ExecStart=ip -n n3ns link set lo up
ExecStart=ip -n n3ns link set dev n3-ran-veth up
ExecStart=ip link set dev n3-upf-veth up
ExecStart=ethtool -K eth0 gro off
ExecStart=ip netns exec n3ns ethtool -K n3-ran-veth tso off gso off
ExecStart=/usr/local/bin/ocudu_performance -y
ExecStop=-ip netns delete n3ns

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```shell
sudo systemctl daemon-reload
sudo systemctl enable --now n3ns.service
```

`tcx` mode requires both interfaces to deliver unmerged packets — see [Disable merged packets](disable_merged_packets.md).

Make sure `eth0` is not optional in `/etc/netplan/50-cloud-init.yaml`:

```yaml
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true
      optional: false
```

Then apply the Netplan configuration:

```shell
sudo netplan apply
```

## 3. Configure Ella Core

Configure Ella Core's N3 and N3 interfaces to use the `n3ns` namespace and set N6 to the physical interface `eth0`:

```yaml hl_lines="11 14 16"
logging:
  system:
    level: "debug"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "/var/snap/ella-core/common/data/ella.db"
interfaces:
  n2:
    name: "n3-upf-veth"
    ngap-port: 38412
  n3:
    name: "n3-upf-veth"
  n6:
    name: "eth0"
  api:
    address: "0.0.0.0"
    port: 5002
datapath:
  attach-mode: "tcx"
telemetry:
  enabled: false
```

!!! note
    We use `tcx` mode here because the Raspberry Pi 5's built-in NIC does not support native XDP. If your host's NIC supports native XDP, set `attach-mode` to `xdp-native` and follow the [Use native XDP with veth interfaces](native_xdp_veth.md) guide to attach an XDP program to the peer veth.

Create the override file `/etc/systemd/system/snap.ella-core.cored.service.d/override.conf`:

```ini
[Unit]
Requires=n3ns.service
Wants=network-online.target
After=n3ns.service network-online.target
```

Then reload systemd and start Ella Core:

```shell
sudo systemctl daemon-reload
sudo snap start --enable ella-core.cored
```

## 4. Configure OCUDU

Configure OCUDU's CU to use the `n3ns` namespace:

```yaml hl_lines="3 5 19"
cu_cp:
  amf:
    addr: 10.202.0.3
    port: 38412
    bind_addr: 10.202.0.5
    supported_tracking_areas:
      - tac: 1
        plmn_list:
          - plmn: "99901"
            tai_slice_support_list:
              - sst: 1
  inactivity_timer: 300
  security:
    nea_pref_list: nea2,nea1
    nia_pref_list: nia2,nia1
cu_up:
  ngu:
    socket:
      - bind_addr: 10.202.0.5
ru_sdr:
  device_driver: uhd
  device_args: type=b200
  clock: internal
  srate: 23.04
  tx_gain: 80
  rx_gain: 40

cell_cfg:
  dl_arfcn: 665000
  band: 77
  channel_bandwidth_MHz: 20
  common_scs: 30
  plmn: "99901"
  tac: 1
  pdcch:
    dedicated:
      ss2_type: common
      dci_format_0_1_and_1_1: false
  prach:
    prach_config_index: 160
  pdsch:
    mcs_table: qam64
  pusch:
    mcs_table: qam64

log:
  filename: /tmp/gnb.log
  all_level: warning

pcap:
  mac_enable: disable
  ngap_enable: disable
```

Save this configuration to `/etc/gnb.yml`, then create a systemd service to run OCUDU in the `n3ns` namespace.

Create `/etc/systemd/system/gnb.service`:

```ini
[Unit]
Description=OCUDU gNB
Documentation=https://docs.ocudu.org/
Requires=n3ns.service
Wants=network-online.target
After=n3ns.service network-online.target

[Service]
User=root
Group=root
Restart=on-failure
NetworkNamespacePath=/var/run/netns/n3ns
ExecStart=/usr/local/bin/gnb -c /etc/gnb.yml

[Install]
WantedBy=multi-user.target
```

Enable and start OCUDU:

```shell
sudo systemctl daemon-reload
sudo systemctl enable --now gnb.service
```

You should see OCUDU logs indicating successful connection to Ella Core

```shell
--== OCUDU gNB (commit d1ca6d2744) ==--

2026-02-16T18:34:35.963576 [GNB     ] [I] Built in Release mode using commit d1ca6d2744 on branch dev
Lower PHY in dual baseband executor mode.
Available radio types: uhd.
[INFO] [UHD] linux; GNU C++ version 13.2.0; Boost_108300; UHD_4.6.0.0+ds1-5.1ubuntu0.24.04.1
Making USRP object with args 'type=b200'
[INFO] [LOGGING] Fastpath logging disabled at runtime.
[INFO] [B200] Detected Device: B205mini
[INFO] [B200] Operating over USB 3.
[INFO] [B200] Initialize CODEC control...
[INFO] [B200] Initialize Radio control...
[INFO] [B200] Performing register loopback test... 
[INFO] [B200] Register loopback test passed
[INFO] [B200] Setting master clock rate selection to 'automatic'.
[INFO] [B200] Asking for clock rate 16.000000 MHz... 
[INFO] [B200] Actually got clock rate 16.000000 MHz.
[INFO] [MULTI_USRP] Setting master clock rate selection to 'manual'.
[INFO] [B200] Asking for clock rate 23.040000 MHz... 
[INFO] [B200] Actually got clock rate 23.040000 MHz.
Cell pci=1, bw=20 MHz, 1T1R, dl_arfcn=665000 (n77), dl_freq=3975 MHz, dl_ssb_arfcn=664704, ul_freq=3975 MHz

N2: Connection to AMF on 10.202.0.3:38412 completed
Remote control server listening on 0.0.0.0:8001
==== gNB started ===
```
