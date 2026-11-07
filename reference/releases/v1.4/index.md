# v1.4

- **Release date**: March 3, 2026
- **GitHub release**: [v1.4.0](https://github.com/ellanetworks/core/releases/tag/v1.4.0)

## New features

- **Traffic Flows**: Gain visibility into how each subscriber reaches external networks. Powered by eBPF/XDP, Ella Core captures network flow metadata (IMSI, source and destination IP addresses and port, protocol, and traffic direction). This enables observability, security monitoring, troubleshooting, and compliance use cases. The feature is on by default and can be disabled by administrators.
- **Global snackbar**: Introduce a global snackbar/toast notification system across the UI for consistent user feedback.
- **Trace\<->log correlation**: Correlate logs with distributed traces using trace IDs.

## Documentation

- **Improved observability reference**: Add example Grafana [alert rules](https://docs.ellanetworks.com/core/reference/observability/#alert-rules) and recommend Tempo and Mimir for Tracing and Metrics.

## Maintenance

- Bug fixes
- Dependency updates
- Performance improvements
