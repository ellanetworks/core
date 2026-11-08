---
description: Step-by-step instructions to let subscribers of a partner network roam on Ella Core.
---

# Configure Roaming

This page has two guides: 5G roaming and 4G roaming. Each adds a roaming partner, so the partner's subscribers can attach to Ella Core. To learn how roaming works, see [Roaming](../explanation/roaming.md).

## 5G roaming

### Prerequisites

- **Data networks and slices**: a data network named after each DNN that the partner's subscribers use, e.g. `internet` (names match regardless of letter case), and slices with the S-NSSAI values of their subscriptions, each serving these data networks.
- **Partner subscriptions**: the partner allows local breakout for these data networks in its subscriptions.

### 1. Enable the N32 interface

On each node, set `interfaces.n32` in the [configuration file](../reference/config_file.md) to the interface that reaches the partner's SEPP:

```yaml
interfaces:
  n32:
    address: "192.0.2.11"
    port: 443
```

Restart Ella Core.

### 2. Exchange information with the partner

On the **Operator** page, the **Certificate** row of the **Roaming** section shows each node's SEPP FQDN. The **N32 (SEPP)** card of the **Networking** page's **Interfaces** tab shows each node's N32 address.

Give the partner:

| Item | Example |
| ---- | ------- |
| Your PLMN ID | `001/01` |
| Each node's SEPP FQDN and N32 address | `sepp1.5gc.mnc001.mcc001.3gppnetwork.org`, `192.0.2.11:443` |
| The trust anchors, downloaded from the **Trust anchor** row | `sepp-ca.pem` |

Get from the partner:

| Item | Example |
| ---- | ------- |
| Its PLMN IDs | `999/70` |
| Its SEPP FQDN | `sepp1.5gc.mnc070.mcc999.3gppnetwork.org` |
| Its N32-c URI | `https://sepp1.5gc.mnc070.mcc999.3gppnetwork.org:443` |
| Its trust anchor | `partner-ca.pem` |
| The NRF API root of each of its PLMN IDs | `https://nrf.5gc.mnc070.mcc999.3gppnetwork.org` |

Configure DNS so that Ella Core resolves the partner's SEPP FQDN, and the partner resolves each node's SEPP FQDN.

### 3. Create a profile for roamers

Go to the **Profiles** page and create a profile named `roamers`, with **5G** access.

Add a policy on each data network that roamers may use, e.g. `internet`.

### 4. Add the partner

On the **Operator** page, in the **Roaming** section, open **Partners** and click **Add Partner**:

- **Name**: e.g. `Partner Telecom`.
- **MCC** and **MNC**: the partner's PLMN ID, e.g. `999` and `70`. Click **Add PLMN** for each other PLMN ID of the partner.
- **Profile**: `roamers`.
- **5G Roaming**: turn on, and enter the partner's **SEPP FQDN**, **N32-c URI**, **Trust Anchor**, and the **NRF API Root** of each PLMN ID.

Click **Create**.

### 5. Verify

In **Partners** on each node, the **N32** column shows **Established**.

Register a 5G UE with a SIM of the partner's network, with data roaming on. Open the **Roamers** tab of the **Subscribers** page of the node that serves it: the roamer appears, with its roaming partner.

## 4G roaming

### Prerequisites

- **Data networks**: a data network named after each APN that the partner's subscribers use, e.g. `internet` (names match regardless of letter case), served on a slice.
- **Partner subscriptions**: the partner allows local breakout for these APNs in its subscriptions.

### 1. Exchange information with the partner

Give the partner:

| Item | Example |
| ---- | ------- |
| Your PLMN ID | `001/01` |
| Each node's Diameter identity, from its [Diameter status](../reference/api/networking.md#get-diameter-status) | `mmec01.mmegi0001.mme.epc.mnc001.mcc001.3gppnetwork.org` |
| Each node's Diameter address and port (`interfaces.diameter`) | `192.0.2.11`, `3868` |

Get from the partner:

| Item | Example |
| ---- | ------- |
| Its PLMN IDs | `999/70` |
| The address and SCTP port of its HSS or Diameter Edge Agent | `198.51.100.5`, `3868` |

### 2. Create a profile for roamers

Go to the **Profiles** page and create a profile named `roamers`, with **4G** access.

Add a policy on each data network that roamers may use, e.g. `internet`.

### 3. Add the Diameter peer

On the **Operator** page, in the **Roaming** section, open **Diameter peers** and click **Add Diameter Peer**:

- **Address**: the partner's HSS or Diameter Edge Agent, e.g. `198.51.100.5`.
- **Port**: e.g. `3868`.

Click **Create**.

### 4. Add the partner

Open **Partners** and click **Add Partner**:

- **Name**: e.g. `Partner Telecom`.
- **MCC** and **MNC**: the partner's PLMN ID, e.g. `999` and `70`. Click **Add PLMN** for each other PLMN ID of the partner.
- **Profile**: `roamers`.
- **4G Roaming**: turn on, and select the Diameter peer from step 3.

Click **Create**.

### 5. Verify

The partner's Diameter peer is `open` in each node's [Diameter status](../reference/api/networking.md#get-diameter-status).

Attach a 4G UE with a SIM of the partner's network, with data roaming on. Open the **Roamers** tab of the **Subscribers** page of the node that serves it: the roamer appears, with its roaming partner.

!!! note
    All steps in this guide can also be performed via the REST API. See the [Operator API reference](../reference/api/operator.md#list-roaming-partners) for details.
