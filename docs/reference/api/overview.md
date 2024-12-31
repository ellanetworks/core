
# API Overview

Ella Core exposes a RESTful API for managing subscribers, radios, profiles, users, and network configuration.

## Authentication

Almost every operation requires a client token. The client token must be sent as Authorization HTTP Header using the Bearer <token> scheme.

## Responses

Ella Core's API responses are JSON objects with the following structure:

```json
{
  "result": "Result content",
  "error": "Error message",
}
```

!!! info

    GET calls to the `/metrics` endpoint don't follow this rule, it returns text response in the [Prometheus exposition format](https://prometheus.io/docs/instrumenting/exposition_formats/#text-format-details).

## Status codes

- 200 - Success.
- 201 - Created.
- 400 - Bad request.
- 401 - Unauthorized.
- 500 - Internal server error.
