# Slices

Slices represent S-NSSAI (Single Network Slice Selection Assistance Information) configurations. Each slice defines a Slice Service Type (SST) and an optional Slice Differentiator (SD). Ella Core uses slice information alongside the data network name to determine which policies apply to a subscriber's session.

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
- `sst` (integer): The Slice Service Type (SST). Must be an 8-bit integer (0-255).
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

- `sst` (integer): The Slice Service Type (SST). Must be an 8-bit integer (0-255).
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
