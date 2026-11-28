# v1.21

- **Release date**: November 3, 2026
- **GitHub release**: [v1.21.0](https://github.com/ellanetworks/core/releases/tag/v1.21.0)

## New features

- **Mission Critical Push To Talk (MCPTT)**: Subscribers can make MCPTT calls over 4G and 5G using an external IMS and a 3GPP MCPTT server. See the [MCPTT guide](https://docs.ellanetworks.com/core/how_to/enable_mcptt/index.md).
- **IMS Application Servers**: Configure IMS application servers (ex. MCPTT, Voicemail) from the Operator page and route subscribers to them with IMS filter criteria on their profile.
- **SMS over IP**: Ella Core acts as the HSS for the IMS's IP-SM-GW. While an IP-SM-GW is registered for a subscriber, incoming SMS are delivered over the IMS.
- **Distributed tracing**: Follow a request end to end across Ella Core, [Ella SMSC](https://github.com/ellanetworks/smsc) and [Ella IMS](https://github.com/ellanetworks/ims), ex. an SMS from the routing lookup to delivery to the phone. Trace context is carried over Diameter with every peer, and traces are exported over OTLP to any OpenTelemetry backend (ex. Grafana Tempo, Jaeger). See the [observability reference](https://docs.ellanetworks.com/core/reference/observability/#3-traces).
- **Traffic class metrics**: New `app_upf_session_*` Prometheus metrics count forwarded packets, bytes and drops (per reason) by slice, data network and 5QI.

## Maintenance

- Support bundle now includes Ella Core logs and all configuration data.
- Bug fixes
- Dependency updates
