# Operator

The Operator API provides endpoints to manage the Operator Information used to identify the operator - Operator ID (MCC, MNC), Tracking Information, Operator Code (OP), NAS Security Algorithms, and the Service Provider Name (SPN).

## Get Operator Information

This path returns the complete operator information. This includes the Operator ID and Tracking Information. The Operator Code is never returned.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/operator` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "id": {
            "mcc": "001",
            "mnc": "01"
        },
        "tracking": {
            "supportedTACs": [
                "001",
                "002",
                "003"
            ]
        },
        "homeNetworkKeys": [
                {
                    "id": 1,
                    "keyIdentifier": 0,
                    "scheme": "A",
                    "publicKey": "021bd3c0ba857e6f45b6ecb76ad826fd27fecef441f23d0e418b645829261e16"
                }
            ],
        "nasSecurity": {
            "ciphering": ["NEA2", "NEA1", "NEA0"],
            "integrity": ["NIA2", "NIA1", "NIA0"]
        },
        "spn": {
            "fullName": "Ella Networks",
            "shortName": "Ella"
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

```
{
    "result": {
        "message": "Operator ID updated successfully"
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

```
{
    "result": {
        "message": "Operator tracking information updated successfully"
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

```
{
    "result": {
        "message": "Operator Code updated successfully"
    }
}
```

## Create a Home Network Key

Adds a new home network key for SUCI de-concealment. The key is identified by a (keyIdentifier, scheme) pair. Profile A keys use Curve25519 (X25519); Profile B keys use NIST P-256. Maximum 12 keys.

| Method | Path                                 |
| ------ | ------------------------------------ |
| POST   | `/api/v1/operator/home-network-keys` |

### Parameters

- `keyIdentifier` (integer): The key identifier. Must be between 0 and 255. Must match the value provisioned on the SIM/USIM.
- `scheme` (string): The scheme. Must be `"A"` (Curve25519/X25519) or `"B"` (NIST P-256).
- `privateKey` (string): The private key. Must be a 64-character hexadecimal string.

### Sample Response

```
{
    "result": {
        "message": "Home network key created successfully"
    }
}
```

## Get a Home Network Key's Private Key

Returns the private key for a home network key. This is a sensitive operation that is recorded in the audit log. Only administrators and network managers can access this endpoint.

| Method | Path                                                  |
| ------ | ----------------------------------------------------- |
| GET    | `/api/v1/operator/home-network-keys/{id}/private-key` |

### Parameters

- `id` (integer, path): The database ID of the home network key.

### Sample Response

```
{
    "result": {
        "privateKey": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
    }
}
```

## Delete a Home Network Key

Removes a home network key. UEs using this key will no longer be able to register.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| DELETE | `/api/v1/operator/home-network-keys/{id}` |

### Parameters

- `id` (integer, path): The database ID of the home network key.

### Sample Response

```
{
    "result": {
        "message": "Home network key deleted successfully"
    }
}
```

## Update the NAS Security Algorithms

This path updates the NAS security algorithm preference order for ciphering and integrity protection. The order determines which algorithms the network prefers during subscriber device security capability negotiation. Changes take effect for the next subscriber registration.

| Method | Path                            |
| ------ | ------------------------------- |
| PUT    | `/api/v1/operator/nas-security` |

### Parameters

- `ciphering` (array of strings): The preferred ciphering algorithm order. Each entry must be one of `NEA0`, `NEA1`, or `NEA2`. At least one algorithm is required. No duplicates allowed.
- `integrity` (array of strings): The preferred integrity algorithm order. Each entry must be one of `NIA0`, `NIA1`, or `NIA2`. At least one algorithm is required. No duplicates allowed.

### Sample Request

```
{
    "ciphering": ["NEA2", "NEA1"],
    "integrity": ["NIA2", "NIA1"]
}
```

### Sample Response

```
{
    "result": {
        "message": "Operator NAS security algorithms updated successfully"
    }
}
```

## Update the Service Provider Name (SPN)

This path updates the network name (Service Provider Name) displayed on connected devices. Both the full and short names are encoded in the GSM 7-bit alphabet and sent to subscriber devices in the NAS Configuration Update Command. Changes take effect for the next subscriber registration.

| Method | Path                   |
| ------ | ---------------------- |
| PUT    | `/api/v1/operator/spn` |

### Parameters

- `fullName` (string): The full network name shown on subscriber device displays. Must be between 1 and 50 characters.
- `shortName` (string): An abbreviated network name. Must be between 1 and 50 characters.

### Sample Request

```
{
    "fullName": "Ella Networks",
    "shortName": "Ella"
}
```

### Sample Response

```
{
    "result": {
        "message": "Operator SPN updated successfully"
    }
}
```
