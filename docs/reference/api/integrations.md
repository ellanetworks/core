---
description: RESTful API reference for managing the integrations of Ella Core with external systems - SMS, IMS, roaming, and Diameter.
---

# Integrations

The Integrations API provides endpoints to manage how Ella Core connects to external systems: SMS settings and SMSCs, IMS settings, allowed IMS peers, IMS application servers and service profiles, roaming (roaming partners, roaming Diameter peers, and the SEPP), and the Diameter status.

## Get the SMS Settings

This path returns Ella Core's serving node number.

| Method | Path                       |
| ------ | -------------------------- |
| GET    | `/api/v1/integrations/sms` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "servingNodeNumber": "+15550001111"
    }
}
```

## Update the SMS Settings

This path sets Ella Core's serving node number. SMS is available once the serving node number is set and at least one SMSC is configured.

| Method | Path                       |
| ------ | -------------------------- |
| PUT    | `/api/v1/integrations/sms` |

### Parameters

- `servingNodeNumber` (string): Ella Core's E.164 number for SMS. An empty string clears it.

### Sample Request

```json
{
    "servingNodeNumber": "+15550001111"
}
```

### Sample Response

```json
{
    "result": {
        "message": "SMS settings updated successfully"
    }
}
```

## List SMSCs

This path returns the list of SMSCs.

| Method | Path                             |
| ------ | -------------------------------- |
| GET    | `/api/v1/integrations/sms/smscs` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "name": "main",
                "serviceCentres": ["+15550000000"],
                "nodes": [
                    {
                        "originHost": "smsc1.node.example.org",
                        "address": "192.0.2.10",
                        "port": 3868,
                        "status": {
                            "state": "open",
                            "originHost": "smsc1.node.example.org",
                            "realm": "example.org",
                            "since": "2026-09-29T13:44:09Z"
                        }
                    },
                    {
                        "originHost": "smsc2.node.example.org",
                        "address": "192.0.2.11",
                        "port": 3868,
                        "status": {
                            "state": "down",
                            "since": "2026-09-29T13:44:09Z",
                            "error": "dial 192.0.2.11:3868: connection refused"
                        }
                    }
                ]
            }
        ]
    }
}
```

## Get an SMSC

This path returns an SMSC.

| Method | Path                                    |
| ------ | --------------------------------------- |
| GET    | `/api/v1/integrations/sms/smscs/{name}` |

### Parameters

None

## Create an SMSC

This path creates an SMSC.

| Method | Path                             |
| ------ | -------------------------------- |
| POST   | `/api/v1/integrations/sms/smscs` |

### Parameters

- `name` (string): The name of the SMSC.
- `serviceCentres` (array of strings): The E.164 service centre numbers the SMSC serves, for example `+15550000000`.
- `nodes` (array of objects): The SMSC's Diameter nodes.
    - `originHost` (optional string): The node's Origin-Host. When set, the node must give this Origin-Host when it connects.
    - `address` (string): The IPv4 or IPv6 address or the host name of the node's Diameter endpoint.
    - `port` (optional integer): The SCTP port of the node's Diameter endpoint, between 1 and 65535. Defaults to `3868`.

### Sample Request

```json
{
    "name": "main",
    "serviceCentres": ["+15550000000"],
    "nodes": [
        {
            "originHost": "smsc1.node.example.org",
            "address": "192.0.2.10"
        },
        {
            "originHost": "smsc2.node.example.org",
            "address": "192.0.2.11"
        }
    ]
}
```

### Sample Response

```json
{
    "result": {
        "name": "main",
        "serviceCentres": ["+15550000000"],
        "nodes": [
            {
                "originHost": "smsc1.node.example.org",
                "address": "192.0.2.10",
                "port": 3868
            },
            {
                "originHost": "smsc2.node.example.org",
                "address": "192.0.2.11",
                "port": 3868
            }
        ]
    }
}
```

## Update an SMSC

This path updates an SMSC.

| Method | Path                                    |
| ------ | --------------------------------------- |
| PUT    | `/api/v1/integrations/sms/smscs/{name}` |

### Parameters

Same as [Create an SMSC](#create-an-smsc), without `name`.

### Sample Response

```json
{
    "result": {
        "message": "SMSC updated successfully"
    }
}
```

## Delete an SMSC

This path deletes an SMSC.

| Method | Path                                    |
| ------ | --------------------------------------- |
| DELETE | `/api/v1/integrations/sms/smscs/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "SMSC deleted successfully"
    }
}
```

## Get the IMS Settings

This path returns the P-CSCF addresses, the voice status, the IMS home domain, whether IMS voice is advertised, and whether peers are restricted to the allowed IMS peers.

| Method | Path                       |
| ------ | -------------------------- |
| GET    | `/api/v1/integrations/ims` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "pcscfAddresses": ["192.0.2.20", "2001:db8::20"],
        "voiceStatus": "enabled",
        "homeDomain": "ims.example.org",
        "derivedHomeDomain": "ims.mnc001.mcc001.3gppnetwork.org",
        "advertiseVoice": true,
        "restrictPeers": true
    }
}
```

| Field | Description |
| ----- | ----------- |
| `voiceStatus` | Whether the IMS configuration is ready for voice: `enabled`, `no_pcscf`, `no_ims_data_network` (no IMS data network named `ims`), or `ip_family_mismatch` (no P-CSCF address shares an IP version with the pools of the `ims` data network). |
| `homeDomain` | The configured IMS home domain. Empty when `derivedHomeDomain` is used. |
| `advertiseVoice` | `true` when Ella Core tells home UEs that IMS voice is supported. |
| `restrictPeers` | `true` when only peers matching an [allowed IMS peer](#create-an-allowed-ims-peer) may connect. |

## Update the IMS Settings

This path sets the P-CSCF addresses, the IMS home domain, whether IMS voice is advertised, and whether peers are restricted to the allowed IMS peers. Changing the home domain moves the identities of the IMS application servers to the new domain and de-registers the subscribers whose public identities change.

| Method | Path                       |
| ------ | -------------------------- |
| PUT    | `/api/v1/integrations/ims` |

### Parameters

- `pcscfAddresses` (array of strings): Unicast IPv4 and IPv6 addresses or host names of the IMS, in preference order. At most 10, with at most 5 addresses of each IP version, without duplicates. UEs receive the addresses of the IP version they request. An empty list clears it. A changed list applies to new IMS sessions.
- `homeDomain` (string): The IMS home domain, a domain name with at least two labels. An empty string, or the derived domain itself, uses the domain derived from the operator ID, `ims.mnc<MNC>.mcc<MCC>.3gppnetwork.org`. The EPC realm and `3gppnetwork.org` domains of other PLMNs are rejected. Ella Core accepts Diameter requests addressed to it.
- `advertiseVoice` (boolean): Whether Ella Core tells every home UE that IMS voice is supported. It applies at each UE's next registration, attach or tracking area update. Roamers are never told.
- `restrictPeers` (boolean): Whether only peers matching an [allowed IMS peer](#create-an-allowed-ims-peer) may connect. With no allowed IMS peer, every peer is rejected. Turning it on disconnects connected peers that match none.

All four are required.

### Sample Request

```json
{
    "pcscfAddresses": ["pcscf.ims.example.org", "192.0.2.20", "2001:db8::20"],
    "homeDomain": "ims.example.org",
    "advertiseVoice": true,
    "restrictPeers": true
}
```

### Sample Response

```json
{
    "result": {
        "message": "IMS settings updated successfully"
    }
}
```

## List Allowed IMS Peers

This path returns the allowed IMS peers, each with this node's connections from the peers it matches.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| GET    | `/api/v1/integrations/ims/allowed-peers` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "name": "cscf",
                "realm": "ims.example.org",
                "addresses": ["10.3.0.0/24", "2001:db8:3::/64"],
                "connections": [
                    {
                        "state": "open",
                        "originHost": "scscf.ims.example.org",
                        "realm": "ims.example.org",
                        "since": "2026-10-05T13:44:09Z"
                    }
                ]
            }
        ]
    }
}
```

## Get an Allowed IMS Peer

This path returns an allowed IMS peer.

| Method | Path                                             |
| ------ | ------------------------------------------------ |
| GET    | `/api/v1/integrations/ims/allowed-peers/{name}` |

### Parameters

None

## Create an Allowed IMS Peer

This path creates an allowed IMS peer. At most 32 allowed IMS peers can be configured.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| POST   | `/api/v1/integrations/ims/allowed-peers` |

### Parameters

- `name` (string): The name of the allowed IMS peer.
- `originHost` (optional string): The Origin-Host the peer must give, a fully qualified domain name, compared case-insensitively.
- `realm` (optional string): The Origin-Realm the peer must give, a fully qualified domain name.
- `addresses` (array of strings): The IPv4 or IPv6 addresses or CIDR prefixes the peer connects from, at least one and at most 16, e.g. `10.3.0.5` or `10.3.0.0/24`. A bare address means a `/32` or `/128` prefix. `0.0.0.0/0` and `::/0` allow any source address. They are returned as canonical prefixes, sorted and without duplicates.

At least one of `originHost` and `realm` is required.

A Diameter peer matches an allowed IMS peer when it connects from one of the `addresses` and gives the `originHost` and `realm` that are set. Matching peers are labelled `ims`. When `restrictPeers` is set in the [IMS settings](#update-the-ims-settings), other peers are rejected.

### Sample Request

```json
{
    "name": "cscf",
    "realm": "ims.example.org",
    "addresses": ["10.3.0.0/24", "2001:db8:3::/64"]
}
```

### Responses

| Status | Condition |
| ------ | --------- |
| `201`  | Returns the created allowed IMS peer. |
| `400`  | Invalid parameters. |
| `409`  | The name is taken, or another allowed IMS peer has the same Origin-Host, realm and addresses. |

### Sample Response

```json
{
    "result": {
        "name": "cscf",
        "realm": "ims.example.org",
        "addresses": ["10.3.0.0/24", "2001:db8:3::/64"],
        "connections": []
    }
}
```

## Update an Allowed IMS Peer

This path replaces the Origin-Host, realm and addresses of an allowed IMS peer. When `restrictPeers` is set, connected peers that no longer match any allowed IMS peer are disconnected.

| Method | Path                                             |
| ------ | ------------------------------------------------ |
| PUT    | `/api/v1/integrations/ims/allowed-peers/{name}` |

### Parameters

Same as [Create an Allowed IMS Peer](#create-an-allowed-ims-peer), without `name`.

### Sample Response

```json
{
    "result": {
        "message": "Allowed IMS peer updated successfully"
    }
}
```

## Delete an Allowed IMS Peer

This path deletes an allowed IMS peer. When `restrictPeers` is set, connected peers that no longer match any allowed IMS peer are disconnected.

| Method | Path                                             |
| ------ | ------------------------------------------------ |
| DELETE | `/api/v1/integrations/ims/allowed-peers/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Allowed IMS peer deleted successfully"
    }
}
```

## List IMS Application Servers

This path returns the list of IMS application servers, sorted by name.

| Method | Path                                           |
| ------ | ---------------------------------------------- |
| GET    | `/api/v1/integrations/ims/application-servers` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "name": "mcptt",
                "serverName": "sip:mcptt.ims.example.org",
                "publicServiceIdentities": ["sip:!.*!@mcptt.ims.example.org"]
            }
        ]
    }
}
```

## Get an IMS Application Server

This path returns an IMS application server.

| Method | Path                                                  |
| ------ | ----------------------------------------------------- |
| GET    | `/api/v1/integrations/ims/application-servers/{name}` |

### Parameters

None

## Create an IMS Application Server

This path creates an IMS application server.

| Method | Path                                           |
| ------ | ---------------------------------------------- |
| POST   | `/api/v1/integrations/ims/application-servers` |

### Parameters

- `name` (string): The name of the application server.
- `serverName` (string): The SIP URI of the application server.
- `publicServiceIdentities` (optional array of strings): The SIP or tel URIs the application server hosts. A wildcarded identity uses the `!regex!` syntax, for example `sip:!.*!@mcptt.ims.example.org`. Identities are stored in canonical form: lowercase scheme and host, no URI parameters, and tel numbers without visual separators. An identity can't be hosted by, or overlap one of, another application server, be or cover a subscriber's public identity (`sip:+<msisdn>@<IMS home domain>`, `tel:+<msisdn>` or `sip:mcptt-<msisdn>@<IMS home domain>`), or take the form of a subscriber identity (`sip:<IMSI>@<IMS home domain>`).

### Sample Request

```json
{
    "name": "mcptt",
    "serverName": "sip:mcptt.ims.example.org",
    "publicServiceIdentities": ["sip:!.*!@mcptt.ims.example.org"]
}
```

### Sample Response

```json
{
    "result": {
        "name": "mcptt",
        "serverName": "sip:mcptt.ims.example.org",
        "publicServiceIdentities": ["sip:!.*!@mcptt.ims.example.org"]
    }
}
```

## Update an IMS Application Server

This path updates an IMS application server. The server name and the public service identities are replaced.

| Method | Path                                                  |
| ------ | ----------------------------------------------------- |
| PUT    | `/api/v1/integrations/ims/application-servers/{name}` |

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

| Method | Path                                                  |
| ------ | ----------------------------------------------------- |
| DELETE | `/api/v1/integrations/ims/application-servers/{name}` |

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

| Method | Path                                        |
| ------ | ------------------------------------------- |
| GET    | `/api/v1/integrations/ims/service-profiles` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "name": "mcptt",
                "initialFilterCriteria": [
                    {
                        "priority": 0,
                        "applicationServerName": "mcptt",
                        "trigger": {
                            "conditionsMatch": "all",
                            "groups": [{"conditions": [{"method": "REGISTER"}]}]
                        },
                        "defaultHandling": "SESSION_CONTINUED",
                        "includeRegisterRequest": true,
                        "includeRegisterResponse": true
                    }
                ]
            }
        ]
    }
}
```

## Get an IMS Service Profile

This path returns an IMS service profile.

| Method | Path                                               |
| ------ | -------------------------------------------------- |
| GET    | `/api/v1/integrations/ims/service-profiles/{name}` |

### Parameters

None

## Create an IMS Service Profile

This path creates an IMS service profile.

| Method | Path                                        |
| ------ | ------------------------------------------- |
| POST   | `/api/v1/integrations/ims/service-profiles` |

### Parameters

- `name` (string): The name of the service profile.
- `initialFilterCriteria` (optional array): In priority order ([TS 29.228 Annex B.2.2](https://www.3gpp.org/DynaReport/29228.htm)). Each criterion has:
    - `applicationServerName` (string): The name of the [IMS application server](#create-an-ims-application-server) matching requests are sent to.
    - `trigger` (optional object): Without it, the criterion matches every request.
        - `conditionsMatch` (string): `all` if a group matches when all of its conditions match, and the trigger matches when any group matches. `any` if a group matches when any of its conditions matches, and the trigger matches when every group matches.
        - `groups` (array): Groups of `conditions`. Each condition has an optional `negated` (boolean) and exactly one of `requestURI` (string), `method` (string), `sipHeader` (`{header, content}`), `sessionCase` (`ORIGINATING_REGISTERED`, `TERMINATING_REGISTERED`, `TERMINATING_UNREGISTERED`, `ORIGINATING_UNREGISTERED` or `ORIGINATING_CDIV`) and `sessionDescription` (`{line, content}`). With the `REGISTER` method, `registrationTypes` lists up to two of `INITIAL_REGISTRATION`, `RE-REGISTRATION` and `DE-REGISTRATION`. Request URIs, headers, lines and contents are extended regular expressions.
    - `profilePart` (optional string): `REGISTERED` or `UNREGISTERED`, the part of the user profile the criterion belongs to. Omitted for both.
    - `defaultHandling` (optional string): `SESSION_CONTINUED` or `SESSION_TERMINATED`, what the S-CSCF does when the application server can't be reached.
    - `serviceInfo` (optional string): Passed transparently to the application server.
    - `includeRegisterRequest` (optional boolean)
    - `includeRegisterResponse` (optional boolean)

The `priority` of a criterion is its position in the list. It is returned on reads and ignored on writes.

### Sample Request

```json
{
    "name": "conference",
    "initialFilterCriteria": [
        {
            "applicationServerName": "conference",
            "trigger": {
                "conditionsMatch": "all",
                "groups": [{"conditions": [{"requestURI": "^sip:conference@.*$"}]}]
            },
            "defaultHandling": "SESSION_TERMINATED"
        }
    ]
}
```

### Sample Response

```json
{
    "result": {
        "name": "conference",
        "initialFilterCriteria": [
            {
                "priority": 0,
                "applicationServerName": "conference",
                "trigger": {
                    "conditionsMatch": "all",
                    "groups": [{"conditions": [{"requestURI": "^sip:conference@.*$"}]}]
                },
                "defaultHandling": "SESSION_TERMINATED",
                "includeRegisterRequest": false,
                "includeRegisterResponse": false
            }
        ]
    }
}
```

## Update an IMS Service Profile

This path replaces the initial filter criteria of an IMS service profile. Registered subscribers whose profile assigns it get the new user profile.

| Method | Path                                               |
| ------ | -------------------------------------------------- |
| PUT    | `/api/v1/integrations/ims/service-profiles/{name}` |

### Parameters

- `initialFilterCriteria` (array): In priority order, as in [Create an IMS Service Profile](#create-an-ims-service-profile). An empty list removes all of them.

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

| Method | Path                                               |
| ------ | -------------------------------------------------- |
| DELETE | `/api/v1/integrations/ims/service-profiles/{name}` |

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

| Method | Path                                    |
| ------ | --------------------------------------- |
| GET    | `/api/v1/integrations/roaming/partners` |

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
                    "state": "established"
                }
            }
        ]
    }
}
```

| N32 field   | Description |
| ----------- | ----------- |
| `state`     | `idle`, `establishing`, `established`, `failed`, or `unknown` (the handshake could not start). |
| `peer`      | The partner SEPP the context is with, when the handshake was redirected. |
| `lastError` | The reason of the last failure. |

## Create a Roaming Partner

This path creates a new roaming partner.

| Method | Path                                    |
| ------ | --------------------------------------- |
| POST   | `/api/v1/integrations/roaming/partners` |

### Parameters

- `name` (string): The name of the partner. Must be unique.
- `plmns` (array): The partner's PLMN IDs, at least one.
    - `mcc` (string): 3 digits.
    - `mnc` (string): 2 or 3 digits.
    - `nrfAPIRoot` (string, optional): The apiRoot (`http` or `https`) of the NRF of this PLMN, for its roamers. Defaults to `https://nrf.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org` of this PLMN ID. Needs `sepp`.
- `profileName` (string): The name of the profile applied to the partner's roamers.
- `sepp` (object, optional): The 5G agreement.
    - `fqdn` (string): The partner SEPP's FQDN, as in its certificate: `<host>.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, or `<host>.5gc.mnc<MNC>.mcc<MCC>.<IPX ID>.ipxnetwork.org`, for one of the partner's PLMN IDs.
    - `n32cURI` (string): The apiRoot of the partner SEPP's N32-c endpoint: `https`, host, optional port and optional path prefix.
    - `n32fURI` (string, optional): The apiRoot of the partner SEPP's N32-f endpoint. Defaults to `n32cURI`.
    - `trustAnchorPEM` (string): The CA certificates that the partner SEPP's certificate chains to, or the partner's self-signed certificate, in PEM. Each certificate must be valid, with an ECDSA key or an RSA key of at least 2048 bits.
- `diameterPeerID` (string, optional): The ID of the [roaming Diameter peer](#create-a-roaming-diameter-peer) of the 4G agreement.

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
| `201`  | Returns the created partner. |
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

| Method | Path                                         |
| ------ | -------------------------------------------- |
| GET    | `/api/v1/integrations/roaming/partners/{id}` |

### Parameters

None

## Update a Roaming Partner

This path updates an existing roaming partner. When a removed PLMN ID or agreement still has roamers, all the partner's roamers are first deregistered on every node, and purged at their home network.

| Method | Path                                         |
| ------ | -------------------------------------------- |
| PUT    | `/api/v1/integrations/roaming/partners/{id}` |

### Parameters

The parameters of [Create a Roaming Partner](#create-a-roaming-partner).

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | The partner is updated. |
| `400`  | Invalid parameters. |
| `404`  | The partner, the profile or the roaming Diameter peer does not exist. |
| `409`  | The name is taken, a PLMN ID conflicts, or the roamers were not all deregistered within one minute. Nothing is changed. |

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

| Method | Path                                         |
| ------ | -------------------------------------------- |
| DELETE | `/api/v1/integrations/roaming/partners/{id}` |

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

| Method | Path                                                   |
| ------ | ------------------------------------------------------ |
| POST   | `/api/v1/integrations/roaming/partners/{id}/handshake` |

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
            "state": "establishing"
        }
    }
}
```

## List Roaming Diameter Peers

This path returns the list of roaming Diameter peers: the HSS or Diameter Edge Agent of roaming partners. `status` gives the state of this node's connection to the peer.

| Method | Path                                          |
| ------ | --------------------------------------------- |
| GET    | `/api/v1/integrations/roaming/diameter-peers` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": "0199a3c2-5c1e-7b8e-9f3a-2d4c6e8f0a1b",
                "originHost": "hss.epc.mnc070.mcc999.3gppnetwork.org",
                "address": "198.51.100.5",
                "port": 3868,
                "status": {
                    "state": "open",
                    "originHost": "hss.epc.mnc070.mcc999.3gppnetwork.org",
                    "realm": "epc.mnc070.mcc999.3gppnetwork.org",
                    "since": "2026-10-05T13:44:09Z"
                }
            }
        ]
    }
}
```

## Create a Roaming Diameter Peer

This path creates a new roaming Diameter peer.

| Method | Path                                          |
| ------ | --------------------------------------------- |
| POST   | `/api/v1/integrations/roaming/diameter-peers` |

### Parameters

- `address` (string): The IPv4 or IPv6 address of the peer.
- `port` (integer, optional): The peer's SCTP port. Defaults to `3868`.
- `originHost` (string, optional): The peer's Origin-Host. When set, the peer must give this Origin-Host when it connects.

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
        "originHost": "hss.epc.mnc070.mcc999.3gppnetwork.org",
        "address": "198.51.100.5",
        "port": 3868
    }
}
```

## Get a Roaming Diameter Peer

This path returns the details of a specific roaming Diameter peer.

| Method | Path                                               |
| ------ | -------------------------------------------------- |
| GET    | `/api/v1/integrations/roaming/diameter-peers/{id}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "id": "0199a3c2-5c1e-7b8e-9f3a-2d4c6e8f0a1b",
        "originHost": "hss.epc.mnc070.mcc999.3gppnetwork.org",
        "address": "198.51.100.5",
        "port": 3868
    }
}
```

## Update a Roaming Diameter Peer

This path updates an existing roaming Diameter peer.

| Method | Path                                               |
| ------ | -------------------------------------------------- |
| PUT    | `/api/v1/integrations/roaming/diameter-peers/{id}` |

### Parameters

- `address` (string): The IPv4 or IPv6 address of the peer.
- `port` (integer): The peer's SCTP port.
- `originHost` (string, optional): The peer's Origin-Host. When set, the peer must give this Origin-Host when it connects.

### Sample Response

```json
{
    "result": {
        "message": "Roaming Diameter peer updated successfully"
    }
}
```

## Delete a Roaming Diameter Peer

This path deletes a roaming Diameter peer.

| Method | Path                                               |
| ------ | -------------------------------------------------- |
| DELETE | `/api/v1/integrations/roaming/diameter-peers/{id}` |

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

This path returns the N32 listener and SEPP ID of this node, the stored certificate of each SEPP ID, and the trust anchors to give roaming partners.

| Method | Path                                |
| ------ | ----------------------------------- |
| GET    | `/api/v1/integrations/roaming/sepp` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "listener": {
            "seppID": 1,
            "configured": true,
            "address": "192.0.2.11",
            "port": 443,
            "listening": true
        },
        "certificates": [
            {
                "seppID": 1,
                "fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
                "source": "generated",
                "subject": "CN=sepp1.5gc.mnc001.mcc001.3gppnetwork.org,O=5gc.mnc001.mcc001.3gppnetwork.org",
                "sans": ["sepp1.5gc.mnc001.mcc001.3gppnetwork.org"],
                "issuer": "CN=SEPP CA,O=5gc.mnc001.mcc001.3gppnetwork.org",
                "notBefore": "2026-10-01T08:00:00Z",
                "notAfter": "2029-10-01T08:00:00Z",
                "fingerprint": "sha256:a1b2...7e",
                "trustAnchorFingerprint": "sha256:5f0c...9a"
            }
        ],
        "trustAnchors": [
            {
                "source": "generated",
                "subject": "CN=SEPP CA,O=5gc.mnc001.mcc001.3gppnetwork.org",
                "fingerprint": "sha256:5f0c...9a",
                "notAfter": "2036-10-01T08:00:00Z",
                "pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
                "seppIDs": [1, 2]
            }
        ]
    }
}
```

`seppIDs` lists the nodes that use each trust anchor.

## Import a SEPP Certificate

This path replaces a node's SEPP certificate with one issued by your CA. When the FQDN or the trust anchor changes, the node runs the N32 handshake again with every partner.

| Method | Path                                                           |
| ------ | -------------------------------------------------------------- |
| PUT    | `/api/v1/integrations/roaming/sepp/nodes/{seppID}/certificate` |

### Parameters

- `certificateChainPEM` (string): The certificate, then any intermediates, ending with the CA certificate that partners trust. The certificate must be an end-entity certificate with keyUsage digitalSignature, extendedKeyUsage serverAuth and clientAuth (or none), and a subjectAltName DNS name under `5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org` of your PLMN ID.
- `privateKeyPEM` (string): The certificate's private key: ECDSA P-256 or P-384, or RSA of at least 2048 bits.
- `fqdn` (string, optional): The node's SEPP FQDN, when the certificate has several DNS names. Defaults to the subject CN.

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | Returns the imported certificate, as listed in [Get the SEPP Status](#get-the-sepp-status), with `warnings`: a missing extendedKeyUsage, a validity over 3 years, or a trust anchor that partners must import. |
| `400`  | Invalid certificate or key. |
| `409`  | Another node has this FQDN. |

### Sample Response

```json
{
    "result": {
        "seppID": 1,
        "fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
        "source": "imported",
        "subject": "CN=sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
        "sans": ["sepp1.5gc.mnc001.mcc001.3gppnetwork.org"],
        "issuer": "CN=Operator Root CA",
        "notBefore": "2026-10-01T08:00:00Z",
        "notAfter": "2028-10-01T08:00:00Z",
        "fingerprint": "sha256:d4e5...01",
        "trustAnchorFingerprint": "sha256:77aa...3c",
        "warnings": [
            "The trust anchor of this node changed: every roaming partner must import the new trust anchor before N32 with this node works again"
        ]
    }
}
```

## Delete a SEPP Certificate

This path deletes a node's SEPP certificate, generated or imported. The node then generates a new one signed by the cluster SEPP CA. Use it to return to a generated certificate after an import.

| Method | Path                                                           |
| ------ | -------------------------------------------------------------- |
| DELETE | `/api/v1/integrations/roaming/sepp/nodes/{seppID}/certificate` |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | The certificate was deleted. |
| `400`  | `seppID` is not between 1 and 63. |
| `404`  | The node has no SEPP certificate. |

## Regenerate the SEPP CA

This path replaces the cluster SEPP CA and deletes every generated SEPP certificate; each node then generates a new one with the new CA. Imported certificates are kept.

| Method | Path                                              |
| ------ | ------------------------------------------------- |
| POST   | `/api/v1/integrations/roaming/sepp/ca/regenerate` |

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
        "warnings": [
            "The cluster SEPP CA changed: every roaming partner must import the new trust anchor before N32 with the nodes that use generated certificates works again"
        ]
    }
}
```

## Get the Diameter Status

This path returns Ella Core's Diameter realm, the Origin-Host of every cluster node, the Diameter listener, and the state of this node's Diameter peers.

| Method | Path                            |
| ------ | ------------------------------- |
| GET    | `/api/v1/integrations/diameter` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "realm": "epc.mnc001.mcc001.3gppnetwork.org",
        "servedRealms": [
            "epc.mnc001.mcc001.3gppnetwork.org",
            "ims.mnc001.mcc001.3gppnetwork.org",
            "ims.example.org"
        ],
        "nodes": [
            {
                "nodeId": "1",
                "displayName": "core-1",
                "originHost": "mmec01.mmegi0001.mme.epc.mnc001.mcc001.3gppnetwork.org",
                "local": true
            }
        ],
        "listener": {
            "enabled": true,
            "addresses": ["192.0.2.1"],
            "port": 3868
        },
        "peers": [
            {
                "id": "smsc-0199a3c2-7a10-7c3e-8d2f-4b5c6d7e8f90",
                "role": "smsc",
                "owner": {
                    "kind": "smsc",
                    "name": "main"
                },
                "originHost": "smsc.example.org",
                "realm": "example.org",
                "address": "192.0.2.10",
                "port": 3868,
                "direction": "outbound",
                "state": "open",
                "since": "2026-09-29T13:44:09Z"
            },
            {
                "id": "scscf.ims.example.org",
                "role": "ims",
                "owner": {
                    "kind": "imsPeer",
                    "name": "cscf"
                },
                "originHost": "scscf.ims.example.org",
                "realm": "ims.example.org",
                "address": "192.0.2.20",
                "direction": "inbound",
                "state": "open",
                "since": "2026-10-05T13:44:09Z"
            }
        ]
    }
}
```

| Field | Description |
| ----- | ----------- |
| `realm` | Ella Core's Diameter realm, `epc.mnc<MNC>.mcc<MCC>.3gppnetwork.org`. |
| `servedRealms` | The realms Ella Core accepts requests for: `realm`, the derived IMS home domain, and the configured IMS home domain. |
| `peers[].role` | `smsc` for an [SMSC](#list-smscs) node, `ims` for a peer that matches an [allowed IMS peer](#list-allowed-ims-peers), `roaming` for a [roaming Diameter peer](#list-roaming-diameter-peers), or `unknown` for an inbound peer that matches no allowed IMS peer. |
| `peers[].owner` | The SMSC, allowed IMS peer, or roaming Diameter peer of the connection: `kind` is `smsc`, `imsPeer`, or `roamingDiameterPeer`; `name` is its name, or its ID for a roaming Diameter peer. |
| `peers[].direction` | `inbound` when the peer connects to Ella Core, `outbound` when Ella Core connects to the peer. Inbound peers have no `port`. |
| `peers[].error` | The reason the last connection failed, when the peer is not connected. |
