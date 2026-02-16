# Ella Core

<p align="center">
  <img src="docs/images/summary.svg" alt="Ella Core Logo"/>
</p>

[![ella-core](https://snapcraft.io/ella-core/badge.svg)](https://snapcraft.io/ella-core)

Ella Core is a 5G core designed for private networks. It consolidates the complexity of traditional 5G networks into a single application, offering simplicity, reliability, and security.

Typical mobile networks are expensive, complex, and inadequate for private deployments. They require a team of experts to deploy, maintain, and operate. Open source alternatives are often incomplete, difficult to use, and geared towards research and development. Ella Core is an open-source, production-geared solution that simplifies the deployment and operation of private mobile networks.

Use Ella Core where you need 5G connectivity: in a factory, a warehouse, a farm, a stadium, a ship, a military base, or a remote location.

[Get Started Now!](https://docs.ellanetworks.com/tutorials/getting_started/)

## Key features

- **5G Compliant**: Ella Core implements 3GPP-standard interfaces and has been validated with multiple 5G radios, including integrated and software-defined RANs, commercial phones and devices. It is 5G RedCap compliant for IoT deployments.
- **Performant Data Plane**: Achieve high throughput and low latency with an eBPF-based data plane. Ella Core delivers over 10 Gbps of throughput and less than 1 ms of latency.
- **Lightweight**: Ella Core is a single binary with an embedded database, making it easy and quick to stand up. It requires as little as 1 CPU core, 1GB of RAM, and 10GB of disk space. Forget specialized hardware; all you need to operate your 5G core network is a Linux system with a network interface.
- **Intuitive User Experience**: Manage subscribers, radios, data networks, policies, and operator information through a user-friendly web interface. Automate network operations with a complete REST API.
- **Real-Time Observability**: Access logs, metrics, traces, profiles, and dashboards to monitor network health through the UI, the Prometheus-compliant API, or an OpenTelemetry collector.
- **Backup and Restore**: Backup and restore your data in 1 click.
- **Audit Logs**: At any moment, keep track of who did what and when on your network.
- **Open Source**: Ella Core is open source and available under the Apache 2.0 license.

## Quick Links

- [Website](https://ellanetworks.com/)
- [Documentation](https://docs.ellanetworks.com/)
- [YouTube Channel](https://www.youtube.com/@ellanetworks)
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
