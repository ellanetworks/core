---
description: Step-by-step instructions to integrate Ella Core with an external IMS.
---

# Integrate with an IMS

This guide connects Ella Core to an external IP Multimedia Subsystem (IMS), with Ella Core as the IMS's HSS and PCRF. It is the first part of [Enable Voice](enable_voice.md) and [Enable MCPTT](enable_mcptt.md). To learn how Ella Core works with the IMS, see [IMS Integration](../explanation/ims_integration.md).

This guide uses [Ella IMS](https://github.com/ellanetworks/ims), but any 3GPP-compliant IMS can be used. For the Ella IMS side, see [Integrate with Ella Core](https://docs.ellanetworks.com/ims/how_to/integrate_with_ella_core/).

## 1. Enable the Diameter interface

In the Ella Core configuration file, set the address Ella Core accepts Diameter connections on:

```yaml
interfaces:
  diameter:
    address: "10.3.0.2"
```

In a [high availability cluster](deploy_ha_cluster.md), use the same `interfaces.diameter.port` on every node.

Restart Ella Core.

## 2. Configure the IMS

In the Ella Core UI, go to **Integrations › IMS**. Under **This Core**, note the **Diameter Realm**, e.g. `epc.mnc001.mcc001.3gppnetwork.org`, each node's **Origin-Host**, the **Diameter Address**, and the **Home Domain**.

Configure your IMS with the following settings:

- **HSS (Cx)**: Ella Core's **Diameter Address**, e.g. `10.3.0.2`, port `3868`.
- **PCRF (Rx)**: Ella Core's **Diameter Address**, e.g. `10.3.0.2`, port `3868`.
- **HSS and PCRF realm**: Ella Core's **Diameter Realm**, or its **Home Domain**.
- **Home domain**: Ella Core's **Home Domain**, e.g. `ims.mnc001.mcc001.3gppnetwork.org`.

To use another home domain, e.g. `ims.example.org`, click the edit icon next to **Home Domain**, enter it, click **Update**, then **Confirm**. Set the same home domain on your subscribers' ISIMs.

## 3. Allow your IMS nodes

In **Integrations › IMS**, under **Allowed IMS Peers**, click **Add Allowed Peer** and enter:

- **Name**: e.g. `cscf`.
- **Origin-Host** of an IMS node, e.g. `scscf.ims.mnc001.mcc001.3gppnetwork.org`, or the **Realm** of your IMS nodes, e.g. `ims.mnc001.mcc001.3gppnetwork.org`, or both.
- **Source Addresses**: the IP addresses or CIDR prefixes your IMS nodes connect from, e.g. `10.3.0.0/24`. At least one is required.

Click **Add**. Repeat for each group of IMS nodes: I-CSCF, S-CSCF, P-CSCF and application servers.

## 4. Restrict Diameter access to your IMS nodes

In **Integrations › IMS**, under **This Core**, click the edit icon next to **Restrict to Allowed Peers**, turn it on, and click **Update**.

## 5. Set the P-CSCF addresses

In **Integrations › IMS**, click the edit icon next to **P-CSCF Addresses**, enter your IMS's P-CSCF addresses or host names, e.g. `10.6.0.5`, and click **Update**.

## 6. Route the IMS data networks

In your IMS, add a route to the IP pools of your IMS data networks via Ella Core's N6 address, with an MTU of 1400, e.g. `ip route add 10.0.0.0/8 via 10.6.0.2 mtu lock 1400`.