# API

Ella Core exposes a RESTful API for managing subscribers, radios, data networks, profiles, slices, policies, users, routes, and operator configuration.

## Authentication

Almost every operation requires a client token. The client token must be sent as Authorization HTTP Header using the Bearer scheme. That token can either be the JWT returned by the [login](https://docs.ellanetworks.com/reference/api/auth/#login) endpoint or an [API token](https://docs.ellanetworks.com/reference/api/users/#create-an-api-token).

## Responses

Ella Core's API responses are JSON objects with the following structure:

```
{
  "result": "Result content",
  "error": "Error message",
}
```

Info

GET calls to the `/metrics` endpoint don't follow this rule, it returns text response in the [Prometheus exposition format](https://prometheus.io/docs/instrumenting/exposition_formats/#text-format-details).

## OpenAPI Specification

The full API is described by an [OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.0) specification embedded in the binary. Fetch it from any running Ella Core instance:

```
GET /api/v1/openapi.yaml
```

This endpoint is unauthenticated. The spec can be used to generate client libraries, import into tools like Postman or Swagger UI, or integrate with AI agents and automation frameworks that consume OpenAPI definitions.

## Status codes

- 200 - Success.
- 201 - Created.
- 400 - Bad request.
- 401 - Unauthorized.
- 429 - Too many requests.
- 500 - Internal server error.

## Client

Ella Core provides a [Go client](https://pkg.go.dev/github.com/ellanetworks/core/client) for interacting with the API.

```
package main

import (
    "log"

    "github.com/ellanetworks/core/client"
)

func main() {
    clientConfig := &client.Config{
        BaseURL:  "http://127.0.0.1:5002",
        APIToken: "ellacore_Xl2yU1rcy2BP_8q5iOpNBtoXLYdwddbBCHInx",
    }

    ella, err := client.New(clientConfig)
    if err != nil {
        log.Println("Failed to create client:", err)
    }

    createSubscriberOpts := &client.CreateSubscriberOptions{
        Imsi:           "001010100000033",
        Key:            "5122250214c33e723a5dd523fc145fc0",
        SequenceNumber: "000000000022",
        ProfileName:    "default",
    }

    err = ella.CreateSubscriber(createSubscriberOpts)
    if err != nil {
        log.Println("Failed to create subscriber:", err)
    }
}
```
