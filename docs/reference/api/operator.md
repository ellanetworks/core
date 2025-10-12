---
description: RESTful API reference for managing the Operator Information - ID, Slice, Tracking, and Code.
---

# Operator

The Operator API provides endpoints to manage the Operator Information used to identify the operator - Operator ID (MCC, MNC), Slice Information (SST, SD), Tracking Information, and Operator Code (OP).

## Get Operator Information

This path returns the complete operator information. This includes the Operator ID, Slice Information, and Tracking Information. The Operator Code is never returned.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/operator` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "id": {
            "mcc": "001",
            "mnc": "01"
        },
        "slice": {
            "sst": 1,
            "sd": ""
        },
        "tracking": {
            "supportedTACs": [
                "001",
                "002",
                "003"
            ]
        },
        "homeNetwork": {
            "publicKey": "021bd3c0ba857e6f45b6ecb76ad826fd27fecef441f23d0e418b645829261e16",
        }
    }
}
```

## Update the Operator ID

This path updates the operator ID. The Mobile Country Code (MCC) and Mobile Network Code (MNC) are used to identify the operator. The operator ID can't be changed when there are subscribers created in the system.

| Method | Path                  |
| ------ | --------------------- |
| PUT    | `/api/v1/operator/id` |

### Parameters

- `mcc` (string): The Mobile Country Code (MCC) of the network. Must be a 3-digit string.
- `mnc` (string): The Mobile Network Code (MNC) of the network. Must be a 2 or 3-digit string.

### Sample Response

```json
{
    "result": {
        "message": "Operator ID updated successfully"
    }
}
```

## Get Operator ID

This path returns the operator ID.

| Method | Path                  |
| ------ | --------------------- |
| GET    | `/api/v1/operator/id` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "mcc": "001",
        "mnc": "01",
    }
}
```

## Update the Operator Slice Information

This path updates the operator slice information. Only one slice is supported. The Slice Service Type (SST) and Service Differentiator (SD) are used to identify the slice.

| Method | Path                     |
| ------ | ------------------------ |
| PUT    | `/api/v1/operator/slice` |

### Parameters

- `sst` (integer): The Slice Service Type (SST) of the network. Must be an 8-bit integer.
- `sd` (optional string): The Service Differentiator (SD) of the network. Must be a 3-byte hexadecimal string without the "0x" prefix. Ex. "010203".

### Sample Response

```json
{
    "result": {
        "message": "Operator slice information updated successfully"
    }
}
```

## Get Operator Slice Information

This path returns the operator Slice Information.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/api/v1/operator/slice` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "sst": 1,
        "sd": ""
    }
}
```

## Update the Operator Tracking Information

This path updates the operator tracking information. The Tracking Area Codes (TACs) are used to identify the tracking areas supported by the operator. 5G radios will need to be configured with one or more of these TACs to connect to the network.

| Method | Path                        |
| ------ | --------------------------- |
| PUT    | `/api/v1/operator/tracking` |

### Parameters

- `supportedTACs` (array): An array of supported TACs (Tracking Area Codes). Each TAC must be a 24-bit integer.

### Sample Response

```json
{
    "result": {
        "message": "Operator tracking information updated successfully"
    }
}
```

## Get Operator Tracking Information

This path returns the operator Tracking Information.

| Method | Path                        |
| ------ | --------------------------- |
| GET    | `/api/v1/operator/tracking` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "supportedTACs": [
            "001",
            "002",
            "003"
        ],
    }
}
```

## Update the Operator Code (OP)

This path updates the Operator Code (OP). The OP is a 32-character hexadecimal string that identifies the operator. This value is secret and should be kept confidential. The OP is used to create the derived Operator Code (OPc). The OP can't be changed when there are subscribers created in the system.

| Method | Path                    |
| ------ | ----------------------- |
| PUT    | `/api/v1/operator/code` |

### Parameters

- `operatorCode` (string): The Operator Code (OP). Must be a 32-character hexadecimal string.

### Sample Response

```json
{
    "result": {
        "message": "Operator Code updated successfully"
    }
}
```

## Update the Home Network Information

This path updates the Home Network Information. The Home Network Private Key ensures IMSI privacy. User Equipment (UE) devices will use the public key to encrypt the IMSI before sending it to the network. The network will then use the private key to decrypt the IMSI.

| Method | Path                            |
| ------ | ------------------------------- |
| PUT    | `/api/v1/operator/home-network` |

### Parameters

- `privateKey` (string): The Home Network Private Key. Must be a 64-character hexadecimal string.

### Sample Response

```json
{
    "result": {
        "message": "Home Network private key updated successfully"
    }
}
```
