---
description: RESTful API reference for managing data networks.
---

# Data Networks

## List Data Networks

This path returns the list of data networks.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/networking/data-networks` |

### Query Parameters

| Name       | In    | Type | Default | Allowed | Description                   |
| ---------- | ----- | ---- | ------- | ------- | ----------------------------- |
| `page`     | query | int  | `1`     | `>= 1`  | 1-based page index.           |
| `per_page` | query | int  | `25`    | `1…100` | Number of items per page.     |

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "name": "internet",
                "ip_pool": "172.250.0.0/24",
                "dns": "8.8.8.8",
                "mtu": 1460,
                "status": {
                    "sessions": 0
                }
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Create a Data Network

This path creates a new Data Network.

| Method | Path                    |
| ------ | ----------------------- |
| POST   | `/api/v1/networking/data-networks` |

### Parameters

- `name` (string): The Name of the Data Network (dnn)
- `ip_pool` (string): The IP pool of the data network in CIDR notation. Example: `172.250.0.0/24`.
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

- `ip_pool` (string): The IP pool of the data network in CIDR notation. Example: `172.250.0.0/24`.
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
        "ip_pool": "0.0.0.0/24",
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

# Interfaces

## Get Network Interfaces Config

This path returns the network interfaces.

| Method | Path                      |
| ------ | ------------------------- |
| GET    | `/api/v1/networking/interfaces` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "n2": {
            "address": "192.168.40.6",
            "port": 38412
        },
        "n3": {
            "name": "wlp131s0",
            "address": "192.168.40.6",
            "external_address": ""
        },
        "n6": {
            "name": "lo"
        },
        "api": {
            "address": "",
            "port": 5002
        }
    }
}
```

## Update N3 Interface Settings

This path updates the N3 interface settings.

| Method | Path                      |
| ------ | ------------------------- |
| PUT    | `/api/v1/networking/interfaces/n3` |

### Parameters

- `external_address` (string): The external address to be used for the N3 interface. This address will be advertised to the gNodeB in the GTPTunnel Transport Layer Address field part of the PDU Session Setup Request message. The address will be used by the gNodeB to set the GTP tunnel. This setting is useful when Ella Core is behind a proxy or NAT and the N3 interface address is not reachable by the gNodeB. If not set, Ella Core will use the address of the N3 interface as defined in the config file.

### Sample Response

```json
{
    "result": {
        "message": "N3 interface updated"
    }
}
```

# Routes

## List Routes

This path returns the list of routes.


| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/networking/routes` |

### Query Parameters

| Name       | In    | Type | Default | Allowed | Description                   |
| ---------- | ----- | ---- | ------- | ------- | ----------------------------- |
| `page`     | query | int  | `1`     | `>= 1`  | 1-based page index.           |
| `per_page` | query | int  | `25`    | `1…100` | Number of items per page.     |

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": 1,
                "destination": "0.0.0.0/0",
                "gateway": "203.0.113.1",
                "interface": "n6",
                "metric": 0
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
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

# Flow Accounting

## Get Flow Accounting Info

This path returns the current flow accounting configuration.

| Method | Path                                 |
| ------ | ------------------------------------ |
| GET    | `/api/v1/networking/flow-accounting` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "enabled": true
    }
}
```

## Update Flow Accounting Info

This path updates the flow accounting configuration.

| Method | Path                                 |
| ------ | ------------------------------------ |
| PUT    | `/api/v1/networking/flow-accounting` |

### Parameters

- `enabled` (boolean): Enable or disable flow accounting.

### Sample Response

```json
{
    "result": {
        "message": "Flow accounting settings updated successfully"
    }
}
```
