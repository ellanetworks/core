# v1.20

- **Release date**: October 19, 2026
- **GitHub release**: [v1.20.0](https://github.com/ellanetworks/core/releases/tag/v1.20.0)

## New features

- **Voice**: Subscribers can make voice and video calls over 4G (VoLTE) and 5G (VoNR) using an external IMS (ex. [Ella IMS](https://github.com/ellanetworks/ims) or Kamailio). Ella Core acts as the HSS (Cx) and PCRF/PCF (Rx) for the IMS, and carries SIP signalling and call media on a dedicated `ims` data network.
- **SMS**: Subscribers can send and receive SMS over 4G and 5G using an external SMSC (ex. [Ella SMSC](https://github.com/ellanetworks/smsc)). Ella Core carries messages in NAS signalling and connects to SMSCs over Diameter (SGd and S6c). Configure SMSC peers and Ella Core's SMS number from the Operator page.
- **Location (Beta)**:
  - Locate 4G subscribers with A-GNSS and UE-assisted E-CID over LPP.
  - Choose the E-CID measurement quantities in location requests.
  - Request periodic location reports for a subscriber.

## Maintenance

- Snap auto-connect: `snap install ella-core` now connects the required interfaces automatically, with no `snap connect` commands needed.
- OCUDU snap: Install OCUDU with `snap install ocudu`. See the [co-hosting guide](https://docs.ellanetworks.com/core/how_to/co_host_with_ocudu/index.md).
- Distributed tracing across HA nodes
- Bug fixes
- Dependency updates
