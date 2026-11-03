---
description: RESTful API reference for roaming partners, roaming Diameter peers, the SEPP, roamers, and roaming usage.
---

# Roaming

This section describes the RESTful API for managing roaming partners, roaming Diameter peers, the SEPP, roamers, and roaming usage.

## List Roaming Partners

This path returns the list of roaming partners. For a partner with a 5G agreement, `n32` gives the N32-c context of each node with `interfaces.n32`.

| Method | Path                        |
| ------ | --------------------------- |
| GET    | `/api/v1/roaming/partners`  |

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
                "n32": [
                    {
                        "nodeID": "1",
                        "seppID": 1,
                        "state": "established",
                        "since": "2026-11-03T09:12:44Z",
                        "initiator": "local",
                        "nftlst": true
                    }
                ]
            }
        ]
    }
}
```

| N32 field    | Description |
| ------------ | ----------- |
| `state`      | `idle`, `establishing`, `established`, `failed`, `terminated`, or `unknown` (the node did not answer). |
| `initiator`  | `local` or `peer`. |
| `nftlst`     | Whether both SEPPs can tear the context down. |
| `peer`       | The partner SEPP the context is with, when the handshake was redirected. |
| `lastError`  | The reason of the last failure. |
| `otherPeers` | Contexts that other SEPPs of the partner established with the node. |

## Create a Roaming Partner

This path creates a new roaming partner.

| Method | Path                        |
| ------ | --------------------------- |
| POST   | `/api/v1/roaming/partners`  |

### Parameters

- `name` (string): The name of the partner. Must be unique.
- `plmns` (array): The partner's PLMN IDs, at least one. Two PLMN IDs conflict when they have the same MCC and their MNCs are equal once zero-padded to 3 digits, or one is a prefix of the other. A PLMN ID must not conflict with another of the partner, with your own PLMN ID, or with another partner's.
    - `mcc` (string): 3 digits.
    - `mnc` (string): 2 or 3 digits.
- `profileName` (string): The name of the profile applied to the partner's roamers.
- `sepp` (object, optional): The 5G agreement. Absent means no 5G roaming.
    - `fqdn` (string): The partner SEPP's FQDN, as in its certificate: `<host>.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, or `<host>.5gc.mnc<MNC>.mcc<MCC>.<IPX ID>.ipxnetwork.org`, for one of the partner's PLMN IDs. Must not be the well-known `sepp.5gc…` FQDN. Must be unique across partners.
    - `n32cURI` (string): The apiRoot of the partner SEPP's N32-c endpoint: `https`, host, optional port and optional path prefix.
    - `n32fURI` (string, optional): The apiRoot of the partner SEPP's N32-f endpoint. Defaults to `n32cURI`.
    - `trustAnchorPEM` (string): The CA certificates that the partner SEPP's certificate chains to, or the partner's self-signed certificate, in PEM. Each certificate must be valid, with an ECDSA key or an RSA key of at least 2048 bits.
    - `nrfAPIRoot` (string, optional): The apiRoot (`http` or `https`) of the partner's NRF. Defaults to `https://nrf.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, for the PLMN ID of `fqdn`.
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

| Method | Path                             |
| ------ | -------------------------------- |
| GET    | `/api/v1/roaming/partners/{id}`  |

### Parameters

None

## Update a Roaming Partner

This path updates an existing roaming partner. Removing a PLMN ID or an agreement first deregisters the affected roamers on every node, and purges them at their home network.

| Method | Path                             |
| ------ | -------------------------------- |
| PUT    | `/api/v1/roaming/partners/{id}`  |

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

This path deletes a roaming partner. Its roamers are first deregistered on every node, and purged at their home network. Its roaming usage is kept.

| Method | Path                             |
| ------ | -------------------------------- |
| DELETE | `/api/v1/roaming/partners/{id}`  |

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

This path starts a new N32-c handshake with the partner's SEPP on every node with `interfaces.n32`. It returns the state of each node right after the start.

| Method | Path                                       |
| ------ | ------------------------------------------ |
| POST   | `/api/v1/roaming/partners/{id}/handshake`  |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `202`  | The handshake started. |
| `404`  | The partner does not exist. |
| `409`  | The partner has no 5G agreement, or no node has `interfaces.n32`. |

### Sample Response

```json
{
    "result": {
        "n32": [
            {
                "nodeID": "1",
                "seppID": 1,
                "state": "establishing",
                "since": "2026-11-03T10:02:17Z",
                "nftlst": false
            }
        ]
    }
}
```

## List Roaming Diameter Peers

This path returns the list of roaming Diameter peers: the HSS or Diameter Edge Agent of roaming partners.

| Method | Path                              |
| ------ | --------------------------------- |
| GET    | `/api/v1/roaming/diameter-peers`  |

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

| Method | Path                              |
| ------ | --------------------------------- |
| POST   | `/api/v1/roaming/diameter-peers`  |

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

| Method | Path                                   |
| ------ | -------------------------------------- |
| GET    | `/api/v1/roaming/diameter-peers/{id}`  |

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

| Method | Path                                   |
| ------ | -------------------------------------- |
| PUT    | `/api/v1/roaming/diameter-peers/{id}`  |

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

| Method | Path                                   |
| ------ | -------------------------------------- |
| DELETE | `/api/v1/roaming/diameter-peers/{id}`  |

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

This path returns the trust anchors to give roaming partners, and the SEPP identity, certificate and N32 listener of each node.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/roaming/sepp`  |

### Parameters

None

### Sample Response

```json
{
    "result": {
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
        "nodes": [
            {
                "nodeID": "1",
                "seppID": 1,
                "fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
                "source": "generated",
                "reachable": true,
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
            }
        ]
    }
}
```

`seppIDs` lists the nodes that use each trust anchor. A node that does not answer has `reachable` `false` and an `error`.

## Get a SEPP Certificate

This path returns a node's SEPP certificate chain and trust anchor, with a warning for each departure from the TS 33.310 SEPP certificate profile.

| Method | Path                                                 |
| ------ | ---------------------------------------------------- |
| GET    | `/api/v1/roaming/sepp/nodes/{seppID}/certificate`    |

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

| Method | Path                                                 |
| ------ | ---------------------------------------------------- |
| PUT    | `/api/v1/roaming/sepp/nodes/{seppID}/certificate`    |

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

| Method | Path                                                          |
| ------ | ------------------------------------------------------------- |
| POST   | `/api/v1/roaming/sepp/nodes/{seppID}/certificate/generate`    |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `201`  | Returns the generated certificate, as in [Get a SEPP Certificate](#get-a-sepp-certificate). |
| `400`  | `seppID` is not between 1 and 63. |

## Regenerate the SEPP CA

This path replaces the cluster SEPP CA and reissues every generated SEPP certificate. Imported certificates are kept.

| Method | Path                                  |
| ------ | ------------------------------------- |
| POST   | `/api/v1/roaming/sepp/ca/regenerate`  |

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

## List Roamers

This path returns the list of roamers registered on the cluster. Roamers are not listed by the [Subscribers API](subscribers.md).

| Method | Path                        |
| ------ | --------------------------- |
| GET    | `/api/v1/roaming/roamers`   |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "imsi": "999700000000001",
                "partnerID": "0199a3c2-1f4e-7d21-8c9b-5a6e7f8091a2",
                "partnerName": "Partner Telecom",
                "homePLMN": {"mcc": "999", "mnc": "70"},
                "registrations": [
                    {
                        "rat": "5G",
                        "nodeID": "1",
                        "reachable": true,
                        "registered": true,
                        "connectionState": "connected",
                        "since": "2026-11-03T09:40:02Z",
                        "lastSeenAt": "2026-11-03T10:05:31Z",
                        "radio": "gNB-1"
                    }
                ],
                "sessions": [
                    {
                        "rat": "5G",
                        "nodeID": "1",
                        "id": 1,
                        "dataNetwork": "internet",
                        "ipv4Address": "10.45.0.7"
                    }
                ],
                "usageToday": {
                    "uplinkBytes": 1048576,
                    "downlinkBytes": 8388608,
                    "totalBytes": 9437184
                }
            }
        ],
        "unreachableNodes": [
            {"nodeID": "3", "error": "context deadline exceeded"}
        ]
    }
}
```

| Field              | Description |
| ------------------ | ----------- |
| `registrations`    | One per node and RAT (`4G` or `5G`) that holds the roamer. `connectionState` is `connected` or `idle`. |
| `sessions`         | PDU sessions (5G) and PDN connections (4G), with `ipv4Address` and `ipv6Prefix` when allocated. |
| `usageToday`       | Data usage for the current UTC day. Only for users who can read subscriber usage. |
| `unreachableNodes` | The nodes that did not answer, whose roamers are missing. Absent when every node answered. |

## Get a Roamer

This path returns the details of a specific roamer, as in [List Roamers](#list-roamers).

| Method | Path                               |
| ------ | ---------------------------------- |
| GET    | `/api/v1/roaming/roamers/{imsi}`   |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | Returns the roamer. |
| `404`  | The IMSI is not a current roamer. |

## Deregister a Roamer

This path deregisters a roamer and purges it at its home network. The UE may register again.

| Method | Path                               |
| ------ | ---------------------------------- |
| DELETE | `/api/v1/roaming/roamers/{imsi}`   |

### Parameters

None

### Responses

| Status | Condition |
| ------ | --------- |
| `200`  | The roamer is deregistered. |
| `404`  | The IMSI is not a current roamer. |
| `409`  | The roamer was not deregistered within one minute. |

### Sample Response

```json
{
    "result": {
        "message": "Roamer deregistered successfully"
    }
}
```

## Get Roaming Usage

This path returns the data usage of roamers per home PLMN ID and day. `partner_name` is the partner that holds the PLMN ID, if any. The [subscriber usage retention policy](usage.md#update-subscriber-usage-retention-policy) applies.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/api/v1/roaming/usage`  |

### Query Parameters

| Name    | In    | Type   | Default  | Allowed | Description |
| ------- | ----- | ------ | -------- | ------- | ----------- |
| `start` | query | string | `now-7d` |         | Inclusive lower bound, RFC3339 (e.g. `2026-11-01T00:00:00Z`). |
| `end`   | query | string | `now`    |         | Exclusive upper bound, RFC3339. |
| `imsi`  | query | string |          |         | Filter usage data for a specific roamer. Only for users who can read subscriber usage. |

### Sample Response

```json
{
    "result": [
        {
            "date": "2026-11-02",
            "mcc": "999",
            "mnc": "70",
            "partner_name": "Partner Telecom",
            "uplink_bytes": 52428800,
            "downlink_bytes": 314572800,
            "total_bytes": 367001600
        }
    ]
}
```
