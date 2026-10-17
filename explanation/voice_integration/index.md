# Voice Integration

Ella Core lets subscribers make voice and video calls over 4G (VoLTE) and 5G (VoNR). Calls are handled by an external IP Multimedia Subsystem (IMS), and Ella Core provides the subscriber data and the Quality of Service (QoS) the IMS needs.

## What is an IMS?

An IMS is the system that sets up and routes calls in a mobile network. Phones talk to it using SIP, over a regular data session. Its main components are Call Session Control Functions (CSCFs):

- **P-CSCF (Proxy)**: The phone's first point of contact with the IMS.
- **I-CSCF (Interrogating)**: Finds the S-CSCF serving a subscriber.
- **S-CSCF (Serving)**: Registers subscribers and routes their calls.

Ella Core works with any 3GPP-compliant IMS, for example [Ella IMS](https://github.com/ellanetworks/ims).

Ella Core and the IMS

## The role of Ella Core

Ella Core plays two roles for the IMS, both over Diameter:

| Role       | Interface | Used for                                                                           |
| ---------- | --------- | ---------------------------------------------------------------------------------- |
| HSS        | Cx        | Subscriber identities, authentication, and the S-CSCF assigned to each subscriber. |
| PCRF / PCF | Rx        | QoS for the media of each call.                                                    |

Both interfaces are used for 4G and 5G subscribers. The IMS connects to Ella Core's Diameter interface, over SCTP or TCP.

Ella Core also carries the phone's traffic to and from the IMS, on a dedicated data network named `ims`.

## The `ims` data network

The `ims` data network carries the phones' SIP signalling and call media.

- **P-CSCF discovery**: When a phone opens a session on `ims`, Ella Core gives it the P-CSCF addresses configured in the Operator settings, of the IP version the phone requested.
- **Signalling QoS**: The session's default QoS uses 5QI / QCI 5, the standard class for IMS signalling.
- **No NAT**: Traffic on `ims` is never translated, so the IMS can reach phones at their own IP address.
- **UE-to-UE**: Traffic between two phones on `ims` is always switched locally, so calls between them work even when local switching is disabled.

## IMS identities

Ella Core derives each subscriber's IMS identities from its existing data. No IMS-specific provisioning is needed.

| Identity                       | Format                     | Example                                                 |
| ------------------------------ | -------------------------- | ------------------------------------------------------- |
| Private identity (IMPI)        | `<imsi>@<IMS home domain>` | `001010100007487@ims.mnc001.mcc001.3gppnetwork.org`     |
| Public identity (IMPU)         | `tel:+<msisdn>`            | `tel:+15551230001`                                      |
| Public identity (IMPU), barred | `sip:<private identity>`   | `sip:001010100007487@ims.mnc001.mcc001.3gppnetwork.org` |

The IMS home domain is `ims.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, built from the operator's PLMN ID.

The `sip:` identity is barred, so it cannot be used for calls. A subscriber without an MSISDN has no usable public identity, and cannot make or receive calls.

## Voice support indication

During registration (5G) or attach and tracking area update (4G), Ella Core tells each phone whether IMS voice is supported. Phones rely on it to decide whether to use VoLTE or VoNR.

Ella Core indicates IMS voice support when all of the following hold:

- At least one P-CSCF address is configured.
- The `ims` data network exists, with an IP pool of the same version as one of the P-CSCF addresses.
- The subscriber's profile has a policy on the `ims` data network.
- The subscriber has an MSISDN.

On the Subscriber page, **Voice** shows the subscriber's voice state and, when voice is off, which condition is missing.

## Call setup and QoS

1. During call setup, the P-CSCF describes the call's media to Ella Core over Rx: its type, IP addresses, ports, and bandwidth.
1. Ella Core creates a guaranteed bitrate (GBR) QoS flow (5G) or dedicated bearer (4G) for the media, on the phone's `ims` session.
1. When the call ends, the P-CSCF closes the Rx session and Ella Core releases the QoS flow or bearer.

| Media | 5QI / QCI | ARP priority |
| ----- | --------- | ------------ |
| Voice | 1         | 2            |
| Video | 2         | 4            |

## EPS fallback

Some 5G radios are configured not to carry voice calls. When a voice call starts, such a radio refuses the voice QoS flow and moves the phone to 4G, where Ella Core sets up the call's QoS as a dedicated bearer.

## In an HA cluster

Each node of a [High Availability](https://docs.ellanetworks.com/explanation/high_availability/index.md) cluster is a separate Diameter peer of the IMS.

- **Cx**: Any node can answer. IMS registrations replicate across the cluster.
- **Rx**: The QoS for a call must be set up by the node that holds the phone's session. When the IMS reaches another node, that node redirects it to the right one.

## Further reading

- [Enable Voice](https://docs.ellanetworks.com/how_to/enable_voice/index.md) — step-by-step guide to enable voice and video calls.
- [Operator API reference](https://docs.ellanetworks.com/reference/api/operator/index.md) — P-CSCF addresses and voice status.
- [Subscribers API reference](https://docs.ellanetworks.com/reference/api/subscribers/index.md) — IMS identities and registration state.
- [3GPP Compliance](https://docs.ellanetworks.com/reference/3gpp_compliance/index.md) — supported Cx and Rx procedures.
