# Audit Logs

Ella Core automatically logs all actions performed by users. This includes login attempts, API calls, and changes to the system configuration.

Audit logs are sent to stdout, along with the rest of the logs. They can be differentiated and filtered with the "Audit" tag.

## Example

In the following example, we see the `guillaume` user logging in, listing profiles, and creating a new profile with the associated timestamps.

```
2024-12-31T17:35:05.274-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "login", "actor": "guillaume", "details": "User logged in"}
2024-12-31T17:35:08.025-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "list_profiles", "actor": "guillaume", "details": "User listed profiles"}
2024-12-31T17:35:10.997-0500    INFO    logger/logger.go:118    audit event     {"component": "Audit", "action": "create_profile", "actor": "guillaume", "details": "User created profile"}
```
