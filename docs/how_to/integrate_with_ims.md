---
description: Step-by-step instructions to integrate Ella Core with an external IMS.
---

# Integrate with an IMS

This guide connects Ella Core to an external IP Multimedia Subsystem (IMS), with Ella Core as the IMS's HSS and PCRF. It is the first part of [Enable Voice](enable_voice.md) and [Enable MCPTT](enable_mcptt.md). To learn how Ella Core works with the IMS, see [IMS Integration](../explanation/ims_integration.md).

This guide uses [Ella IMS](https://github.com/ellanetworks/ims), but any 3GPP-compliant IMS can be used.

## 1. Enable the Diameter interface

In the Ella Core configuration file, set the address Ella Core accepts Diameter connections on:

```yaml
interfaces:
  diameter:
    address: "10.3.0.2"
```

In a [high availability cluster](deploy_ha_cluster.md), use the same `port` on every node.

Restart Ella Core.

## 2. Configure the IMS

Configure your IMS with the following settings, using the MCC and 3-digit MNC of Ella Core's Operator settings:

- **HSS (Cx)**: Ella Core's Diameter address, e.g. `10.3.0.2`, port `3868`.
- **PCRF (Rx)**: Ella Core's Diameter address, e.g. `10.3.0.2`, port `3868`.
- **HSS and PCRF realm**: `epc.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, e.g. `epc.mnc001.mcc001.3gppnetwork.org`.
- **Home domain**: `ims.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, e.g. `ims.mnc001.mcc001.3gppnetwork.org`.

## 3. Set the P-CSCF addresses

In the Ella Core UI, go to the Operator page and scroll to the **IMS** section.

Click the edit icon next to **P-CSCF Addresses**, enter your IMS's P-CSCF addresses, e.g. `10.6.0.5`, and click **Update**.

## 4. Route the IMS data networks

In your IMS, add a route to the IP pools of your IMS data networks via Ella Core's N6 address, with an MTU of 1400, e.g. `ip route add 10.0.0.0/8 via 10.6.0.2 mtu lock 1400`.