# Roaming

Ella Core lets subscribers of partner networks attach over 4G and 5G, with Ella Core as the visited network. Their home network authenticates them and holds their subscription. Ella Core carries their traffic to its local data networks (local breakout).

## Local breakout

Ella Core supports local breakout for roaming subscribers, their traffic is routed to the local data networks instead of back to their home network.

### 5G

In 5G, Ella Core reaches the partner's 5G core through its SEPP, over N32. Each side trusts the other's certificate authority.

5G roaming

### 4G

In 4G, Ella Core reaches the partner's HSS over S6a, directly or through a Diameter Edge Agent.

4G roaming

## Roaming partners

A roaming partner is an agreement with another operator. It has:

- **PLMN IDs**: the partner's MCC and MNC. Ella Core recognizes the partner's subscribers by their IMSI.
- **A profile**: the policy applied to the partner's subscribers, like any [profile](https://docs.ellanetworks.com/core/reference/api/profiles/index.md).
- **A 5G agreement, a 4G agreement, or both**: how Ella Core reaches the partner's network.

## Roamers

A roamer is a subscriber from a partner network who is currently roaming in Ella Core's network.

- Roamers are listed apart from the subscribers, with their roaming partner.
- It appears when it registers and disappears when it leaves.
- Roamers count toward the maximum number of subscribers.
- Roamers get data service.
- Deleting a partner deregisters its roamers.

## In an HA cluster

Each node of a [High Availability](https://docs.ellanetworks.com/core/explanation/high_availability/index.md) cluster connects to the partner on its own, so the partner must accept every node. See [Roaming in an HA cluster](https://docs.ellanetworks.com/core/explanation/high_availability/#roaming).

## Further reading

- [Configure Roaming](https://docs.ellanetworks.com/core/how_to/configure_roaming/index.md) — step-by-step guide to add a roaming partner.
- [Integrations API reference](https://docs.ellanetworks.com/core/reference/api/integrations/#list-roaming-partners) — partners, Diameter peers, and SEPP.
- [Roamers API reference](https://docs.ellanetworks.com/core/reference/api/roamers/index.md) — list, inspect, and deregister roamers.
- [3GPP Compliance](https://docs.ellanetworks.com/core/reference/3gpp_compliance/index.md) — supported roaming procedures.
