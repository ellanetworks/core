---
description: Explanation of platform security - authentication, authorization, transport encryption, and audit logging.
---

# Platform Security

!!! info
    To report a security vulnerability, please file a [Private Security Report](https://github.com/ellanetworks/core/security).

Security is one of Ella Core's core tenets. From the UI to the database, security is built into every layer of the system.

## Authentication & Authorization

Ella Core enforces authentication on API requests towards most endpoints. Two authentication methods are supported:

- **Session-based authentication.** Users authenticate with email and password. A session cookie and a short-lived access token are issued. The login endpoint enforces per-IP rate limiting.
- **API tokens.** Per-user tokens with explicit expiry that can be revoked individually. Recommended for programmatic access.

### Role-Based Access Control

Every request is authorized against a role-based permission system with three built-in roles:

| Role | Scope |
|------|-------|
| **Admin** | Full access to all resources and operations. |
| **Network Manager** | Manages network resources (subscribers, policies, data networks, routes). Cannot manage users. |
| **Read Only** | Read-only access to network resources. |

## Secret Storage

- **User passwords** are stored as one-way hashes. Verification uses constant-time comparison.
- **API token secrets** are stored as one-way hashes. The raw token is returned only once at creation time and is never retrievable afterward.
- **Session tokens** are cryptographically random values. Only a one-way hash is persisted.
- **JWT signing secret** is a cryptographically random value generated once and stored in the database. Rotating it invalidates all previously issued tokens and sessions.

## Transport Security

Ella Core uses TLS to secure its API and web interface.

The TLS configuration is defined in the [configuration file](../reference/config_file.md). The snap installation generates a self-signed certificate (valid for 365 days) by default. Users can replace the certificate and key files at any time.

For production deployments, replace the self-signed certificate with one issued by a trusted Certificate Authority (CA) and restrict access to the private key.

Ella Core supports TLS `1.2` and `1.3`.

In a [high-availability](high_availability.md) cluster, inter-node communication is secured with mutual TLS (TLS `1.3`).

## Minimal Attack Surface

Ella Core minimizes its attack surface through minimal packaging:

- **Container image.** Built on a distroless base with no operating system layer, shell, or package manager. Only the strictly necessary runtime dependencies are included. Image size: **under 100 MB**.
- **Snap.** Ships only the application binary and a minimal configuration file. Package size: **under 20 MB**.

## Audit Logging

Ella Core logs security-relevant events as audit records that can be accessed via the UI and the API. These logs record who did what and when on your network.

Each audit record contains:

| Field | Description |
|-------|-------------|
| **Timestamp** | RFC 3339 UTC timestamp. |
| **Actor** | Email of the user who performed the action. |
| **Action** | Machine-readable action identifier (e.g., `auth_login`). |
| **IP** | Client IP address. |
| **Details** | Human-readable description. |

### Retention

Audit logs are retained for **7 days** by default. The retention period is configurable through the [Audit Logs API](../reference/api/audit_logs.md).
