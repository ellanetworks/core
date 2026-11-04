---
description: RESTful API reference for managing the Operator Information - ID, Tracking, Code, Security Algorithms, Network Name (SPN), SMS, IMS, and Roaming.
---

# Operator

The Operator API provides endpoints to manage the Operator Information used to identify the operator: MCC, MNC, Tracking information, OP, NAS security algorithms, Service Provider Name, SMS settings, IMS settings and roaming (roaming partners, roaming Diameter peers, and the SEPP).

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
        },
        "ims": {
            "pcscfAddresses": ["192.0.2.20", "2001:db8::20"],
            "voiceStatus": "enabled"
        }
    }
}
```

`ims.voiceStatus` is `enabled`, `no_pcscf`, `no_ims_data_network`, or `ip_family_mismatch` (no P-CSCF address shares an IP version with the pools of the `ims` data network). `no_ims_data_network` means there is no IMS data network named `ims` (the IMS well-known APN).

## Update the Operator ID

This path updates the operator ID. The Mobile Country Code (MCC) and Mobile Network Code (MNC) are used to identify the operator. The operator ID can't be changed when there are subscribers created in the system. Changing it renames the Diameter identity and realm of every node, so the Ella Core peers configured in the IMS and in the SMSC must be updated afterwards. IMS application servers in the old IMS home domain move to the new one.

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
                "diameterIdentity": "smsc.example.org",
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

- `diameterIdentity` (optional string): The SMSC's Diameter identity (Origin-Host).
- `address` (string): The IPv4 or IPv6 address of the SMSC's Diameter endpoint.
- `port` (optional integer): The SCTP port of the SMSC's Diameter endpoint, between 1 and 65535. Defaults to `3868`.
- `serviceCentres` (array of strings): The E.164 service centre numbers the SMSC serves, for example `+15550000000`.

### Sample Request

```json
{
    "diameterIdentity": "smsc.example.org",
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
        "diameterIdentity": "smsc.example.org",
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

## Update the IMS Settings

This path sets the P-CSCF addresses of the IMS that Ella Core gives to UEs on IMS data networks.

| Method | Path                     |
| ------ | ------------------------ |
| PUT    | `/api/v1/operator/ims` |

### Parameters

- `pcscfAddresses` (array of strings): Unicast IPv4 and IPv6 addresses of the IMS, in preference order. At most 3 of each IP version, without duplicates. UEs receive the addresses of the IP version they request. An empty list clears it.

### Sample Request

```json
{
    "pcscfAddresses": ["192.0.2.20", "2001:db8::20"]
}
```

### Sample Response

```json
{
    "result": {
        "message": "Operator IMS settings updated successfully"
    }
}
```

## List IMS Application Servers

This path returns the list of IMS application servers, sorted by name.

| Method | Path                                         |
| ------ | -------------------------------------------- |
| GET    | `/api/v1/operator/ims/application-servers`   |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "name": "mcptt",
                "server_name": "sip:mcptt.ims.example.org",
                "public_service_identities": ["sip:!.*!@mcptt.ims.example.org"]
            }
        ]
    }
}
```

## Get an IMS Application Server

This path returns an IMS application server.

| Method | Path                                                |
| ------ | --------------------------------------------------- |
| GET    | `/api/v1/operator/ims/application-servers/{name}`   |

### Parameters

None

## Create an IMS Application Server

This path creates an IMS application server.

| Method | Path                                         |
| ------ | -------------------------------------------- |
| POST   | `/api/v1/operator/ims/application-servers`   |

### Parameters

- `name` (string): The name of the application server.
- `server_name` (string): The SIP URI of the application server.
- `public_service_identities` (optional array of strings): The SIP or tel URIs the application server hosts. A wildcarded identity uses the `!regex!` syntax, for example `sip:!.*!@mcptt.ims.example.org`. Identities are stored in canonical form: lowercase scheme and host, no URI parameters, and tel numbers without visual separators. An identity can't be hosted by, or overlap one of, another application server, be or cover a subscriber's public identity (`sip:+<msisdn>@<IMS home domain>`, `tel:+<msisdn>` or `sip:mcptt-<msisdn>@<IMS home domain>`), or take the form of a subscriber identity (`sip:<IMSI>@<IMS home domain>`).

### Sample Request

```json
{
    "name": "mcptt",
    "server_name": "sip:mcptt.ims.example.org",
    "public_service_identities": ["sip:!.*!@mcptt.ims.example.org"]
}
```

### Sample Response

```json
{
    "result": {
        "name": "mcptt",
        "server_name": "sip:mcptt.ims.example.org",
        "public_service_identities": ["sip:!.*!@mcptt.ims.example.org"]
    }
}
```

## Update an IMS Application Server

This path updates an IMS application server. The server name and the public service identities are replaced.

| Method | Path                                                |
| ------ | --------------------------------------------------- |
| PUT    | `/api/v1/operator/ims/application-servers/{name}`   |

### Parameters

Same as [Create an IMS Application Server](#create-an-ims-application-server), without `name`.

### Sample Response

```json
{
    "result": {
        "message": "Application server updated successfully"
    }
}
```

## Delete an IMS Application Server

This path deletes an IMS application server. An application server used by the initial filter criteria of a service profile can't be deleted.

| Method | Path                                                |
| ------ | --------------------------------------------------- |
| DELETE | `/api/v1/operator/ims/application-servers/{name}`   |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Application server deleted successfully"
    }
}
```

## List IMS Service Profiles

This path returns the IMS service profiles, sorted by name.

| Method | Path                                    |
| ------ | --------------------------------------- |
| GET    | `/api/v1/operator/ims/service-profiles` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "name": "mcptt",
                "initial_filter_criteria": [
                    {
                        "priority": 0,
                        "application_server_name": "mcptt",
                        "trigger": {
                            "conditions_match": "all",
                            "groups": [{"conditions": [{"method": "REGISTER"}]}]
                        },
                        "default_handling": "SESSION_CONTINUED",
                        "include_register_request": true,
                        "include_register_response": true
                    }
                ]
            }
        ]
    }
}
```

## Get an IMS Service Profile

This path returns an IMS service profile.

| Method | Path                                           |
| ------ | ---------------------------------------------- |
| GET    | `/api/v1/operator/ims/service-profiles/{name}` |

### Parameters

None

## Create an IMS Service Profile

This path creates an IMS service profile.

| Method | Path                                    |
| ------ | --------------------------------------- |
| POST   | `/api/v1/operator/ims/service-profiles` |

### Parameters

- `name` (string): The name of the service profile.
- `initial_filter_criteria` (optional array): In priority order ([TS 29.228 Annex B.2.2](https://www.3gpp.org/DynaReport/29228.htm)). Each criterion has:
    - `application_server_name` (string): The name of the [IMS application server](#create-an-ims-application-server) matching requests are sent to.
    - `trigger` (optional object): Without it, the criterion matches every request.
        - `conditions_match` (string): `all` if a group matches when all of its conditions match, and the trigger matches when any group matches. `any` if a group matches when any of its conditions matches, and the trigger matches when every group matches.
        - `groups` (array): Groups of `conditions`. Each condition has an optional `negated` (boolean) and exactly one of `request_uri` (string), `method` (string), `sip_header` (`{header, content}`), `session_case` (`ORIGINATING_REGISTERED`, `TERMINATING_REGISTERED`, `TERMINATING_UNREGISTERED`, `ORIGINATING_UNREGISTERED` or `ORIGINATING_CDIV`) and `session_description` (`{line, content}`). With the `REGISTER` method, `registration_types` lists up to two of `INITIAL_REGISTRATION`, `RE-REGISTRATION` and `DE-REGISTRATION`. Request URIs, headers, lines and contents are extended regular expressions.
    - `profile_part` (optional string): `REGISTERED` or `UNREGISTERED`, the part of the user profile the criterion belongs to. Omitted for both.
    - `default_handling` (optional string): `SESSION_CONTINUED` or `SESSION_TERMINATED`, what the S-CSCF does when the application server can't be reached.
    - `service_info` (optional string): Passed transparently to the application server.
    - `include_register_request` (optional boolean)
    - `include_register_response` (optional boolean)

The `priority` of a criterion is its position in the list. It is returned on reads and ignored on writes.

### Sample Request

```json
{
    "name": "conference",
    "initial_filter_criteria": [
        {
            "application_server_name": "conference",
            "trigger": {
                "conditions_match": "all",
                "groups": [{"conditions": [{"request_uri": "^sip:conference@.*$"}]}]
            },
            "default_handling": "SESSION_TERMINATED"
        }
    ]
}
```

### Sample Response

```json
{
    "result": {
        "name": "conference",
        "initial_filter_criteria": [
            {
                "priority": 0,
                "application_server_name": "conference",
                "trigger": {
                    "conditions_match": "all",
                    "groups": [{"conditions": [{"request_uri": "^sip:conference@.*$"}]}]
                },
                "default_handling": "SESSION_TERMINATED",
                "include_register_request": false,
                "include_register_response": false
            }
        ]
    }
}
```

## Update an IMS Service Profile

This path replaces the initial filter criteria of an IMS service profile. Registered subscribers whose profile assigns it get the new user profile.

| Method | Path                                           |
| ------ | ---------------------------------------------- |
| PUT    | `/api/v1/operator/ims/service-profiles/{name}` |

### Parameters

- `initial_filter_criteria` (array): In priority order, as in [Create an IMS Service Profile](#create-an-ims-service-profile). An empty list removes all of them.

### Sample Response

```json
{
    "result": {
        "message": "Service profile updated successfully"
    }
}
```

## Delete an IMS Service Profile

This path deletes an IMS service profile. A service profile a profile assigns can't be deleted.

| Method | Path                                           |
| ------ | ---------------------------------------------- |
| DELETE | `/api/v1/operator/ims/service-profiles/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Service profile deleted successfully"
    }
}
```

## List Roaming Partners

This path returns the list of roaming partners. For a partner with a 5G agreement, `n32` gives the N32-c context of this node, when it has `interfaces.n32`.

| Method | Path                                |
| ------ | ----------------------------------- |
| GET    | `/api/v1/operator/roaming/partners` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": "0199a3c2-1f4e-7d21-8c9b-5a6e7f8091a2",
                "name": "Partner Telecom",
                "plmns": [
                    {"mcc": "999", "mnc": "70"}
                ],
                "profileName": "roamers",
                "sepp": {
                    "fqdn": "sepp1.5gc.mnc070.mcc999.3gppnetwork.org",
                    "n32cURI": "https://sepp1.5gc.mnc070.mcc999.3gppnetwork.org:443",
                    "trustAnchorPEM": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"
                },
                "diameterPeerID": "0199a3c2-5c1e-7b8e-9f3a-2d4c6e8f0a1b",
                "n32": {
                    "state": "established",
                    "since": "2026-11-03T09:12:44Z",
                    "initiator": "local",
                    "nftlst": true
                }
            }
        ]
    }
}
```

| N32 field    | Description |
| ------------ | ----------- |
| `state`      | `idle`, `establishing`, `established`, `failed`, `terminated`, or `unknown` (the handshake could not start). |
| `initiator`  | `local` or `peer`. |
| `nftlst`     | Whether both SEPPs can tear the context down. |
| `peer`       | The partner SEPP the context is with, when the handshake was redirected. |
| `lastError`  | The reason of the last failure. |
| `otherPeers` | Contexts that other SEPPs of the partner established with the node. |

## Create a Roaming Partner

This path creates a new roaming partner.

| Method | Path                                |
| ------ | ----------------------------------- |
| POST   | `/api/v1/operator/roaming/partners` |

### Parameters

- `name` (string): The name of the partner. Must be unique.
- `plmns` (array): The partner's PLMN IDs, at least one. Two PLMN IDs conflict when they have the same MCC and their MNCs are equal once zero-padded to 3 digits, or one is a prefix of the other. A PLMN ID must not conflict with another of the partner, with your own PLMN ID, or with another partner's.
    - `mcc` (string): 3 digits.
    - `mnc` (string): 2 or 3 digits.
    - `nrfAPIRoot` (string, optional): The apiRoot (`http` or `https`) of the NRF of this PLMN, for its roamers. Defaults to `https://nrf.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org` of this PLMN ID. Needs `sepp`.
- `profileName` (string): The name of the profile applied to the partner's roamers.
- `sepp` (object, optional): The 5G agreement. Absent means no 5G roaming.
    - `fqdn` (string): The partner SEPP's FQDN, as in its certificate: `<host>.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, or `<host>.5gc.mnc<MNC>.mcc<MCC>.<IPX ID>.ipxnetwork.org`, for one of the partner's PLMN IDs. Must not be the well-known `sepp.5gc…` FQDN. Must be unique across partners.
    - `n32cURI` (string): The apiRoot of the partner SEPP's N32-c endpoint: `https`, host, optional port and optional path prefix.
    - `n32fURI` (string, optional): The apiRoot of the partner SEPP's N32-f endpoint. Defaults to `n32cURI`.
    - `trustAnchorPEM` (string): The CA certificates that the partner SEPP's certificate chains to, or the partner's self-signed certificate, in PEM. Each certificate must be valid, with an ECDSA key or an RSA key of at least 2048 bits.
- `diameterPeerID` (string, optional): The ID of the [roaming Diameter peer](#create-a-roaming-diameter-peer) of the 4G agreement. Absent means no 4G roaming.

### Sample Request

```json
{
    "name": "Partner Telecom",
    "plmns": [{"mcc": "999", "mnc": "70"}],
    "profileName": "roamers",
    "sepp": {
        "fqdn": "sepp1.5gc.mnc070.mcc999.3gppnetwork.org",
        "n32cURI": "https://sepp1.5gc.mnc070.mcc999.3gppnetwork.org:443",
        "trustAnchorPEM": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"
    },
    "diameterPeerID": "0199a3c2-5c1e-7b8e-9f3a-2d4c6e8f0a1b"
}
```

### Responses

| Status | Condition |
| ------ | --------- |
| `201`  | Returns the created partner, without `n32`. |
| `400`  | Invalid parameters. |
| `404`  | The profile or the roaming Diameter peer does not exist. |
| `409`  | The name is taken, or a PLMN ID conflicts. |

### Sample Response

```json
{
    "result": {
        "id": "0199a3c2-1f4e-7d21-8c9b-5a6e7f8091a2",
        "name": "Partner Telecom",
        "plmns": [
            {"mcc": "999", "mnc": "70"}
        ],
        "profileName": "roamers",
        "sepp": {
            "fqdn": "sepp1.5gc.mnc070.mcc999.3gppnetwork.org",
            "n32cURI": "https://sepp1.5gc.mnc070.mcc999.3gppnetwork.org:443",
            "trustAnchorPEM": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"
        },
        "diameterPeerID": "0199a3c2-5c1e-7b8e-9f3a-2d4c6e8f0a1b"
    }
}
```

## Get a Roaming Partner

This path returns the details of a specific roaming partner, as in [List Roaming Partners](#list-roaming-partners).

| Method | Path                                     |
| ------ | ---------------------------------------- |
| GET    | `/api/v1/operator/roaming/partners/{id}` |

### Parameters

None

## Update a Roaming Partner

This path updates an existing roaming partner. Removing a PLMN ID or an agreement first deregisters the affected roamers on every node, and purges them at their home network.

| Method | Path                                     |
| ------ | ---------------------------------------- |
| PUT    | `/api/v1/operator/roaming/partners/{id}` |

### Parameters

The parameters of [Create a Roaming Partner](#create-a-roaming-partner).

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | The partner is updated. |
| `400`  | Invalid parameters. |
| `404`  | The partner, the profile or the roaming Diameter peer does not exist. |
| `409`  | The name is taken, a PLMN ID conflicts, or the affected roamers were not all deregistered within one minute. Nothing is changed. |

### Sample Response

```json
{
    "result": {
        "message": "Roaming partner updated successfully"
    }
}
```

## Delete a Roaming Partner

This path deletes a roaming partner. Its roamers are first deregistered on every node, and purged at their home network. The usage of its roamers is kept.

| Method | Path                                     |
| ------ | ---------------------------------------- |
| DELETE | `/api/v1/operator/roaming/partners/{id}` |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | The partner is deleted. |
| `404`  | The partner does not exist. |
| `409`  | The roamers were not all deregistered within one minute. Nothing is changed. |

### Sample Response

```json
{
    "result": {
        "message": "Roaming partner deleted successfully"
    }
}
```

## Re-run the N32 Handshake

This path starts a new N32-c handshake with the partner's SEPP on this node. It returns the state right after the start.

| Method | Path                                               |
| ------ | -------------------------------------------------- |
| POST   | `/api/v1/operator/roaming/partners/{id}/handshake` |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `202`  | The handshake started. |
| `404`  | The partner does not exist. |
| `409`  | The partner has no 5G agreement, or this node has no `interfaces.n32`. |

### Sample Response

```json
{
    "result": {
        "n32": {
            "state": "establishing",
            "since": "2026-11-03T10:02:17Z",
            "nftlst": false
        }
    }
}
```

## List Roaming Diameter Peers

This path returns the list of roaming Diameter peers: the HSS or Diameter Edge Agent of roaming partners.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| GET    | `/api/v1/operator/roaming/diameter-peers` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": "0199a3c2-5c1e-7b8e-9f3a-2d4c6e8f0a1b",
                "diameterIdentity": "hss.epc.mnc070.mcc999.3gppnetwork.org",
                "address": "198.51.100.5",
                "port": 3868
            }
        ]
    }
}
```

## Create a Roaming Diameter Peer

This path creates a new roaming Diameter peer.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| POST   | `/api/v1/operator/roaming/diameter-peers` |

### Parameters

- `address` (string): The IPv4 or IPv6 address of the peer.
- `port` (integer, optional): The peer's SCTP port. Defaults to `3868`.
- `diameterIdentity` (string, optional): The peer's Diameter identity (Origin-Host). When set, the peer must give this identity when it connects.

### Responses

| Status | Condition |
| ------ | --------- |
| `201`  | Returns the created peer. |
| `400`  | Invalid parameters. |
| `409`  | Another Diameter peer has the identity or the address. |

### Sample Response

```json
{
    "result": {
        "id": "0199a3c2-5c1e-7b8e-9f3a-2d4c6e8f0a1b",
        "diameterIdentity": "hss.epc.mnc070.mcc999.3gppnetwork.org",
        "address": "198.51.100.5",
        "port": 3868
    }
}
```

## Get a Roaming Diameter Peer

This path returns the details of a specific roaming Diameter peer.

| Method | Path                                           |
| ------ | ---------------------------------------------- |
| GET    | `/api/v1/operator/roaming/diameter-peers/{id}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "id": "0199a3c2-5c1e-7b8e-9f3a-2d4c6e8f0a1b",
        "diameterIdentity": "hss.epc.mnc070.mcc999.3gppnetwork.org",
        "address": "198.51.100.5",
        "port": 3868
    }
}
```

## Update a Roaming Diameter Peer

This path updates an existing roaming Diameter peer.

| Method | Path                                           |
| ------ | ---------------------------------------------- |
| PUT    | `/api/v1/operator/roaming/diameter-peers/{id}` |

### Parameters

- `address` (string): The IPv4 or IPv6 address of the peer.
- `port` (integer): The peer's SCTP port.
- `diameterIdentity` (string, optional): The peer's Diameter identity (Origin-Host). When set, the peer must give this identity when it connects.

### Sample Response

```json
{
    "result": {
        "message": "Roaming Diameter peer updated successfully"
    }
}
```

## Delete a Roaming Diameter Peer

This path deletes a roaming Diameter peer. A peer used by a roaming partner cannot be deleted.

| Method | Path                                           |
| ------ | ---------------------------------------------- |
| DELETE | `/api/v1/operator/roaming/diameter-peers/{id}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Roaming Diameter peer deleted successfully"
    }
}
```

## Get the SEPP Status

This path returns the SEPP identity, certificate and N32 listener of this node, the trust anchors to give roaming partners, and the stored certificate of each SEPP ID.

| Method | Path                            |
| ------ | ------------------------------- |
| GET    | `/api/v1/operator/roaming/sepp` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "node": {
            "seppID": 1,
            "fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
            "source": "generated",
            "n32Configured": true,
            "address": "192.0.2.11",
            "port": 443,
            "listening": true,
            "certificate": {
                "subject": "CN=sepp1.5gc.mnc001.mcc001.3gppnetwork.org,O=5gc.mnc001.mcc001.3gppnetwork.org",
                "sans": ["sepp1.5gc.mnc001.mcc001.3gppnetwork.org"],
                "issuer": "CN=SEPP CA,O=5gc.mnc001.mcc001.3gppnetwork.org",
                "notBefore": "2026-10-01T08:00:00Z",
                "notAfter": "2029-10-01T08:00:00Z",
                "fingerprint": "sha256:a1b2...7e",
                "trustAnchorFingerprint": "sha256:5f0c...9a"
            }
        },
        "trustAnchors": [
            {
                "source": "generated",
                "subject": "CN=SEPP CA,O=5gc.mnc001.mcc001.3gppnetwork.org",
                "fingerprint": "sha256:5f0c...9a",
                "notAfter": "2036-10-01T08:00:00Z",
                "pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
                "seppIDs": [1, 2]
            }
        ],
        "certificates": [
            {
                "seppID": 1,
                "fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
                "source": "generated",
                "certificate": {
                    "subject": "CN=sepp1.5gc.mnc001.mcc001.3gppnetwork.org,O=5gc.mnc001.mcc001.3gppnetwork.org",
                    "sans": ["sepp1.5gc.mnc001.mcc001.3gppnetwork.org"],
                    "issuer": "CN=SEPP CA,O=5gc.mnc001.mcc001.3gppnetwork.org",
                    "notBefore": "2026-10-01T08:00:00Z",
                    "notAfter": "2029-10-01T08:00:00Z",
                    "fingerprint": "sha256:a1b2...7e",
                    "trustAnchorFingerprint": "sha256:5f0c...9a"
                }
            }
        ]
    }
}
```

`seppIDs` lists the nodes that use each trust anchor.

## Get a SEPP Certificate

This path returns a node's SEPP certificate chain and trust anchor, with a warning for each departure from the TS 33.310 SEPP certificate profile.

| Method | Path                                                       |
| ------ | ---------------------------------------------------------- |
| GET    | `/api/v1/operator/roaming/sepp/nodes/{seppID}/certificate` |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | Returns the certificate. |
| `400`  | `seppID` is not between 1 and 63. |
| `404`  | The node has no SEPP certificate. |

### Sample Response

```json
{
    "result": {
        "seppID": 1,
        "fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
        "source": "generated",
        "certificateChainPEM": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
        "trustAnchorPEM": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
        "fingerprint": "sha256:a1b2...7e",
        "notAfter": "2029-10-01T08:00:00Z",
        "warnings": [
            "The certificate has no cRLDistributionPoints, which TS 33.310 clause 6.1.3c.3 requires: partners can only pin it"
        ]
    }
}
```

## Import a SEPP Certificate

This path replaces a node's SEPP certificate with one issued by your CA. The node runs the N32 handshake again with every partner.

| Method | Path                                                       |
| ------ | ---------------------------------------------------------- |
| PUT    | `/api/v1/operator/roaming/sepp/nodes/{seppID}/certificate` |

### Parameters

- `certificateChainPEM` (string): The certificate, then any intermediates, ending with the CA certificate that partners trust. The certificate must be an end-entity certificate with keyUsage digitalSignature, extendedKeyUsage serverAuth and clientAuth (or none), and a subjectAltName DNS name under `5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org` of your PLMN ID.
- `privateKeyPEM` (string): The certificate's private key: ECDSA P-256 or P-384, or RSA of at least 2048 bits.
- `fqdn` (string, optional): The node's SEPP FQDN, when the certificate has several DNS names. Defaults to the subject CN.

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | Returns the imported certificate, as in [Get a SEPP Certificate](#get-a-sepp-certificate). |
| `400`  | Invalid certificate or key. |
| `409`  | Another node has this FQDN. |

## Generate a SEPP Certificate

This path replaces a node's SEPP certificate with a new one signed by the cluster SEPP CA.

| Method | Path                                                                |
| ------ | ------------------------------------------------------------------- |
| POST   | `/api/v1/operator/roaming/sepp/nodes/{seppID}/certificate/generate` |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `201`  | Returns the generated certificate, as in [Get a SEPP Certificate](#get-a-sepp-certificate). |
| `400`  | `seppID` is not between 1 and 63. |

## Regenerate the SEPP CA

This path replaces the cluster SEPP CA and reissues every generated SEPP certificate. Imported certificates are kept.

| Method | Path                                          |
| ------ | --------------------------------------------- |
| POST   | `/api/v1/operator/roaming/sepp/ca/regenerate` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "trustAnchor": {
            "source": "generated",
            "subject": "CN=SEPP CA,O=5gc.mnc001.mcc001.3gppnetwork.org",
            "fingerprint": "sha256:c47d...12",
            "notAfter": "2036-11-03T10:00:00Z",
            "pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
            "seppIDs": []
        },
        "reissued": [1, 2],
        "warnings": [
            "The cluster SEPP CA changed: every roaming partner must import the new trust anchor before N32 with the nodes that use generated certificates works again"
        ]
    }
}
```
