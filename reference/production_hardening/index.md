# Production Hardening

This reference document provides guidelines for operating Ella Core in a production environment.

## Recommendations

- **Deploy on a production-grade system**. Ensure your system meets the [production requirements](https://docs.ellanetworks.com/reference/system_reqs/index.md).
- **Deploy with the snap**: Use the [Snap installation method](https://docs.ellanetworks.com/how_to/install/#__tabbed_1_1) to deploy Ella Core.
- **Isolate network interfaces**: Use separate network interfaces for N2, N3, N6, and API traffic.
- **Use TLS**: Configure TLS for the API interface in the configuration file. Use certificates from a trusted Certificate Authority (CA).
- **Use the fastest attach mode the interfaces allow**: set `datapath.attach-mode` to `xdp-native` where the driver supports it.
- **Set logging level to info**: Configure system logging level to `info`.
- **Disable telemetry**: Disable telemetry in the configuration file.
- **Back up the database**: Back up the database file on a **daily** basis. Retain backups for at least **7 days**.
- **Monitor metrics and logs**: Operate an external Observability stack to collect and visualize metrics and logs exposed by Ella Core.
