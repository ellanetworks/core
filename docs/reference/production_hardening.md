---
description: Reference for operating Ella Core in production.
---

# Production Hardening

This reference document provides guidelines for operating Ella Core in a production environment.

## Recommendations

- **Deploy on a production-grade system**. Ensure your system meets the [production requirements](system_reqs.md).
- **Deploy with the snap**: Use the [Snap installation method](../how_to/install.md#__tabbed_1_1) to deploy Ella Core.
- **Isolate network interfaces**: Use separate network interfaces for N2, N3, N6, and API traffic.
- **Use TLS**: Configure TLS for the API interface in the configuration file. Use certificates from a trusted Certificate Authority (CA).
- **Use XDP in native mode**: Configure Ella Core to use XDP in `native` mode. This requires a compatible network driver.
- **Set logging level to info**: Configure system logging level to `info` and use file output.
- **Disable telemetry**: Disable telemetry in the configuration file.
- **Rotate logs**: Implement log rotation for system and audit logs.
- **Back up the database**: Back up the database file on a **daily** basis. Retain backups for at least **7 days**.
- **Monitor metrics**: Operate an external Observability stack to collect and visualize metrics exposed by Ella Core.
