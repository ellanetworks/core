---
description: Explanation of how voice and other IMS services work in Ella Core.
---

# IMS Integration

Ella Core lets subscribers make voice and video calls over 4G (VoLTE) and 5G (VoNR). Calls are handled by an external IP Multimedia Subsystem (IMS), and Ella Core provides the subscriber data and the Quality of Service (QoS) the IMS needs. The same integration serves other IMS services, such as Mission Critical Push To Talk (MCPTT).

## What is an IMS?

An IMS is the system that sets up and routes calls in a mobile network. Phones talk to it using SIP, over a regular data session. Its main components are Call Session Control Functions (CSCFs):

- **P-CSCF (Proxy)**: The phone's first point of contact with the IMS.
- **I-CSCF (Interrogating)**: Finds the S-CSCF serving a subscriber.
- **S-CSCF (Serving)**: Registers subscribers and routes their calls.

Ella Core works with any 3GPP-compliant IMS. It has been validated against:

- [Ella IMS](https://github.com/ellanetworks/ims)
- [Kamailio](https://github.com/kamailio/kamailio), which has [known deviations from 3GPP and IETF specifications](https://github.com/kamailio/kamailio/issues/5027)
- [OpenSIPS](https://github.com/OpenSIPS/opensips), which has [known deviations from 3GPP and IETF specifications](https://github.com/OpenSIPS/opensips/issues/4386)

<figure markdown="span">
  ![Ella Core and the IMS](../images/voice_diagram.svg){ width="700" }
  <figcaption>Ella Core and the IMS</figcaption>
</figure>

## The role of Ella Core

Ella Core plays two roles for the IMS, both over Diameter:

| Role | Interface | Used for |
| ---- | --------- | -------- |
| HSS | Cx | Subscriber identities, authentication, the S-CSCF assigned to each subscriber, and the application servers that serve them. |
| PCRF / PCF | Rx | QoS for the media of each call. |

Both interfaces are used for 4G and 5G subscribers. The IMS connects to Ella Core's Diameter interface, over SCTP or TCP.

## IMS data networks

An IMS data network carries the phones' SIP signalling and call media. Any data network can be marked as IMS; there can be several, for example `ims` for phones and a separate one for mission critical devices.

- **P-CSCF discovery**: When a phone opens a session on an IMS data network, Ella Core gives it the P-CSCF addresses configured in the Operator settings, of the IP version the phone requested.
- **Signalling QoS**: The session's default QoS uses 5QI / QCI 5, the standard class for IMS signalling, or 69 for mission critical signalling.
- **No NAT**: Traffic on an IMS data network is never translated, so the IMS can reach phones at their own IP address.
- **UE-to-UE**: Traffic between two phones on an IMS data network is always switched locally, so calls between them work even when local switching is disabled.

## IMS identities

Ella Core derives each subscriber's IMS identities from its existing data. No IMS-specific provisioning is needed.

| Identity | Format | Example |
| -------- | ------ | ------- |
| Private identity (IMPI) | `<imsi>@<IMS home domain>` | `001010100007487@ims.mnc001.mcc001.3gppnetwork.org` |
| Public identity (IMPU) | `tel:+<msisdn>` | `tel:+15551230001` |
| Public identity (IMPU), barred | `sip:<private identity>` | `sip:001010100007487@ims.mnc001.mcc001.3gppnetwork.org` |

The IMS home domain is `ims.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, built from the operator's PLMN ID.

The `sip:` identity is barred, so it cannot be used for calls. A subscriber without an MSISDN has no usable public identity, and cannot make or receive calls.

## Voice support indication

During registration (5G) or attach and tracking area update (4G), Ella Core tells each phone whether IMS voice is supported. Phones rely on it to decide whether to use VoLTE or VoNR.

Ella Core indicates IMS voice support when all of the following hold:

- At least one P-CSCF address is configured.
- The data network named `ims`, the IMS well-known APN that phones use for voice (GSMA IR.92), exists and is marked as IMS, with an IP pool of the same version as one of the P-CSCF addresses.
- The subscriber's profile has a policy on the `ims` data network.
- The subscriber has an MSISDN.

A policy on another IMS data network, such as one for mission critical services, lets the subscriber register with the IMS but does not offer voice over PS.

On the Subscriber page, **Voice** shows the subscriber's voice state and, when voice is off, which condition is missing.

## Application servers and filter criteria

Services such as MCPTT run on application servers: SIP servers that the S-CSCF sends some of a subscriber's requests to. Each application server has:

- A **server name**: the SIP URI the S-CSCF sends requests to.
- **Public Service Identities (PSIs)**: the SIP or tel URIs the server hosts, either distinct or wildcarded to cover a range of identities. Requests addressed to a PSI go straight to its application server.

Each profile has an ordered list of **IMS initial filter criteria**. A filter criterion sends the requests of the profile's subscribers that match its trigger, such as a SIP method or header, to an application server. The S-CSCF evaluates them in the profile's order.

Changes to a profile's filter criteria, an application server's server name, or a subscriber's profile reach registered subscribers right away if the S-CSCF supports Cx Push Profile, and at their next registration otherwise. Changing or removing a registered subscriber's MSISDN de-registers it.

### MCPTT filter criteria

MCPTT needs two filter criteria, both to the MCPTT server:

1. **Registration**: tells the MCPTT server when a subscriber registers or de-registers. If the MCPTT server can't be reached, the IMS registration still succeeds.
2. **Originating MCPTT sessions**: sends the subscriber's MCPTT calls to the MCPTT server. If the MCPTT server can't be reached, the call fails.

Calls from the MCPTT server to subscribers need no filter criteria.

## Call setup and QoS

1. During call setup, the P-CSCF describes the call's media to Ella Core over Rx: its type, IP addresses, ports, and bandwidth.
2. Ella Core creates a guaranteed bitrate (GBR) QoS flow (5G) or dedicated bearer (4G) for the media, on the phone's IMS session.
3. When the call ends, the P-CSCF closes the Rx session and Ella Core releases the QoS flow or bearer.

| Media | 5QI / QCI | ARP priority |
| ----- | --------- | ------------ |
| Voice | 1 | 2 |
| Video | 2 | 4 |
| MCPTT voice and floor control | 65 | 1 to 8, from the call's Resource-Priority |

### MCPTT calls

An MCPTT call's voice and floor control share one GBR QoS flow or bearer with QCI/5QI 65. The session's default QoS flow or bearer on the MC data network uses QCI/5QI 69. A radio without 69 refuses the session, and one without 65 refuses the call.

The call's ARP comes from its Resource-Priority. Only a Resource-Priority set inside the IMS, such as by the MCPTT server, counts; one set by the client does not.

| Resource-Priority | ARP priority | Pre-emption capability | Pre-emption vulnerability |
| ----------------- | ------------ | ---------------------- | ------------------------- |
| `mcpttp.<n>` | 8 − ⌊n / 2⌋, e.g. 1 for `mcpttp.15` | May pre-empt | Not pre-emptable at priority 1 and 2, pre-emptable otherwise |
| `mcpttq.<n>` | 8 − ⌊n / 2⌋ | Shall not pre-empt | Not pre-emptable at priority 1 and 2, pre-emptable otherwise |
| None | 8 | Shall not pre-empt | Pre-emptable |

When a call's priority changes, for example from normal to emergency, Ella Core updates the ARP of its existing QoS flow or bearer.

If a radio pre-empts a session's default QoS flow or bearer, the whole session is released. This is why the MC data network's policy is usually not pre-emptable.

## EPS fallback

Some 5G radios are configured not to carry voice calls. When a voice call starts, such a radio refuses the voice QoS flow and moves the phone to 4G, where Ella Core sets up the call's QoS as a dedicated bearer.

## In an HA cluster

Each node of a [High Availability](high_availability.md) cluster is a separate Diameter peer of the IMS.

- **Cx**: Any node can answer. IMS registrations replicate across the cluster.
- **Rx**: The QoS for a call must be set up by the node that holds the phone's session. When the IMS reaches another node, that node redirects it to the right one.

## Further reading

- [Integrate with an IMS](../how_to/integrate_with_ims.md) — step-by-step guide to connect Ella Core to an IMS.
- [Enable Voice](../how_to/enable_voice.md) — step-by-step guide to enable voice and video calls.
- [Enable MCPTT](../how_to/enable_mcptt.md) — step-by-step guide to serve MCPTT.
- [Operator API reference](../reference/api/operator.md) — P-CSCF addresses, voice status, and IMS application servers.
- [Profiles API reference](../reference/api/profiles.md) — IMS initial filter criteria.
- [Subscribers API reference](../reference/api/subscribers.md) — IMS identities and registration state.
- [3GPP Compliance](../reference/3gpp_compliance.md) — supported Cx and Rx procedures.
