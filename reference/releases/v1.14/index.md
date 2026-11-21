# v1.14

- **Release date**: July 21, 2026
- **GitHub release**: [v1.14.0](https://github.com/ellanetworks/core/releases/tag/v1.14.0)

## New features

- **Framed Routing**: Allow an IP network to sit behind a UE so that a range of IP addresses is reachable over a single PDU session. Administrators can manage framed routes via the data networks page in the UI or the API.
- **Static IPs**: Assign specific IP addresses to your mobile subscribers. Administrators can manage static IPs via the data networks page in the UI or the API. Static IPs are optional, dynamic allocation remains the default IP allocation mechanism.
- **Location on 4G (beta)**: The location beta (Cell ID and E-CID) now supports 4G subscribers in addition to 5G.
- **Location with A-GNSS (beta)**: Extends the location beta with Assisted GNSS (A-GNSS) positioning via the LTE Positioning Protocol (LPP), building on the Cell ID and E-CID support introduced in v1.13. This feature is only available for 5G subscribers at the moment and requires device support.

## Maintenance

- Security fixes
- Bug fixes
- Dependency updates
