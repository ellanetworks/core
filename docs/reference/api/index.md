
# API

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

## Status codes

- 200 - Success.
- 201 - Created.
- 400 - Bad request.
- 401 - Unauthorized.
- 500 - Internal server error.

## Endpoints

| Endpoint          | HTTP Method | Description                   |
| ----------------- | ----------- | ----------------------------- |
| `/api/v1/metrics` | GET         | Get metrics (Unauthenticated) |
| `/api/v1/network` | PUT         | Update network configuration  |
| `/api/v1/network` | GET         | Get network configuration     |
