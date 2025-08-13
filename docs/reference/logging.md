---
description: Reference for managing logs in Ella Core.
---

# Logging

Ella Core produces two types of logs: **system logs** and **audit logs**.

Ella Core does not assist with log rotation; we recommend using a log rotation tool to manage log files.

## System Logs

Ella Core logs many events, including errors, warnings, and information messages. The logs help monitor the health of the system and diagnose issues. Users can configure the log level and output (`stdout` or `file`) for system logs.

## Audit Logs

Ella Core automatically logs all user actions, including login attempts, API calls, and changes to the system configuration. Users can configure the output (`stdout` or `file`) for audit logs.

In addition to the output defined via the configuration file, audit Logs are accessible via the [API](api/logs.md) and the Web UI.

### Example

In the following example, we see the `admin@allanetworks.com` user creating a policy named `new-policy` with the associated timestamp.

```
2025-03-01T09:47:59.410-0500    INFO    logger/logger.go:214    audit event     {"component": "Audit", "action": "create_policy", "actor": "admin@ellanetworks.com", "details": "User created policy: new-policy", "ip": "127.0.0.1"}
```

## Configuration

For more information on configuring logging in Ella Core, refer to the [Configuration File](config_file.md) documentation.
