# Platform Security

Info

To report a security vulnerability, please file a [Private Security Report](https://github.com/ellanetworks/core/security).

Security is one of Ella Core's core tenets. From authentication and authorization to transport encryption and audit logging, security is built into every layer of the system.

## Authentication & Authorization

Ella Core enforces authentication on API requests towards most endpoints. Two authentication methods are supported:

- **Session-based authentication.** Users authenticate with email and password. A session cookie and a short-lived access token are issued. The login endpoint enforces per-IP rate limiting to protect against brute-force attacks.
- **API tokens.** Per-user tokens with explicit expiry that can be revoked individually. Recommended for programmatic access.

### Role-Based Access Control

Every request is authorized against a role-based permission system with three built-in roles:

| Role                | Scope                                                                                          |
| ------------------- | ---------------------------------------------------------------------------------------------- |
| **Admin**           | Full access to all resources and operations.                                                   |
| **Network Manager** | Manages network resources (subscribers, policies, data networks, routes). Cannot manage users. |
| **Read Only**       | Read-only access to network resources.                                                         |

## Secret Storage

- **User passwords** are stored as one-way hashes. Verification uses constant-time comparison to prevent timing attacks.
- **API token secrets** are stored as one-way hashes. The raw token is returned only once at creation time and is never retrievable afterward.
- **Session tokens** are cryptographically random values. Only a one-way hash is persisted.
- **JWT signing secret** is generated randomly at startup and held only in memory. It is never written to disk or exposed through the API. A service restart invalidates all previously issued tokens.
- **Cluster PKI signing keys** (root and intermediate) are stored in the replicated database. Each node caches its own leaf key (0600) and leaf certificate and trust bundle (0644) on disk under the data directory. Treat the data directory and backup archives as secret-bearing.

## Transport Security

Ella Core uses TLS to secure its API and web interface.

The TLS configuration is defined in the [configuration file](https://docs.ellanetworks.com/reference/config_file/index.md). The snap installation generates a self-signed certificate (valid for 365 days) by default. Users can replace the certificate and key files at any time; a service restart applies the change.

For production deployments, replace the self-signed certificate with one issued by a trusted Certificate Authority (CA) and restrict access to the private key.

Ella Core supports TLS `1.2` and `1.3`.

## Minimal Attack Surface

Ella Core minimizes its attack surface through minimal packaging:

- **Container image.** Built on a distroless base with no operating system layer, shell, or package manager. Only the strictly necessary runtime dependencies are included. Image size: **under 80 MB**.
- **Snap.** Ships only the application binary and a minimal configuration file. Package size: **under 20 MB**.

## Audit Logging

Ella Core logs security-relevant events as audit records that can be accessed via the UI and the API. These logs provide a comprehensive record of who did what and when on your network, helping you monitor activity, investigate incidents, and meet compliance requirements.

Each audit record contains:

| Field         | Description                                              |
| ------------- | -------------------------------------------------------- |
| **Timestamp** | RFC 3339 UTC timestamp.                                  |
| **Actor**     | Email of the user who performed the action.              |
| **Action**    | Machine-readable action identifier (e.g., `auth_login`). |
| **IP**        | Client IP address.                                       |
| **Details**   | Human-readable description.                              |

### Retention

Audit logs are retained for **7 days** by default. The retention period is configurable through the [Audit Logs API](https://docs.ellanetworks.com/reference/api/audit_logs/index.md). A background worker runs every 24 hours and deletes records older than the configured retention period.
