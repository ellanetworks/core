# Ella Core

<p align="center">
  <img src="docs/images/logo.png" alt="Ella Core Logo" width="200"/>
</p>

[![ella-core](https://snapcraft.io/ella-core/badge.svg)](https://snapcraft.io/ella-core)

> :construction: **Beta Notice**  
> Ella Core is currently in beta. If you encounter any issues, please [report them here](https://github.com/ellanetworks/core/issues/new/choose).

Ella Core is a 5G mobile core network designed for private deployments. It consolidates the complexity of traditional 5G networks into a single application, offering simplicity, reliability, and security. 

Typical mobile networks are expensive, complex, and inadequate for private deployments. They require a team of experts to deploy, maintain, and operate. Open source alternatives are often incomplete, difficult to use, and geared towards research and development. Ella Core is an open-source, production-geared solution that simplifies the deployment and operation of private mobile networks.

Use Ella Core where you need 5G connectivity: in a factory, a warehouse, a farm, a stadium, a ship, a military base, or a remote location.

[Get Started Now!](https://ellanetworks.github.io/core/tutorials/getting_started/)

## Key features

- **5G compliant**: Deploy Ella Core with 5G radios and devices. Ella Core's interfaces follow 3GPP standards.
- **Intuitive UI**: Manage subscribers, radios, profiles, and operator information through a user-friendly web interface.
- **Extensive HTTP API**: Automate network operations with a complete REST API.
- **Real-Time  Observability**: Access detailed metrics and dashboards to monitor network health through the UI and the Prometheus-compliant API.
- **Backup and restore**: Backup and restore the network configuration and data.

## Tenets

Building Ella Core, we make engineering decisions based on the following tenets:

1. **Simplicity**: We are committed to developing the simplest possible mobile core network user experience. We thrive on having a very short Getting Started tutorial, a simple configuration file, a single binary, an embedded database, and a simple UI.
2. **Reliability**: We are commited to developing a reliable mobile network you can trust to work 24/7. We are committed to delivering high-quality code, tests, and documentation. We are committed to exposing dashboards, metrics, and logs to help users monitor their networks.
3. **Security**: We are committed to minimizing the private network's attack surface and using secure encryption protocols to protect our users' data. We are committed to identifying and fixing security vulnerabilities in a timely manner.

## Documentation

Documentation is available [here](https://ellanetworks.github.io/core/). It includes:
- **Tutorials**: A hands-on introduction.
- **How-to guides**: Step-by-step instructions to perform common tasks.
- **Reference**: Technical information about configuration options, metrics, API, connectivity, and more.
- **Explanation**: Discussion and clarification of key topics

## Acknowledgements

Ella is built on top of the following open source projects:
- [Aether](https://aetherproject.org/)
- [eUPF](https://github.com/edgecomllc/eupf)

## Contributing

We welcome contributions to Ella. Please read our [Contributing Guide](CONTRIBUTING.md) to get started.
