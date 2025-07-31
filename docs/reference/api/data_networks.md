---
description: RESTful API reference for managing data networks.
---

# Data Networks

## List Data Networks

This path returns the list of data networks.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/data-networks` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "name": "default-default",
            "ip-pool": "172.250.0.0/24",
            "dns": "8.8.8.8",
            "mtu": 1460
        }
    ]
}
```

## Create a Data Network

This path creates a new Data Network.

| Method | Path                    |
| ------ | ----------------------- |
| POST   | `/api/v1/data-networks` |

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
| PUT    | `/api/v1/data-networks/{name}` |

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
| GET    | `/api/v1/data-networks/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "name": "internet",
        "ue-ip-pool": "0.0.0.0/24",
        "dns": "8.8.8.8",
        "mtu": 1460
    }
}
```

## Delete a Data Network

This path deletes a data network from Ella Core.

| Method | Path                           |
| ------ | ------------------------------ |
| DELETE | `/api/v1/data-networks/{name}` |

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
