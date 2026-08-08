# 3GPP Compliance

Ella Core implements 3GPP-standard interfaces for 4G and 5G SA.

Note

Need a procedure or capability that is not supported for a production deployment? Open an [enhancement proposal](https://github.com/ellanetworks/core/issues/new?template=enhancement_proposal.yml) and tell us about your use case.

## Supported

### Interfaces

| Interface | Transport                      |
| --------- | ------------------------------ |
| N1        | NAS, over N2                   |
| N2        | NGAP over SCTP                 |
| N3        | GTP-U over UDP (IPv4 and IPv6) |
| N6        | IP                             |
| S1-MME    | S1AP over SCTP                 |
| S1-U      | GTP-U over UDP (IPv4 and IPv6) |
| SGi       | IP                             |

Ella Core is a single binary and does not expose internal 3GPP interfaces. See [Architecture](https://docs.ellanetworks.com/explanation/architecture/index.md).

### Registration and mobility

- **Registration.** 4G: attach, detach, and normal and periodic tracking area update. 5G: initial, mobility, and periodic registration, and UE- and network-initiated deregistration.
- **Service request.** An idle UE returns to connected mode to resume its session.
- **Paging.** Ella Core pages an idle UE when downlink data arrives for it.
- **Handover.** 4G: S1 handover, and X2 handover via the Path Switch procedure. 5G: Xn handover, and N2 handover between radios served by Ella Core.

### Sessions

Ella Core carries IP data sessions for 4G and 5G subscribers.

- **Session management.** 4G PDN connectivity and 5G PDU sessions: establishment, modification, and release, including network-requested procedures.
- **Session types.** IPv4, IPv6, and IPv4v6.
- **QoS.** One non-GBR QoS flow per session.

### Security

- **Procedures.** Identity, authentication, and security mode control.
- **Authentication.** EPS-AKA on 4G, 5G-AKA on 5G.
- **Subscriber identity concealment.** SUCI with the null scheme, Profile A, and Profile B, on 5G.
- **Ciphering and integrity.** The null, SNOW 3G, and AES algorithms: EEA0/1/2 and EIA0/1/2 on 4G, NEA0/1/2 and NIA0/1/2 on 5G.

### Location

Cell identity and E-CID positioning: LPPa on 4G, NRPPa on 5G. See the [Location API](https://docs.ellanetworks.com/reference/api/location/index.md), which is beta.

## Limitations

- **No voice.** Ella Core provides no IMS, VoLTE, or VoNR.
- **No emergency services.** Emergency sessions and emergency service requests are rejected.
- **No roaming.** Ella Core is a self-contained core for a single network; there is no S6a, S8, or inter-operator interface.
