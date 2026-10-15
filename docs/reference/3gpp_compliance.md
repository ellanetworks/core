---
description: The 3GPP procedures, interfaces, and capabilities Ella Core implements.
---

# 3GPP Compliance

Ella Core implements 3GPP-standard interfaces for signalling.

!!! note

    Open an [enhancement proposal](https://github.com/ellanetworks/core/issues/new?template=enhancement_proposal.yml) if you notice a 3GPP-compliance gap or require a new capability for a production deployment.

## Supported

### Interfaces

| Interface | Transport |
| --------- | --------- |
| Cx | Diameter over SCTP or TCP |
| N1 | NAS, over N2 |
| N2 | NGAP over SCTP |
| N3 | GTP-U over UDP (IPv4 and IPv6) |
| N6 | IP |
| Rx | Diameter over SCTP or TCP |
| S1-MME | S1AP over SCTP |
| S1-U | GTP-U over UDP (IPv4 and IPv6) |
| S6c | Diameter over SCTP |
| SGd | Diameter over SCTP |
| SGi | IP |

Ella Core is a single binary and does not expose internal 3GPP interfaces. See [Architecture](../explanation/architecture.md).

### Registration and mobility

- **Registration.** 4G: attach, detach, and normal and periodic tracking area update. 5G: initial, mobility, and periodic registration, and UE- and network-initiated deregistration.
- **Service request.** An idle UE returns to connected mode to resume its session.
- **Paging.** Ella Core pages an idle UE when downlink data arrives for it.
- **Handover.** 4G: S1 handover, and X2 handover via the Path Switch procedure. 5G: Xn handover, and N2 handover between radios served by Ella Core.
- **4G/5G interworking.** A device moving between 4G and 5G keeps its IP address and its session.

### Sessions

Ella Core carries IP data sessions for 4G and 5G subscribers.

- **Session management.** 4G PDN connectivity and 5G PDU sessions: establishment, modification, and release, including network-requested procedures.
- **Session types.** IPv4, IPv6, and IPv4v6.
- **QoS.** One non-GBR QoS flow per session, plus GBR QoS flows (5G) and dedicated bearers (4G) for IMS voice and video.

### Security

- **Procedures.** Identity, authentication, and security mode control.
- **Authentication.** EPS-AKA on 4G, 5G-AKA on 5G.
- **Subscriber identity concealment.** SUCI with the null scheme, Profile A, and Profile B, on 5G.
- **Ciphering and integrity.** The null, SNOW 3G, and AES algorithms: EEA0/1/2 and EIA0/1/2 on 4G, NEA0/1/2 and NIA0/1/2 on 5G.

### SMS

- **SMS over NAS.** 4G (SMS in MME) and 5G.
- **Mobile-originated and mobile-terminated SMS.** Relayed to and from external SMSCs over SGd.
- **HSS for the SMSC.** Send Routing Info for SM, Report SM Delivery Status, and Alert Service Centre over S6c.

### Voice

- **IMS voice and video.** VoLTE on 4G and VoNR on 5G, with an external IMS.
- **HSS for the IMS.** User authorization, server assignment, location info, multimedia authentication, and registration termination over Cx.
- **PCRF for the IMS.** AA, re-auth, session termination, and abort session over Rx, with GBR QoS for voice and video.
- **EPS fallback.** A 5G voice call moves to 4G when the 5G radio requests it.

### Location (beta)

Cell identity and E-CID positioning: LPPa on 4G, NRPPa on 5G. GNSS positioning in standalone mode: LPP on 4G and 5G. See the [Location API](api/location.md).

## Limitations

- **No emergency services.** Emergency sessions and emergency service requests are rejected.
- **No roaming.** Ella Core does not support roaming between networks.
