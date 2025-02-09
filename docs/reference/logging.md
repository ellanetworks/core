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

### Example

In the following example, we see the `guillaume` user logging in, listing profiles, and creating a profile named `new-profile` with the associated timestamps.

```
2025-01-01T17:03:31.393-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "login", "actor": "guillaume", "details": "User logged in"}
2025-01-01T17:03:33.254-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "list_profiles", "actor": "guillaume", "details": "User listed profiles"}
2025-01-01T17:03:39.451-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "create_profile", "actor": "guillaume", "details": "User created profile: new-profile"}
```

## Configuration

For more information on configuring logging in Ella Core, refer to the [Configuration File](config_file.md) documentation.
