---
description: A hands-on introduction to Ella Core for new users.
---

# Getting Started

In this tutorial, we will deploy, initialize, and configure Ella Core. First, we will use [Multipass](https://canonical.com/multipass/docs) to create a virtual machine, install Ella Core, access the the UI, initialize Ella Core, and configure it. Then, we will create another virtual machine and install a 5G radio simulator, connect it to Ella Core, and validate that it is automatically detected.

## Pre-requisites

To complete this tutorial, you will need a Ubuntu 24.04 machine with the following specifications:

- 16GB of RAM
- 6 CPU cores
- 50GB of disk space

## Install Ella Core

### Setup the Virtual Machine where Ella Core will be installed

From the Ubuntu machine, install Multipass:

```shell
sudo snap install multipass
```

Create two lxc networks:

```shell
lxc network create n3
lxc network create n6
```

Use Multipass to create a bare Ubuntu 24.04 instance with two additional network interfaces:
```shell
multipass launch noble --name=ella-core --disk=10G --network n3 --network n6
```

Validate that the instance has been created with the two additional network interfaces:

```shell
multipass list
```

You should see the following output:
```shell
guillaume@courge:~$ multipass list
Name                    State             IPv4             Image
ella-core               Running           10.103.62.227    Ubuntu 24.04 LTS
                                          10.243.161.26
                                          10.117.122.101
```

You should see three IP addresses. Take note of the first one. In this example, the IP address is `10.103.62.227`. You will use this IP address later to access the Ella Core UI.

Connect to the instance:
```shell
multipass shell ella-core
```

Validate that the instance has the two additional network interfaces:
```shell
ip a
```

You should see that the instance has four network interfaces: `lo`, `ens3`, `ens4`, and `ens5`.

### Install and start Ella Core

Inside of the `ella-core` Multipass instance, install the Ella Core snap:
```shell
sudo snap install ella-core --channel=edge --devmode
```

Edit the configuration file at `/var/snap/ella-core/common/core.yaml` to configure the network interfaces:

```yaml hl_lines="7 11"
log-level: "info"
db:
  path: "/var/snap/ella-core/common/data/core.db"
interfaces: 
  n2:
    name: "ens3"
    address: "10.103.62.227"    # The IP address of the ella-core Multipass instance.
    port: 38412
  n3:
    name: "ens4"
    address: "10.243.161.26"    # The IP address of the radio Multipass instance.
  n6:
    name: "ens5"
  api:
    name: "lo"
    port: 5002
    tls:
      cert: "/var/snap/ella-core/common/cert.pem"
      key: "/var/snap/ella-core/common/key.pem"
```

Modify the highlighted values:

- `interfaces.n2.address`: The `ens3` IP address of the `ella-core` Multipass instance.
- `interfaces.n3.address`: The `ens4` IP address of the `ella-core` Multipass instance.

Start Ella Core:

```shell
sudo snap start ella-core.cored
```

Validate that Ella Core is running:

```shell
sudo snap services ella-core.cored
```

You should see that the service is active:

```shell
ubuntu@ella-core:~$ sudo snap services ella-core.cored 
Service          Startup   Current  Notes
ella-core.cored  disabled  active   -
```

Exit the Multipass instance:
```shell
exit
```

### Access the UI

Navigate to `https://<your instance IP>:5002` to access Ella Core's UI. Use the IP address you noted earlier.

You should see the Initialization page.

![Initialize Ella Core](../images/initialize.png){ align=center }

!!! note
    Your browser may display a warning about the security of the connection. This is because the certificate used by Ella Core is self-signed. You can safely ignore this warning.

### Initialize Ella Core

In the Initialization page, create the first user with the following credentials:

- Email: `admin@ellanetworks.com`
- Password: `admin`

After creating the user, you will be redirected to the login page. Use the credentials you just created to log in.

You will be redirected to the dashboard.

Ella Core is now initialized and ready to be used.

### Create a Profile and a Subscriber

Here, we will navigate through the Ella Core UI to create a profile, and a subscriber.

#### Create a Profile

Navigate to the `Profiles` page and click on the `Create` button.

Create a profile with the name `default`. You can keep the default values for the other parameters:

- Name: `default`
- UE IP Pool: `172.250.0.0/24`
- DNS: `8.8.8.8`
- MTU: `1500`
- Bitrate Uplink: `200 Mbps`
- Bitrate Downlink: `100 Mbps`

#### Create a subscriber

Navigate to the `Subscribers` page and click on the `Create` button.

Create a subscriber with the following parameters:

- IMSI: Click on "Generate" to automatically generate the MSIN.
- Key: Click on "Generate" to automatically generate a key.
- Sequence Number: Keep the default value.
- Profile: `default`

#### Validate that no radio is connected

Navigate to the `Radios` page. You should see that no radio is connected.

Exit the Multipass instance:

```shell
exit
```

## Install a 5G Radio Simulator

In this section, we will create another Multipass instance and install srsRAN, a 5G radio simulator. We will then connect the radio simulator to Ella Core.

### Setup the Virtual Machine where a radio simulator will be installed

Use Multipass to create a bare Ubuntu 24.04 instance:
```shell
multipass launch noble --name=radio --memory=4G --cpus 2
```

### Install and start srsRAN

Connect to the instance:
```shell
multipass shell radio
```

Install the srsRAN snap:
```shell
sudo snap install srsran5g --channel=edge
```

Connect the snap interfaces:

```shell
sudo snap connect srsran5g:kernel-module-observe
sudo snap connect srsran5g:network-control
sudo snap connect srsran5g:process-control
sudo snap connect srsran5g:system-observe
```

Create a configuration file for the radio and save it under `/var/snap/srsran5g/common/gnb2.yaml`:

```yaml hl_lines="3 5"
cu_cp:
  amf:
    addr: 10.103.62.227                 # The N2 interface address of Ella Core.
    port: 38412
    bind_addr: 10.103.62.196            # The local address to bind to (ens3).
    supported_tracking_areas:
      - tac: 1
        plmn_list:
          - plmn: "00101"
            tai_slice_support_list:
              - sst: 1
                sd: 1056816
  inactivity_timer: 7200

ru_sdr:
  device_driver: zmq
  device_args: tx_port=tcp://127.0.0.1:2000,rx_port=tcp://127.0.0.1:2001,base_srate=23.04e6
  srate: 23.04
  tx_gain: 75
  rx_gain: 75

cell_cfg:
  dl_arfcn: 368500
  band: 3
  channel_bandwidth_MHz: 20
  common_scs: 15
  plmn: "00101"
  tac: 1
  pdcch:
    common:
      ss0_index: 0
      coreset0_index: 12
    dedicated:
      ss2_type: common
      dci_format_0_1_and_1_1: false
  prach:
    prach_config_index: 1
  pdsch:
    mcs_table: qam64
  pusch:
    mcs_table: qam64

log:
  filename: /tmp/gnb.log
  all_level: info
  hex_max_size: 0

pcap:
  mac_enable: false
  mac_filename: /tmp/gnb_mac.pcap
  ngap_enable: false
  ngap_filename: /tmp/gnb_ngap.pcap
```

Modify the highlighted values:

- `cu_cp.amf.addr`: The `ens3` IP addresses the `ella-core` Multipass instance.
- `cu_cp.amf.bind_addr`: The `ens3` IP address of the `radio` Multipass instance.

Start the 5G radio:

```shell
srsran5g.gnb -c /var/snap/srsran5g/common/gnb.yaml
```

You should see the following output:

```shell
ubuntu@radio2:~$ srsran5g.gnb -c /var/snap/srsran5g/common/gnb.yaml 

--== srsRAN gNB (commit 9d5dd742a) ==--


The PRACH detector will not meet the performance requirements with the configuration {Format 0, ZCZ 0, SCS 1.25kHz, Rx ports 1}.
Lower PHY in executor blocking mode.
Available radio types: zmq.
Cell pci=1, bw=20 MHz, 1T1R, dl_arfcn=368500 (n3), dl_freq=1842.5 MHz, dl_ssb_arfcn=368410, ul_freq=1747.5 MHz

N2: Connection to AMF on 10.103.62.49:38412 completed
==== gNB started ===
Type <h> to view help

```

Leave the radio running.

On your browser, navigate to the Ella Core UI and click on the `Radios` tab. You should see the radio connected.

![Connected Radio](../images/connected_radio.png){ align=center }

## Destroy the Tutorial Environment

When you are done with the tutorial, you can destroy the Multipass instances:

```shell
multipass delete ella-core
multipass delete radio
```

You can also delete the two lxc networks:

```shell
lxc network delete n3
lxc network delete n6
```
