---
description: RESTful API reference for managing Ella Core.
---

# API

Ella Core exposes a RESTful API for managing subscribers, radios, data networks, policies, users, routes, and operator configuration.

## Authentication

Almost every operation requires a client token. The client token must be sent as Authorization HTTP Header using the Bearer <token> scheme. That token can either be the JWT returned by the [login](auth.md/#login) endpoint or an [API token](users.md/#create-an-api-token).

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
- 429 - Too many requests.
- 500 - Internal server error.

## Client

Ella Core provides a [Go client](https://pkg.go.dev/github.com/ellanetworks/core/client) for interacting with the API.

```go
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
		PolicyName:     "default",
	}

	err = ella.CreateSubscriber(createSubscriberOpts)
	if err != nil {
		log.Println("Failed to create subscriber:", err)
	}
}
```
