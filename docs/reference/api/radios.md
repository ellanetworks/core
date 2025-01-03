---
description: RESTful API reference for managing the radio inventory.
---

# Radios

## List Radios

This path returns the list of radios in the inventory.


| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/radios` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "name": "dev2-gnbsim",
            "tac": "001"
        }
    ]
}
```

## Create a Radio

This path creates a new radio in the inventory.

| Method | Path             |
| ------ | ---------------- |
| POST   | `/api/v1/radios` |

### Parameters

- `name` (string): The Name of the radio.
- `tac` (string): The tracking area code (TAC) of the radio.

### Sample Response

```json
{
    "result": {
        "message": "Radio created successfully"
    }
}
```

## Update a Radio

This path updates an existing radio in the inventory.

| Method | Path                    |
| ------ | ----------------------- |
| PUT    | `/api/v1/radios/{name}` |

### Parameters

- `tac` (string): The tracking area code (TAC) of the radio.

### Sample Response

```json
{
    "result": {
        "message": "Radio updated successfully"
    }
}
```

## Get a Radio

This path returns the details of a specific radio in the inventory.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/radios/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "name": "dev2-gnbsim",
        "tac": "001"
    }
}
```

## Delete a Radio

This path deletes a radio from Ella Core.

| Method | Path                    |
| ------ | ----------------------- |
| DELETE | `/api/v1/radios/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Radio deleted successfully"
    }
}
```
