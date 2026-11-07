---
description: Step-by-step instructions to run Ella Core, Ella IMS and Ella SMSC on a single host.
---

# Co-host with Ella IMS and Ella SMSC

This guide runs [Ella IMS](https://docs.ellanetworks.com/ims/) and [Ella SMSC](https://docs.ellanetworks.com/smsc/) on the same host as Ella Core. Ella IMS runs in its own network namespace, which Ella Core reaches through an N6 network instance.

## Prerequisites

To follow this guide, you will need:

- A host with Ella Core installed with the snap, as in [Install](install.md), and an `eth0` interface towards the radio and the internet.
- A free address on the LAN for Ella IMS's UI.

The examples use these values:

| Value | Role |
|---|---|
| `eth0` | Host interface: N2, N3, N6, and the Ella Core and Ella SMSC UIs |
| `10.3.0.2` | Ella Core's Diameter address |
| `10.3.0.7` | Ella SMSC's Diameter address |
| `10.6.0.2` | Ella Core's address on the `ims` network instance |
| `10.6.0.5` | Ella IMS's SIP and Diameter address |
| `192.168.1.50/24` | Ella IMS's UI address on the LAN |
| `192.168.1.1` | LAN gateway |
| `10.46.0.0/16` | IP pool of the `ims` data network |
| `001` / `01` | Ella Core's MCC and MNC |
| `epc.mnc001.mcc001.3gppnetwork.org` | Ella Core's Diameter realm |
| `+15550000000` | Ella SMSC's service centre address |
| `15550000100` | Ella IMS's IP-SM-GW number |

## 1. Create the Ella IMS network namespace

Create `/etc/systemd/system/ims-netns.service`:

```ini
[Unit]
Description=Ella IMS network namespace
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
RemainAfterExit=true

ExecStartPre=-ip netns delete ims
ExecStartPre=-ip link delete diameter0
ExecStart=ip link add diameter0 type dummy
ExecStart=ip addr add 10.3.0.2/32 dev diameter0
ExecStart=ip addr add 10.3.0.7/32 dev diameter0
ExecStart=ip link set diameter0 up
ExecStart=ip netns add ims
ExecStart=ip link add n6-ims type veth peer name ims0
ExecStart=ip link set ims0 netns ims
ExecStart=ip addr add 10.6.0.2/24 dev n6-ims
ExecStart=ip link set n6-ims up
ExecStart=ip link add lan0 link eth0 type macvlan mode bridge
ExecStart=ip link set lan0 netns ims
ExecStart=ip -n ims link set lo up
ExecStart=ip -n ims addr add 10.6.0.5/24 dev ims0
ExecStart=ip -n ims link set ims0 up
ExecStart=ip -n ims addr add 192.168.1.50/24 dev lan0
ExecStart=ip -n ims link set lan0 up
ExecStart=ip -n ims route add default via 192.168.1.1 dev lan0
ExecStart=ip -n ims route add 10.3.0.0/24 via 10.6.0.2 dev ims0
ExecStart=ip -n ims route add 10.46.0.0/16 via 10.6.0.2 dev ims0 mtu lock 1400
ExecStart=ip netns exec ims ethtool -K ims0 tso off gso off tx off
ExecStart=ethtool -K eth0 gro off
ExecStop=-ip netns delete ims
ExecStop=-ip link delete diameter0

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```shell
sudo systemctl daemon-reload
sudo systemctl enable --now ims-netns.service
```

## 2. Configure Ella Core

Edit `/var/snap/ella-core/common/core.yaml`:

```yaml hl_lines="15-18 22-23 25"
logging:
  system:
    level: "info"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "/var/snap/ella-core/common/data/ella.db"
interfaces:
  n2:
    name: "eth0"
  n3:
    name: "eth0"
  n6:
    name: "eth0"
  network-instances:
    - name: "ims"
      n6:
        name: "n6-ims"
  api:
    address: "0.0.0.0"
    port: 5002
  diameter:
    address: "10.3.0.2"
datapath:
  attach-mode: "tcx"
telemetry:
  enabled: false
```

Create `/etc/systemd/system/snap.ella-core.cored.service.d/override.conf`:

```ini
[Unit]
Requires=ims-netns.service
After=ims-netns.service
```

Restart Ella Core:

```shell
sudo systemctl daemon-reload
sudo snap restart ella-core.cored
```

## 3. Install Ella IMS in the namespace

Install Ella IMS:

```shell
sudo snap install ella-ims
sudo snap connect ella-ims:network-control
```

Edit `/var/snap/ella-ims/common/ims.yaml` and set its addresses:

```yaml
api:
  address: 0.0.0.0
  port: 5020
sip:
  addresses: [10.6.0.5]
diameter:
  address: 10.6.0.5
```

Create `/etc/systemd/system/snap.ella-ims.imsd.service.d/override.conf`:

```ini
[Unit]
Requires=ims-netns.service
After=ims-netns.service

[Service]
NetworkNamespacePath=/var/run/netns/ims
```

Start Ella IMS:

```shell
sudo systemctl daemon-reload
sudo snap start --enable ella-ims.imsd
```

Ella IMS's UI is at `http://192.168.1.50:5020`.

## 4. Install Ella SMSC

Install Ella SMSC:

```shell
sudo snap install ella-smsc
```

Edit `/var/snap/ella-smsc/common/smsc.yaml` and set its Diameter address:

```yaml
diameter:
  address: 10.3.0.7
```

Create `/etc/systemd/system/snap.ella-smsc.smscd.service.d/override.conf`:

```ini
[Unit]
Requires=ims-netns.service
After=ims-netns.service
```

Start Ella SMSC:

```shell
sudo systemctl daemon-reload
sudo snap start --enable ella-smsc.smscd
```

Ella SMSC's UI is at `http://<eth0 address>:5010`.

## 5. Connect Ella IMS to Ella Core

In the Ella IMS UI, go to the **Operator** page:

- Click the edit icon next to **Operator ID (MCC/MNC)**, enter `001` / `01`, and click **Update**.
- Click the edit icon next to **Numbering Plan**, enter your country code and prefixes, and click **Update**.

Go to the **Cores** page:

- Under **Peers**, click **Add Peer**, enter `10.3.0.2`, check **HSS** and **PCRF**, and click **Add**.
- Under **HSS**, click the edit icon next to **HSS Realm**, enter `epc.mnc001.mcc001.3gppnetwork.org`, and click **Update**.
- Under **Voice QoS**, click the edit icon next to **Policy Function**, select **PCRF**, and click **Update**.
- Click the edit icon next to **PCRF Realm**, enter `epc.mnc001.mcc001.3gppnetwork.org`, and click **Update**.

## 6. Connect Ella SMSC to Ella Core

In the Ella SMSC UI, go to the **Operator** page:

- Click the edit icon next to **Operator ID (MCC/MNC)**, enter `001` / `01`, and click **Update**.
- Click the edit icon next to **Service Centre Address**, enter `+15550000000`, and click **Update**.
- Click the edit icon next to **Numbering**, enter your country code and prefixes, and click **Update**.

Go to the **Diameter** page and note the **Host**, e.g. `smsc.node.epc.mnc001.mcc001.3gppnetwork.org`.

In the Ella Core UI, go to **Operator**, scroll to the **SMS** section, click **Add SMSC**, and enter:

- **Name**: `smsc`
- **Numbers**: `+15550000000`
- **Address**: `10.3.0.7`
- **Diameter Identity**: Ella SMSC's **Host**.

Click **Add**.

## 7. Enable SMS over IP

In the Ella IMS UI, go to the **Cores** page:

- Under **SMS over IP**, click the edit icon next to **IP-SM-GW Number**, enter `15550000100`, and click **Update**. Note the **IP-SM-GW Name**.
- Under **Peers**, click **Add Peer**, and enter:
    - **Address**: `10.3.0.7`
    - **Roles**: **SGd** only
    - **Service Centres**: `15550000000`
    - **Transport**: **SCTP**
    - **Host**: Ella SMSC's **Host**.
- Click **Add**.
- Click the edit icon of the `10.3.0.2` peer, check **Sh**, **S6c** and **SGd**, and click **Update**.

In the Ella Core UI, go to **Operator** and scroll to the **IMS** section:

- Click **Add Application Server**, enter `ip-sm-gw` as the **Name** and Ella IMS's **IP-SM-GW Name** as the **Server Name**, and click **Add**.
- Click **Add Service Profile**, enter `sms-over-ip` as the **Name**. Under **Add from preset**, select the **SMS over IP** preset and the `ip-sm-gw` **Application Server**, click **Add Criteria**, and click **Add**.

## 8. Route the `ims` data network to Ella IMS

In the Ella Core UI:

1. Go to **Networking > Data Networks**, click **Create**, enter `ims` as the **Name** and `10.46.0.0/16` as the **IPv4 Pool**, turn on **IMS (voice and mission critical services)**, and click **Create**.
2. Open the `ims` data network. In **Slices**, select the slice of your subscribers and click **Serve on slice**, then set its **Network instance** to `ims`.
3. Go to **Operator**, scroll to the **IMS** section, click the edit icon next to **P-CSCF Addresses**, enter `10.6.0.5`, and click **Update**.

## 9. Set up the subscribers

In the Ella Core UI, for each profile whose subscribers make calls and send SMS:

- Go to the **Profiles** page and open the profile.
- Click **Add Policy**, enter a **Name**, set the **Data Network** to `ims`, and click **Create**.
- Click the edit icon next to **IMS Service Profiles**, set **MMTel (voice, SMS)** to `sms-over-ip`, and click **Save**.

For each subscriber, go to the **Subscribers** page, open the subscriber, click the edit icon next to **MSISDN**, enter its E.164 number, e.g. `+15551230001`, and click **Update**.

On each SIM card, set the SMSC number to `+15550000000`. On each phone, turn on VoLTE (4G) or VoNR (5G).

## 10. Verify

- In the Ella Core UI, go to **Networking > Interfaces**: the **N6 (External) · ims** card shows `n6-ims`, used by `ims`.
- In the Ella IMS UI, go to **Cores**: under **Peers**, `10.3.0.2` and `10.3.0.7` show **open**.
- In the Ella SMSC UI, go to **Diameter**: under **Connected Peers**, Ella Core and Ella IMS show **open**.
- In the Ella Core UI, go to **Subscribers** and open a subscriber with an MSISDN: **Voice** shows **Registered**.
- Call a phone from another. The call connects with audio in both directions.
- Send an SMS between two phones. In the Ella SMSC UI, on the **Messages** page, it shows **delivered**.
