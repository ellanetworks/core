---
description: RESTful API reference for managing data networks.
---

# Data Networks

## List Data Networks

This path returns the list of data networks.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/networking/data-networks` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "name": "internet",
            "ip-pool": "172.250.0.0/24",
            "dns": "8.8.8.8",
            "mtu": 1460,
            "status": {
                "sessions": 0
            }
        }
    ]
}
```

## Create a Data Network

This path creates a new Data Network.

| Method | Path                    |
| ------ | ----------------------- |
| POST   | `/api/v1/networking/data-networks` |

### Parameters

- `name` (string): The Name of the Data Network (dnn)
- `ip-pool` (string): The IP pool of the data network in CIDR notation. Example: `172.250.0.0/24`.
- `dns` (string): The IP address of the DNS server of the data network. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the data network. Must be an integer between 0 and 65535.

### Sample Response

```json
{
    "result": {
        "message": "Data Network created successfully"
    }
}
```

## Update a Data Network

This path updates an existing data network.

| Method | Path                           |
| ------ | ------------------------------ |
| PUT    | `/api/v1/networking/data-networks/{name}` |

### Parameters

- `ip-pool` (string): The IP pool of the data network in CIDR notation. Example: `172.250.0.0/24`.
- `dns` (string): The IP address of the DNS server of the data network. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the data network. Must be an integer between 0 and 65535.

### Sample Response

```json
{
    "result": {
        "message": "Data Network updated successfully"
    }
}
```

## Get a Data Network

This path returns the details of a specific data network.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/v1/networking/data-networks/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "name": "internet",
        "ip-pool": "0.0.0.0/24",
        "dns": "8.8.8.8",
        "mtu": 1460,
        "status": {
            "sessions": 0
        }
    }
}
```

## Delete a Data Network

This path deletes a data network from Ella Core.

| Method | Path                           |
| ------ | ------------------------------ |
| DELETE | `/api/v1/networking/data-networks/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Data Network deleted successfully"
    }
}
```


# Routes

## List Routes

This path returns the list of routes.


| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/networking/routes` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "id": 1,
            "destination": "0.0.0.0/0",
            "gateway": "203.0.113.1",
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
| POST   | `/api/v1/networking/routes` |

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
| GET    | `/api/v1/networking/routes/{id}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "id": 4,
        "destination": "0.0.0.0/0",
        "gateway": "203.0.113.1",
        "interface": "n6",
        "metric": 0
    }
}
```

## Delete a Route

This path deletes a route from Ella Core.

| Method | Path                  |
| ------ | --------------------- |
| DELETE | `/api/v1/networking/routes/{id}` |

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

# NAT

## Get NAT Info

This path returns the current NAT configuration.

| Method | Path         |
| ------ | ------------ |
| GET    | `/api/v1/networking/nat` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "enabled": true,
    }
}
```

## Update NAT Info

This path updates the NAT configuration.

| Method | Path         |
| ------ | ------------ |
| PUT    | `/api/v1/networking/nat` |

### Parameters

- `enabled` (boolean): Enable or disable NAT.

### Sample Response

```json
{
    "result": {
        "message": "NAT configuration updated successfully"
    }
}
```
