---
description: RESTful API reference for managing routes.
---

# Routes

## List Routes

This path returns the list of routes.


| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/routes` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "id": 1,
            "destination": "0.0.0.0/0",
            "gateway": "66.66.66.6",
            "interface": "n6",
            "metric": 0
        }
    ]
}
```

## Create a Route

This path creates a new route.

| Method | Path             |
| ------ | ---------------- |
| POST   | `/api/v1/routes` |

### Parameters

- `destination` (string): The destination IP address of the route in CIDR notation. Example: `0.0.0.0/0`.
- `gateway` (string): The IP address of the gateway of the route. Example: `1.2.3.4`.
- `interface` (string): The outgoing interface of the route. Allowed values: `n3`, `n6`.
- `metric` (int): The metric of the route. Must be an integer between 0 and 255.

### Sample Response

```json
{
    "result": {
        "message": "Route created successfully",
        "id": 4
    }
}
```

## Get a Route

This path returns the details of a specific route.

| Method | Path                  |
| ------ | --------------------- |
| GET    | `/api/v1/routes/{id}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "id": 4,
        "destination": "0.0.0.0/0",
        "gateway": "66.66.66.6",
        "interface": "n6",
        "metric": 0
    }
}
```

## Delete a Route

This path deletes a route from Ella Core.

| Method | Path                  |
| ------ | --------------------- |
| DELETE | `/api/v1/routes/{id}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Route deleted successfully"
    }
}
```
