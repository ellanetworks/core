---
description: RESTful API reference for managing the Operator Information - ID, Tracking, Code, Security Algorithms, Network Name (SPN), and SMS.
---

# Operator

The Operator API provides endpoints to manage the Operator Information used to identify the operator: MCC, MNC, Tracking information, OP, NAS security algorithms, Service Provider Name and SMS settings.

## Get Operator Information

This path returns the complete operator information.

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
        "tracking": {
            "supportedTacs": [
                "001",
                "002",
                "003"
            ]
        },
        "homeNetworkKeys": [
                {
                    "id": "0197b3a4-5c6d-7e8f-9a0b-1c2d3e4f5a6b",
                    "keyIdentifier": 0,
                    "scheme": "A",
                    "publicKey": "021bd3c0ba857e6f45b6ecb76ad826fd27fecef441f23d0e418b645829261e16"
                }
            ],
        "nasSecurity": {
            "ciphering": ["AES", "SNOW3G", "NULL"],
            "integrity": ["AES", "SNOW3G", "NULL"]
        },
        "spn": {
            "fullName": "Ella Networks",
            "shortName": "Ella"
        },
        "sms": {
            "smsNumber": "+15550001111"
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

## Update the Operator Tracking Information

This path updates the operator tracking information. The Tracking Area Codes (TACs) are used to identify the tracking areas supported by the operator.

| Method | Path                        |
| ------ | --------------------------- |
| PUT    | `/api/v1/operator/tracking` |

### Parameters

- `supportedTacs` (array): An array of supported TACs (Tracking Area Codes). Each TAC must be a 6-character hex string.

### Sample Response

```json
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

```json
{
    "result": {
        "message": "Operator Code updated successfully"
    }
}
```

## Create a Home Network Key

This path adds a new home network key for SUCI de-concealment. The key is identified by a (keyIdentifier, scheme) pair. Profile A keys use Curve25519 (X25519); Profile B keys use NIST P-256. Maximum 12 keys.

| Method | Path                                    |
| ------ | --------------------------------------- |
| POST   | `/api/v1/operator/home-network-keys`    |

### Parameters

- `keyIdentifier` (integer): The key identifier. Must be between 0 and 255. Must match the value provisioned on the SIM/USIM.
- `scheme` (string): The scheme. Must be `"A"` (Curve25519/X25519) or `"B"` (NIST P-256).
- `privateKey` (string): The private key. Must be a 64-character hexadecimal string.

### Sample Response

```json
{
    "result": {
        "message": "Home network key created successfully"
    }
}
```

## Get a Home Network Key's Private Key

This path returns the private key for a home network key. This is a sensitive operation that is recorded in the audit log. Only administrators and network managers can access this endpoint.

| Method | Path                                                      |
| ------ | --------------------------------------------------------- |
| GET    | `/api/v1/operator/home-network-keys/{id}/private-key`     |

### Parameters

- `id` (string, path): The UUID of the home network key.

### Sample Response

```json
{
    "result": {
        "privateKey": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
    }
}
```

## Delete a Home Network Key

This path removes a home network key. UEs using this key will no longer be able to register.

| Method | Path                                        |
| ------ | ------------------------------------------- |
| DELETE | `/api/v1/operator/home-network-keys/{id}`   |

### Parameters

- `id` (string, path): The UUID of the home network key.

### Sample Response

```json
{
    "result": {
        "message": "Home network key deleted successfully"
    }
}
```

## Update the NAS Security Algorithms

This path updates the NAS security algorithm preference order for ciphering and integrity protection. The order determines which algorithms the network prefers during subscriber device security capability negotiation. Changes take effect for the next subscriber registration.

| Method | Path                        |
| ------ | --------------------------- |
| PUT    | `/api/v1/operator/nas-security` |

### Parameters

- `ciphering` (array of strings): The preferred ciphering algorithm order. Each entry must be one of `NULL`, `SNOW3G`, or `AES`. At least one algorithm is required, maximum 3. No duplicates allowed.
- `integrity` (array of strings): The preferred integrity algorithm order. Each entry must be one of `NULL`, `SNOW3G`, or `AES`. At least one algorithm is required, maximum 3. No duplicates allowed.

These algorithm names are RAT-neutral: Ella Core signals them as NEA/NIA to 5G subscribers and as EEA/EIA to 4G subscribers.

| Name | 5G | 4G |
| ---- | -- | -- |
| `NULL` | NEA0 / NIA0 | EEA0 / EIA0 |
| `SNOW3G` | 128-NEA1 / 128-NIA1 | 128-EEA1 / 128-EIA1 |
| `AES` | 128-NEA2 / 128-NIA2 | 128-EEA2 / 128-EIA2 |

### Sample Request

```json
{
    "ciphering": ["AES", "SNOW3G"],
    "integrity": ["AES", "SNOW3G"]
}
```

### Sample Response

```json
{
    "result": {
        "message": "Operator NAS security algorithms updated successfully"
    }
}
```

## Update the Service Provider Name (SPN)

This path updates the network name (Service Provider Name) displayed on connected devices. Both the full and short names are encoded in the GSM 7-bit alphabet and sent to subscriber devices in the NAS Configuration Update Command. Changes take effect for the next subscriber registration.

| Method | Path                    |
| ------ | ----------------------- |
| PUT    | `/api/v1/operator/spn`  |

### Parameters

- `fullName` (string): The full network name shown on subscriber device displays. Must be between 1 and 50 characters.
- `shortName` (string): An abbreviated network name. Must be between 1 and 50 characters.

### Sample Request

```json
{
    "fullName": "Ella Networks",
    "shortName": "Ella"
}
```

### Sample Response

```json
{
    "result": {
        "message": "Operator SPN updated successfully"
    }
}
```

## Update the SMS Settings

This path sets Ella Core's SMS number. SMS is available once the SMS number is set and at least one SMSC peer is configured.

| Method | Path                    |
| ------ | ----------------------- |
| PUT    | `/api/v1/operator/sms`  |

### Parameters

- `smsNumber` (string): Ella Core's E.164 number for SMS.

### Sample Request

```json
{
    "smsNumber": "+15550001111"
}
```

### Sample Response

```json
{
    "result": {
        "message": "Operator SMS settings updated successfully"
    }
}
```

## List SMSC Peers

This path returns the list of SMSC peers.

| Method | Path                                  |
| ------ | ------------------------------------- |
| GET    | `/api/v1/operator/sms/smsc-peers`     |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": "0199a1b2-3c4d-7e5f-8a6b-7c8d9e0f1a2b",
                "address": "192.0.2.10",
                "port": 3868,
                "serviceCentres": ["+15550000000"],
                "status": {
                    "state": "open",
                    "host": "smsc.example.org",
                    "realm": "example.org",
                    "since": "2026-09-29T13:44:09Z"
                }
            }
        ]
    }
}
```

## Get an SMSC Peer

This path returns an SMSC peer.

| Method | Path                                       |
| ------ | ------------------------------------------ |
| GET    | `/api/v1/operator/sms/smsc-peers/{id}`     |

### Parameters

None

## Create an SMSC Peer

This path creates an SMSC peer.

| Method | Path                                  |
| ------ | ------------------------------------- |
| POST   | `/api/v1/operator/sms/smsc-peers`     |

### Parameters

- `address` (string): The IPv4 or IPv6 address of the SMSC's Diameter endpoint.
- `port` (optional integer): The SCTP port of the SMSC's Diameter endpoint, between 1 and 65535. Defaults to `3868`.
- `serviceCentres` (array of strings): The E.164 service centre numbers the SMSC serves, for example `+15550000000`.

### Sample Request

```json
{
    "address": "192.0.2.10",
    "port": 3868,
    "serviceCentres": ["+15550000000"]
}
```

### Sample Response

```json
{
    "result": {
        "id": "0199a1b2-3c4d-7e5f-8a6b-7c8d9e0f1a2b",
        "address": "192.0.2.10",
        "port": 3868,
        "serviceCentres": ["+15550000000"]
    }
}
```

## Update an SMSC Peer

This path updates an SMSC peer.

| Method | Path                                       |
| ------ | ------------------------------------------ |
| PUT    | `/api/v1/operator/sms/smsc-peers/{id}`     |

### Parameters

Same as [Create an SMSC Peer](#create-an-smsc-peer), with `port` required.

### Sample Response

```json
{
    "result": {
        "message": "SMSC peer updated successfully"
    }
}
```

## Delete an SMSC Peer

This path deletes an SMSC peer.

| Method | Path                                       |
| ------ | ------------------------------------------ |
| DELETE | `/api/v1/operator/sms/smsc-peers/{id}`     |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "SMSC peer deleted successfully"
    }
}
```
