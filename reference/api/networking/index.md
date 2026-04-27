# Data Networks

## List Data Networks

This path returns the list of data networks.

| Method | Path                               |
| ------ | ---------------------------------- |
| GET    | `/api/v1/networking/data-networks` |

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
                "name": "internet",
                "ip_pool": "172.250.0.0/24",
                "dns": "8.8.8.8",
                "mtu": 1460,
                "status": {
                    "sessions": 0
                }
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Create a Data Network

This path creates a new Data Network.

| Method | Path                               |
| ------ | ---------------------------------- |
| POST   | `/api/v1/networking/data-networks` |

### Parameters

- `name` (string): The Name of the Data Network (dnn)
- `ip_pool` (string): The IP pool of the data network in CIDR notation. Example: `172.250.0.0/24`.
- `dns` (string): The IP address of the DNS server of the data network. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the data network. Must be an integer between 0 and 65535.

### Sample Response

```
{
    "result": {
        "message": "Data Network created successfully"
    }
}
```

## Update a Data Network

This path updates an existing data network.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| PUT    | `/api/v1/networking/data-networks/{name}` |

### Parameters

- `ip_pool` (string): The IP pool of the data network in CIDR notation. Example: `172.250.0.0/24`.
- `dns` (string): The IP address of the DNS server of the data network. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the data network. Must be an integer between 0 and 65535.

### Sample Response

```
{
    "result": {
        "message": "Data Network updated successfully"
    }
}
```

## Get a Data Network

This path returns the details of a specific data network.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| GET    | `/api/v1/networking/data-networks/{name}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "name": "internet",
        "ip_pool": "0.0.0.0/24",
        "dns": "8.8.8.8",
        "mtu": 1460,
        "status": {
            "sessions": 0
        },
        "ip_allocation": {
            "pool_size": 254,
            "allocated": 0,
            "available": 254
        }
    }
}
```

## List IP Allocations

This path returns a paginated list of IP address allocations (leases) for a specific data network.

| Method | Path                                                     |
| ------ | -------------------------------------------------------- |
| GET    | `/api/v1/networking/data-networks/{name}/ip-allocations` |

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
                "address": "172.250.0.1",
                "imsi": "001010100000001",
                "type": "dynamic",
                "session_id": 1
            }
        ],
        "page": 1,
        "per_page": 25,
        "total_count": 1
    }
}
```

## Delete a Data Network

This path deletes a data network from Ella Core.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| DELETE | `/api/v1/networking/data-networks/{name}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "message": "Data Network deleted successfully"
    }
}
```

# Interfaces

## Get Network Interfaces Config

This path returns the network interfaces.

| Method | Path                            |
| ------ | ------------------------------- |
| GET    | `/api/v1/networking/interfaces` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "n2": {
            "address": "192.168.40.6",
            "port": 38412
        },
        "n3": {
            "name": "wlp131s0",
            "address": "192.168.40.6",
            "external_address": ""
        },
        "n6": {
            "name": "lo"
        },
        "api": {
            "address": "",
            "port": 5002
        }
    }
}
```

## Update N3 Interface Settings

This path updates the N3 interface settings.

| Method | Path                               |
| ------ | ---------------------------------- |
| PUT    | `/api/v1/networking/interfaces/n3` |

### Parameters

- `external_address` (string): The external address to be used for the N3 interface. This address will be advertised to the gNodeB in the GTPTunnel Transport Layer Address field part of the PDU Session Setup Request message. The address will be used by the gNodeB to set the GTP tunnel. This setting is useful when Ella Core is behind a proxy or NAT and the N3 interface address is not reachable by the gNodeB. If not set, Ella Core will use the address of the N3 interface as defined in the config file.

### Sample Response

```
{
    "result": {
        "message": "N3 interface updated"
    }
}
```

# Routes

## List Routes

This path returns the list of routes, including both user-configured static routes and BGP-learned routes. Each route includes a `source` field indicating its origin (`static` or `bgp`).

| Method | Path                        |
| ------ | --------------------------- |
| GET    | `/api/v1/networking/routes` |

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
                "id": 0,
                "destination": "0.0.0.0/0",
                "gateway": "10.0.0.2",
                "interface": "n6",
                "metric": 200,
                "source": "bgp"
            },
            {
                "id": 1,
                "destination": "10.0.0.0/24",
                "gateway": "203.0.113.1",
                "interface": "n6",
                "metric": 0,
                "source": "static"
            }
        ],
        "page": 1,
        "per_page": 25,
        "total_count": 2
    }
}
```

## Create a Route

This path creates a new route.

| Method | Path                        |
| ------ | --------------------------- |
| POST   | `/api/v1/networking/routes` |

### Parameters

- `destination` (string): The destination IP address of the route in CIDR notation. Example: `0.0.0.0/0`.
- `gateway` (string): The IP address of the gateway of the route. Example: `1.2.3.4`.
- `interface` (string): The outgoing interface of the route. Allowed values: `n3`, `n6`.
- `metric` (int): The metric of the route. Must be an integer between 0 and 255.

### Sample Response

```
{
    "result": {
        "message": "Route created successfully",
        "id": 4
    }
}
```

## Get a Route

This path returns the details of a specific route.

| Method | Path                             |
| ------ | -------------------------------- |
| GET    | `/api/v1/networking/routes/{id}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "id": 4,
        "destination": "0.0.0.0/0",
        "gateway": "203.0.113.1",
        "interface": "n6",
        "metric": 0
    }
}
```

## Delete a Route

This path deletes a route from Ella Core.

| Method | Path                             |
| ------ | -------------------------------- |
| DELETE | `/api/v1/networking/routes/{id}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "message": "Route deleted successfully"
    }
}
```

# NAT

## Get NAT Info

This path returns the current NAT configuration.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/api/v1/networking/nat` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "enabled": true,
    }
}
```

## Update NAT Info

This path updates the NAT configuration.

| Method | Path                     |
| ------ | ------------------------ |
| PUT    | `/api/v1/networking/nat` |

### Parameters

- `enabled` (boolean): Enable or disable NAT.

### Sample Response

```
{
    "result": {
        "message": "NAT configuration updated successfully"
    }
}
```

# Flow Accounting

## Get Flow Accounting Info

This path returns the current flow accounting configuration.

| Method | Path                                 |
| ------ | ------------------------------------ |
| GET    | `/api/v1/networking/flow-accounting` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "enabled": true
    }
}
```

## Update Flow Accounting Info

This path updates the flow accounting configuration.

| Method | Path                                 |
| ------ | ------------------------------------ |
| PUT    | `/api/v1/networking/flow-accounting` |

### Parameters

- `enabled` (boolean): Enable or disable flow accounting.

### Sample Response

```
{
    "result": {
        "message": "Flow accounting settings updated successfully"
    }
}
```

# BGP

## Get BGP Settings

Returns the current BGP configuration.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/api/v1/networking/bgp` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "enabled": true,
        "localAS": 64512,
        "routerID": "192.168.5.10",
        "listenAddress": ":179",
        "rejectedPrefixes": [
            {
                "prefix": "127.0.0.0/8",
                "source": "builtin",
                "description": "loopback"
            },
            {
                "prefix": "172.250.0.0/24",
                "source": "data_network",
                "description": "data network: internet"
            },
            {
                "prefix": "192.168.40.0/24",
                "source": "interface",
                "description": "N3 interface subnet"
            }
        ]
    }
}
```

The `rejectedPrefixes` array lists prefixes that are always rejected by the safety filter. These are derived from N3/N6 interface subnets, data network IP pools, and built-in prefixes (link-local, loopback, multicast). They are read-only and cannot be configured.

## Update BGP Settings

Updates the BGP configuration. Enabling BGP starts the embedded BGP speaker. Changing the local AS or router ID triggers a restart of the speaker.

| Method | Path                     |
| ------ | ------------------------ |
| PUT    | `/api/v1/networking/bgp` |

### Parameters

- `enabled` (boolean): Enable or disable BGP.
- `localAS` (integer): The local autonomous system number.
- `routerID` (string): The BGP router ID (IPv4 address format).
- `listenAddress` (string): The address and port to listen on (e.g. `:179`).

### Sample Response

```
{
    "result": {
        "message": "BGP settings updated successfully"
    }
}
```

## List BGP Peers

Returns the list of configured BGP peers with live session status.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/v1/networking/bgp/peers` |

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
                "id": 1,
                "address": "192.168.5.1",
                "remoteAS": 64513,
                "holdTime": 90,
                "hasPassword": true,
                "description": "upstream router",
                "importPrefixes": [
                    {
                        "prefix": "0.0.0.0/0",
                        "maxLength": 32
                    }
                ],
                "state": "established",
                "uptime": "1h23m45s",
                "prefixesSent": 3,
                "prefixesReceived": 2,
                "prefixesAccepted": 1
            }
        ],
        "page": 1,
        "per_page": 25,
        "total_count": 1
    }
}
```

The `hasPassword` field indicates whether MD5 authentication is configured for the peer. The actual password is never returned by the API.

The `state`, `uptime`, `prefixesSent`, `prefixesReceived`, and `prefixesAccepted` fields reflect the live BGP session status. They are empty/omitted when BGP is not running. The `uptime` field is only present when the session state is `established`.

The `importPrefixes` field contains the per-peer import prefix list. When set to `[{"prefix": "0.0.0.0/0", "maxLength": 32}]`, all routes are accepted. An empty array means no routes are accepted from this peer.

## Get a BGP Peer

Returns the details of a specific BGP peer.

| Method | Path                                |
| ------ | ----------------------------------- |
| GET    | `/api/v1/networking/bgp/peers/{id}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "id": 1,
        "address": "192.168.5.1",
        "remoteAS": 64513,
        "holdTime": 90,
        "hasPassword": true,
        "description": "upstream router",
        "importPrefixes": [
            {
                "prefix": "0.0.0.0/0",
                "maxLength": 32
            }
        ],
        "state": "established",
        "uptime": "1h23m45s",
        "prefixesSent": 3,
        "prefixesReceived": 2,
        "prefixesAccepted": 1
    }
}
```

## Create a BGP Peer

Adds a new BGP peer. If BGP is running, the peer is added to the live speaker immediately.

| Method | Path                           |
| ------ | ------------------------------ |
| POST   | `/api/v1/networking/bgp/peers` |

### Parameters

- `address` (string, required): The IPv4 address of the peer.
- `remoteAS` (integer, required): The remote autonomous system number.
- `holdTime` (integer): The BGP hold timer in seconds (default `90`, minimum `3`). Keepalive is derived as holdTime / 3.
- `password` (string): MD5 authentication password. Omit or set to empty string for no authentication.
- `description` (string): An optional description for the peer.
- `importPrefixes` (array): List of prefix entries to accept from this peer. Each entry has `prefix` (CIDR string) and `maxLength` (integer). Use `[{"prefix": "0.0.0.0/0", "maxLength": 32}]` to accept all routes. Omit or set to `[]` to accept no routes.

### Sample Response

```
{
    "result": {
        "message": "BGP peer created successfully"
    }
}
```

## Update a BGP Peer

Updates an existing BGP peer. If BGP is running, the peer is reconfigured in the live speaker.

| Method | Path                                |
| ------ | ----------------------------------- |
| PUT    | `/api/v1/networking/bgp/peers/{id}` |

### Parameters

- `address` (string, required): The IPv4 address of the peer.
- `remoteAS` (integer, required): The remote autonomous system number.
- `holdTime` (integer): The BGP hold timer in seconds (default `90`, minimum `3`).
- `password` (string): MD5 authentication password.
- `description` (string): An optional description for the peer.
- `importPrefixes` (array): List of prefix entries to accept from this peer.

### Sample Response

```
{
    "result": {
        "message": "BGP peer updated successfully"
    }
}
```

## Delete a BGP Peer

Removes a BGP peer by ID. If BGP is running, the peer is removed from the live speaker immediately and any routes learned from that peer are withdrawn from the kernel.

| Method | Path                                |
| ------ | ----------------------------------- |
| DELETE | `/api/v1/networking/bgp/peers/{id}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "message": "BGP peer deleted successfully"
    }
}
```

## Get BGP Advertised Routes

Returns the routes currently advertised to BGP peers (subscriber /32 routes).

| Method | Path                                       |
| ------ | ------------------------------------------ |
| GET    | `/api/v1/networking/bgp/advertised-routes` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "routes": [
            {
                "prefix": "10.45.0.3/32",
                "nextHop": "192.168.5.10",
                "subscriber": "001010100000001"
            }
        ]
    }
}
```

Each route includes the `subscriber` IMSI that owns the IP address being advertised.

## Get BGP Learned Routes

Returns the routes learned from BGP peers that passed the safety filter and import prefix list, and are currently installed in the kernel.

| Method | Path                                    |
| ------ | --------------------------------------- |
| GET    | `/api/v1/networking/bgp/learned-routes` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "routes": [
            {
                "prefix": "10.0.0.0/24",
                "nextHop": "192.168.5.1",
                "peer": "192.168.5.1"
            }
        ]
    }
}
```
