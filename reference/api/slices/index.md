# Slices

Each slice defines a Slice Service Type (SST) and an optional Slice Differentiator (SD). Ella Core uses slice information alongside the data network name to determine which policies apply to a subscriber's session.

## List Slices

This path returns the list of slices.

| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/slices` |

### Query Parameters

| Name       | In    | Type | Default | Allowed | Description               |
| ---------- | ----- | ---- | ------- | ------- | ------------------------- |
| `page`     | query | int  | `1`     | `>= 1`  | 1-based page index.       |
| `per_page` | query | int  | `25`    | `1…100` | Number of items per page. |

### Sample Response

```
{
    "result": {
        "items": [
            {
                "name": "default",
                "sst": 1,
                "sd": "010203"
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Create a Slice

This path creates a new slice.

| Method | Path             |
| ------ | ---------------- |
| POST   | `/api/v1/slices` |

### Parameters

- `name` (string): The name of the slice.
- `sst` (integer): The Slice Service Type (SST). Must be an integer between 1 and 255.
- `sd` (optional string): The Service Differentiator (SD). Must be a 3-byte hexadecimal string without the "0x" prefix. Ex. "010203".

### Sample Response

```
{
    "result": {
        "message": "Slice created successfully"
    }
}
```

## Get a Slice

This path returns the details of a specific slice.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/slices/{name}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "name": "default",
        "sst": 1,
        "sd": "010203"
    }
}
```

## Update a Slice

This path updates an existing slice.

| Method | Path                    |
| ------ | ----------------------- |
| PUT    | `/api/v1/slices/{name}` |

### Parameters

- `sst` (integer): The Slice Service Type (SST). Must be an integer between 1 and 255.
- `sd` (optional string): The Service Differentiator (SD). Must be a 3-byte hexadecimal string without the "0x" prefix. Ex. "010203".

### Sample Response

```
{
    "result": {
        "message": "Slice updated successfully"
    }
}
```

## Delete a Slice

This path deletes a slice from Ella Core. A slice cannot be deleted if it is referenced by any policy.

| Method | Path                    |
| ------ | ----------------------- |
| DELETE | `/api/v1/slices/{name}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "message": "Slice deleted successfully"
    }
}
```

## List the Data Networks of a Slice

This path returns the data networks a slice serves, with the forwarding of each. A policy can only use a data network on a slice that serves it.

| Method | Path                                  |
| ------ | ------------------------------------- |
| GET    | `/api/v1/slices/{name}/data-networks` |

### Sample Response

```
{
    "result": {
        "items": [
            {
                "data_network": "internet",
                "nat": true,
                "local_switch": false,
                "network_instance": "default"
            }
        ]
    }
}
```

- `nat` (boolean): Whether Ella Core translates the IPv4 addresses of the subscribers of this data network on this slice to the N6 address of the node that serves the session.
- `local_switch` (boolean): Whether traffic between two subscribers of this data network on this slice is switched inside the user plane.
- `network_instance` (string): The N6 network instance of the sessions: `default`, or a name from `interfaces.network-instances`.

## Serve a Data Network on a Slice

| Method | Path                                  |
| ------ | ------------------------------------- |
| POST   | `/api/v1/slices/{name}/data-networks` |

### Parameters

- `data_network` (string): The name of the data network.
- `nat` (boolean, optional): Defaults to `true`, or `false` when the data network has framed routes. Always `false` on an IMS data network, and cannot be `true` while the data network has framed routes.
- `local_switch` (boolean, optional): Defaults to `false`. Always `true` on an IMS data network.
- `network_instance` (string, optional): Defaults to `default`.

### Sample Response

```
{
    "result": {
        "message": "The slice now serves the data network"
    }
}
```

## Get a Data Network of a Slice

| Method | Path                                                 |
| ------ | ---------------------------------------------------- |
| GET    | `/api/v1/slices/{name}/data-networks/{data_network}` |

### Sample Response

```
{
    "result": {
        "data_network": "internet",
        "nat": true,
        "local_switch": false,
        "network_instance": "default"
    }
}
```

## Update a Data Network of a Slice

Changes apply to established sessions without releasing them, except on a node that does not define the new network instance, which releases them. Changing `nat` breaks the established connections of the sessions.

| Method | Path                                                 |
| ------ | ---------------------------------------------------- |
| PUT    | `/api/v1/slices/{name}/data-networks/{data_network}` |

### Parameters

- `nat` (boolean, optional): Cannot be enabled on an IMS data network, or while the data network has framed routes. Omitted leaves it unchanged.
- `local_switch` (boolean, optional): Cannot be disabled on an IMS data network. Omitted leaves it unchanged.
- `network_instance` (string, optional): Empty means `default`. Omitted leaves it unchanged.

### Sample Response

```
{
    "result": {
        "message": "Slice data network updated successfully"
    }
}
```

## Stop Serving a Data Network on a Slice

Refused while a policy uses the data network on this slice.

| Method | Path                                                 |
| ------ | ---------------------------------------------------- |
| DELETE | `/api/v1/slices/{name}/data-networks/{data_network}` |

### Sample Response

```
{
    "result": {
        "message": "The slice no longer serves the data network"
    }
}
```
