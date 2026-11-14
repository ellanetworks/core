# v1.17

- **Release date**: September 3, 2026
- **GitHub release**: [v1.17.0](https://github.com/ellanetworks/core/releases/tag/v1.17.0)

## New features

- **Subscriber discoverability**: Write a short description to easily differentiate subscribers (ex. "campsite 1", "camera1-venue2"). Search subscribers by IMSI or description.
- **Subscriber Connection state**: View the subscriber's connection state ("Connected" or "Idle"). View the last radio the subscriber was connected to, even when the subscriber goes Idle.
- **Radio Status**: View the radio status ("Online" or "Offline"). Radios now remain visible to users when they go offline. Users can manually "Forget" offline radios.
- **Decode all radio events**: Every S1AP, NGAP and NAS message Ella Core sends or receives is now visible in the network event decoder.

## Maintenance

- Add Google Pixel 10a to supported devices
- Trim container image and snap size by >30%
- Harden 4G \<-> 5G interworking
- Extend High Availability (HA) testing suite and increase resiliency during edge-case scenarios
- Performance improvements
- Security fixes
- Bug fixes
- Dependency updates
