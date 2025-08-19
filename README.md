# Ella Core

<p align="center">
  <img src="docs/images/summary.svg" alt="Ella Core Logo"/>
</p>

[![ella-core](https://snapcraft.io/ella-core/badge.svg)](https://snapcraft.io/ella-core)

> :construction: **Beta Notice**
> Ella Core is currently in beta. If you encounter any issues, please [report them here](https://github.com/ellanetworks/core/issues/new/choose).

Ella Core is a 5G mobile core network designed for private deployments. It consolidates the complexity of traditional 5G networks into a single application, offering simplicity, reliability, and security.

Typical mobile networks are expensive, complex, and inadequate for private deployments. They require a team of experts to deploy, maintain, and operate. Open source alternatives are often incomplete, difficult to use, and geared towards research and development. Ella Core is an open-source, production-geared solution that simplifies the deployment and operation of private mobile networks.

Use Ella Core where you need 5G connectivity: in a factory, a warehouse, a farm, a stadium, a ship, a military base, or a remote location.

[Get Started Now!](https://docs.ellanetworks.com/tutorials/getting_started/)

## Key features

- **5G compliant**: Deploy Ella Core with 5G radios and devices. Ella Core's interfaces follow 3GPP standards.
- **Performant Data Plane**: Achieve high throughput and low latency with an eBPF-based data plane. Ella Core delivers over 3 Gbps of throughput and less than 2 ms of latency.
- **Lightweight**: Ella Core is a single binary with an embedded database, making it easy and quick to stand up. It requires as little as 2 CPU cores, 2GB of RAM, and 10GB of disk space. Forget specialized hardware; all you need to operate your 5G core network is a Linux system with four network interfaces.
- **Intuitive User Experience**: Manage subscribers, radios, data networks, policies, and operator information through a user-friendly web interface. Automate network operations with a complete REST API.
- **Real-Time Observability**: Access detailed metrics, traces, and dashboards to monitor network health through the UI, the Prometheus-compliant API, or an OpenTelemetry collector.
- **Backup and Restore**: Backup and restore the network configuration and data.
- **Audit Logs**: Keep track of all operations performed on the network.

## Quick Links

- [Documentation](https://docs.ellanetworks.com/)
- [Whitepaper](https://medium.com/@gruyaume/ella-core-simplifying-private-mobile-networks-a82de955c92c)
- [Snap Store Listing](https://snapcraft.io/ella-core)
- [Contributing](CONTRIBUTING.md)

## Tenets

Building Ella Core, we make engineering decisions based on the following tenets:

1. **Simplicity**: We are committed to developing the simplest possible mobile core network user experience. We thrive on having a very short Getting Started tutorial, a simple configuration file, a single binary, an embedded database, and a simple UI.
2. **Reliability**: We are commited to developing a reliable mobile network you can trust to work 24/7. We are committed to delivering high-quality code, tests, and documentation. We are committed to exposing dashboards, metrics, and logs to help users monitor their networks.
3. **Security**: We are committed to minimizing the private network's attack surface, using secure encryption protocols to protect our users' data, to provide audit mechanisms, to identify and fix vulnerabilities, and to provide a secure-by-default configuration.

## Acknowledgements

Ella Core could not have been built without the following open-source projects:
- [Aether](https://aetherproject.org/)
- [eUPF](https://github.com/edgecomllc/eupf)
- [free5GC](https://free5gc.org/)
