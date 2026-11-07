# v1.10

- **Release date**: April 30, 2026
- **GitHub release**: [v1.10.0](https://github.com/ellanetworks/core/releases/tag/v1.10.0)

## New features

- **High Availability**: Deploy Ella Core as a highly available cluster. Deploy three or Five nodes. Each node is active and can accept 5G radios and subscriber traffic. Raft is used as the consensus algorithm between nodes. Persistent resources (subscribers, policies, operator info) are replicated across the cluster and runtime state (Radio connections, PDU sessions, user plane state) is tied to individual nodes.

## Maintenance

- Security fixes
- Bug fixes
- Dependency updates

## Documentation

- **[High Availability Explanation](https://docs.ellanetworks.com/core/explanation/high_availability/index.md)**
- **[How-to Deploy a High Availability Cluster](https://docs.ellanetworks.com/core/how_to/deploy_ha_cluster/index.md)**
- **[How-to Scale Up a High Availability Cluster](https://docs.ellanetworks.com/core/how_to/scale_up_ha_cluster/index.md)**
- **[How-to Perform a Rolling Upgrade](https://docs.ellanetworks.com/core/how_to/rolling_upgrade/index.md)**

## Patch releases

### v1.10.2

- **Release date**: May 3, 2026
- **GitHub release**: [v1.10.2](https://github.com/ellanetworks/core/releases/tag/v1.10.2)

#### Maintenance

- Bug fixes

### v1.10.1

- **Release date**: May 2, 2026
- **GitHub release**: [v1.10.1](https://github.com/ellanetworks/core/releases/tag/v1.10.1)

#### Bug fixes

- Make network config per-node in HA ([#1309](https://github.com/ellanetworks/core/pull/1309))
- Live capability probe + atomic FSM apply (rolling-upgrade fixes) ([#1312](https://github.com/ellanetworks/core/pull/1312))
