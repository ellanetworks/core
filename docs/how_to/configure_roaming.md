---
description: Step-by-step instructions to let subscribers of a partner network roam on Ella Core.
---

# Configure Roaming

This guide adds a roaming partner, so the partner's subscribers can attach to Ella Core over 4G and 5G. To learn how roaming works, see [Roaming](../explanation/roaming.md).

## Prerequisites

- **Data networks and slices**: a data network named after each DNN or APN that the partner's subscribers use, e.g. `internet` (names match regardless of letter case), and slices with the S-NSSAI values of their subscriptions. Ella Core does not map S-NSSAIs between networks, so the partner must use standard S-NSSAIs that match a local slice.
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

On the **Operator** page of each node, go to the **Roaming** section. Its **Certificate** row shows the node's SEPP FQDN. The **N32 (SEPP)** card of the **Networking** page's **Interfaces** tab shows the node's N32 address.

Give the partner:

| Item | Example |
| ---- | ------- |
| Your PLMN ID | `001/01` |
| Each node's SEPP FQDN and N32 address | `sepp1.5gc.mnc001.mcc001.3gppnetwork.org`, `192.0.2.11:443` |
| The trust anchors, downloaded from the **Trust anchor** row | `sepp-ca.pem` |
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

On the **Operator** page, in the **Roaming** section, open **Diameter peers** and click **Create**:

- **Address**: the partner's HSS or Diameter Edge Agent, e.g. `198.51.100.5`.
- **Port**: e.g. `3868`.

Click **Create**.

## 5. Add the partner

Open **Partners** and click **Create**:

- **Name**: e.g. `Partner Telecom`.
- **MCC** and **MNC**: the partner's PLMN ID, e.g. `999` and `70`. Click **Add PLMN** for each other PLMN ID of the partner.
- **Profile**: `roamers`.
- **5G Roaming**: turn on, and enter the partner's **SEPP FQDN**, **N32-c URI**, and **Trust Anchor**.
- **4G Roaming**: turn on, and select the Diameter peer from step 4.

Click **Create**.

## 6. Verify

In **Partners** on each node, the **N32** column shows **Established**.

Attach a UE with a SIM of the partner's network. Open the **Subscribers** page of the node that serves it: the roamer appears, with its roaming partner.

!!! note
    All steps in this guide can also be performed via the REST API. See the [Operator API reference](../reference/api/operator.md#list-roaming-partners) for details. Generating a node's SEPP certificate again and regenerating the SEPP CA are also available through the API: see [Generate a SEPP Certificate](../reference/api/operator.md#generate-a-sepp-certificate) and [Regenerate the SEPP CA](../reference/api/operator.md#regenerate-the-sepp-ca).
