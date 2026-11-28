# v1.13

- **Release date**: July 10, 2026
- **GitHub release**: [v1.13.0](https://github.com/ellanetworks/core/releases/tag/v1.13.0)

## New features

- **4G support**: Run a 4G and 5G private network with Ella Core, still a single, lightweight binary. 5G radios still connect to Ella Core's n2 interface on port `38412` and 4G radios can now connect on port `36412`. 4G has been validated with multiple devices (iPhone, Samsung, and more), the list will grow over the next few months. 4G and 5G are on by default. Admins can manage 4G and 5G access via subscriber profiles. Many items in the UI have been made neutral to accommodate both 4G and 5G versions.
- **Location support (beta)**: Beta APIs for tracking subscriber location via Cell ID or Enhanced Cell ID (E-CID). This feature is not yet available via the UI.

## Maintenance

- Security fixes
- Bug fixes
- Dependency updates
