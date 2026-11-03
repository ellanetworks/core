---
description: Step-by-step instructions to let subscribers of a partner network roam on Ella Core.
---

# Configure Roaming

This guide adds a roaming partner, so the partner's subscribers can attach to Ella Core over 4G and 5G. To learn how roaming works, see [Roaming](../explanation/roaming.md).

## Prerequisites

- **Data networks and slices**: a data network named after each DNN or APN that the partner's subscribers use, e.g. `internet`, and slices with the S-NSSAI values of their subscriptions.
- **Partner subscriptions**: the partner allows local breakout for these data networks in its subscriptions.

## 1. Create a profile for roamers

Go to the **Profiles** page and create a profile named `roamers`, with **4G** and **5G** access.

Add a policy on each data network that roamers may use, e.g. `internet`.

## 2. Enable the N32 interface

On each node, set `interfaces.n32` in the [configuration file](../reference/config_file.md) to the interface that reaches the partner's SEPP:

```yaml
interfaces:
  n32:
    address: "192.0.2.11"
    port: 443
```

Restart Ella Core.

## 3. Exchange information with the partner

On the **Roaming** page, open the **SEPP** tab. Each node appears with its SEPP FQDN and N32 address.

Give the partner:

| Item | Example |
| ---- | ------- |
| Your PLMN ID | `001/01` |
| Each node's SEPP FQDN and N32 address | `sepp1.5gc.mnc001.mcc001.3gppnetwork.org`, `192.0.2.11:443` |
| The trust anchors, downloaded from the **SEPP** tab | `sepp-ca.pem` |
| Each node's Diameter identity, from its [Diameter status](../reference/api/networking.md#get-diameter-status) | `mmec01.mmegi0001.mme.epc.mnc001.mcc001.3gppnetwork.org` |

Get from the partner:

| Item | Example |
| ---- | ------- |
| Its PLMN IDs | `999/70` |
| Its SEPP FQDN | `sepp1.5gc.mnc070.mcc999.3gppnetwork.org` |
| Its N32-c URI | `https://sepp1.5gc.mnc070.mcc999.3gppnetwork.org:443` |
| Its trust anchor | `partner-ca.pem` |
| The address and SCTP port of its HSS or Diameter Edge Agent | `198.51.100.5`, `3868` |

## 4. Add the Diameter peer

On the **Roaming** page, open the **Diameter Peers** tab and click **Create**:

- **Address**: the partner's HSS or Diameter Edge Agent, e.g. `198.51.100.5`.
- **Port**: e.g. `3868`.

Click **Create**.

## 5. Add the partner

Open the **Partners** tab and click **Create**:

- **Name**: e.g. `Partner Telecom`.
- **MCC** and **MNC**: the partner's PLMN ID, e.g. `999` and `70`. Click **Add PLMN** for each other PLMN ID of the partner.
- **Profile**: `roamers`.
- **5G Roaming**: turn on, and enter the partner's **SEPP FQDN**, **N32-c URI**, and **Trust Anchor**.
- **4G Roaming**: turn on, and select the Diameter peer from step 4.

Click **Create**.

## 6. Verify

On the **Partners** tab, the **N32** column shows **Established** for each node.

Attach a UE with a SIM of the partner's network. Open the **Roamers** tab: the roamer appears.

!!! note
    All steps in this guide can also be performed via the REST API. See the [Roaming API reference](../reference/api/roaming.md) for details.
