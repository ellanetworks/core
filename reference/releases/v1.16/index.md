# v1.16

- **Release date**: August 24, 2026
- **GitHub release**: [v1.16.0](https://github.com/ellanetworks/core/releases/tag/v1.16.0)

## New features

- **eBPF TCX user plane**: Attach Ella Core's user plane program to the TCX hook. TCX allows Ella Core to work on every modern Linux system, regardless of driver support. It replaces XDP Generic as a fast and reliable fallback option on systems without XDP Native support. XDP Native remains the preferred option when possible. XDP Generic should only be used for test and dev deployments.
- **4G \<-> 5G Interworking**: Subscribers can smoothly transition from 4G to 5G and vice-versa via N26 (and non-n26) handover, maintaining their IP session.
- **Local switching**: Allow direct UE-to-UE (hairpin) communication. This feature is disabled by default and can be enabled via the UI in the Networking page or the API.
- **IP Fragments**: Support IPv4 and IPv6 fragments in subscriber data path.
- **NITZ**: Send network time to UEs.

## Maintenance

- Use in-house S1AP, NGAP, NAS, and PER libraries.
- Add Baicells Neutrino 220 to supported 4G radios
- Add Playwright UI end-to-end tests
- Extend handover integration testing
- Performance improvements
- Security fixes
- Bug fixes
- Dependency updates
