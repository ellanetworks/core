---
description: Reference for viewing user activity logs.
---

# Audit Logs

Ella Core automatically logs all actions performed by users. This includes login attempts, API calls, and changes to the system configuration.

Audit logs are sent to stdout, along with the rest of the logs. They can be differentiated and filtered with the "Audit" tag.

## Example

In the following example, we see the `guillaume` user logging in, listing profiles, and creating a profile named `new-profile` with the associated timestamps.

```
2025-01-01T17:03:31.393-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "login", "actor": "guillaume", "details": "User logged in"}
2025-01-01T17:03:33.254-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "list_profiles", "actor": "guillaume", "details": "User listed profiles"}
2025-01-01T17:03:39.451-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "create_profile", "actor": "guillaume", "details": "User created profile: new-profile"}
```
