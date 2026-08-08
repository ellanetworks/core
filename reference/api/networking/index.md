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
                "ipv4_pool": "172.250.0.0/24",
                "ipv6_pool": "2001:db8::/48",
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
- `ipv4_pool` (string, optional): The IPv4 pool of the data network in CIDR notation. Example: `172.250.0.0/24`. At least one of `ipv4_pool` or `ipv6_pool` must be provided.
- `ipv6_pool` (string, optional): The IPv6 pool of the data network in CIDR notation. Example: `2001:db8::/48`.
- `dns` (string): The IP address of the DNS server of the data network. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the data network. Must be an integer between 1 and 65535.

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

- `ipv4_pool` (string, optional): The IPv4 pool of the data network in CIDR notation. Example: `172.250.0.0/24`. At least one of `ipv4_pool` or `ipv6_pool` must be provided.
- `ipv6_pool` (string, optional): The IPv6 pool of the data network in CIDR notation. Example: `2001:db8::/48`.
- `dns` (string): The IP address of the DNS server of the data network. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the data network. Must be an integer between 1 and 65535.

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
        "ipv4_pool": "0.0.0.0/24",
        "ipv6_pool": "2001:db8::/48",
        "dns": "8.8.8.8",
        "mtu": 1460,
        "status": {
            "sessions": 0
        },
        "ip_allocation": {
            "pool_size": 254,
            "allocated": 0,
            "available": 254
        },
        "ipv6_allocation": {
            "pool_size": 1,
            "allocated": 0,
            "available": 1
        }
    }
}
```

## List IPv4 Allocations

This path returns a paginated list of IPv4 address allocations (leases) for a specific data network.

| Method | Path                                                       |
| ------ | ---------------------------------------------------------- |
| GET    | `/api/v1/networking/data-networks/{name}/ipv4-allocations` |

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

Each item contains the subscriber's assigned IPv4 address.

## List IPv6 Allocations

This path returns a paginated list of IPv6 address allocations (leases) for a specific data network.

| Method | Path                                                       |
| ------ | ---------------------------------------------------------- |
| GET    | `/api/v1/networking/data-networks/{name}/ipv6-allocations` |

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
                "address": "2001:db8::/64",
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

Each item contains the subscriber's assigned IPv6 /64 prefix.

## List Static IPs

This path returns the static IP reservations for a specific data network.

| Method | Path                                                 |
| ------ | ---------------------------------------------------- |
| GET    | `/api/v1/networking/data-networks/{name}/static-ips` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "items": [
            {
                "imsi": "001010100000001",
                "data_network": "internet",
                "ip_version": "ipv4",
                "address": "172.250.0.10",
                "status": "reserved",
                "session_id": null
            }
        ],
        "page": 1,
        "per_page": 1,
        "total_count": 1
    }
}
```

## Create a Static IP

This path pins an address to a subscriber on a data network. The IP version is inferred from the address family; IPv6 addresses must be /64-aligned.

| Method | Path                                                 |
| ------ | ---------------------------------------------------- |
| POST   | `/api/v1/networking/data-networks/{name}/static-ips` |

### Parameters

- `imsi` (string): The IMSI of the subscriber to pin.
- `address` (string): An IPv4 address or /64-aligned IPv6 prefix within the data network pool. Example: `172.250.0.10`.

### Sample Response

```
{
    "result": {
        "message": "Static IP created successfully"
    }
}
```

## Update a Static IP

This path repins a subscriber's reservation to a new address. A change to a live session releases it so the UE re-establishes on the new address.

| Method | Path                                                                     |
| ------ | ------------------------------------------------------------------------ |
| PUT    | `/api/v1/networking/data-networks/{name}/static-ips/{imsi}/{ip_version}` |

### Parameters

- `address` (string): The new IPv4 address or /64-aligned IPv6 prefix within the data network pool. Example: `172.250.0.20`.

### Sample Response

```
{
    "result": {
        "message": "Static IP updated successfully"
    }
}
```

## Delete a Static IP

This path removes a subscriber's static IP reservation. A change to a live session releases it so the UE re-establishes on a dynamic address.

| Method | Path                                                                     |
| ------ | ------------------------------------------------------------------------ |
| DELETE | `/api/v1/networking/data-networks/{name}/static-ips/{imsi}/{ip_version}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "message": "Static IP deleted successfully"
    }
}
```

## List Framed Routes

This path returns the framed routes for a specific data network, grouped by subscriber. A framed route is an IP prefix routed toward a subscriber's session, so a whole subnet behind the UE is reachable over one session.

| Method | Path                                                    |
| ------ | ------------------------------------------------------- |
| GET    | `/api/v1/networking/data-networks/{name}/framed-routes` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "items": [
            {
                "imsi": "001010100000001",
                "ipv4": ["192.168.60.0/24"],
                "ipv6": ["fd00:60::/64"]
            }
        ],
        "page": 1,
        "per_page": 1,
        "total_count": 1
    }
}
```

## Create Framed Routes

This path sets a subscriber's framed-route set on a data network. It is rejected if the subscriber already has framed routes here (use the update path to replace them), if NAT is enabled, if a prefix overlaps a UE pool, a route, or another subscriber's framed route, or if the subscriber's profile does not bind the data network. At most 8 prefixes per family are allowed, and each is masked to its network form.

| Method | Path                                                    |
| ------ | ------------------------------------------------------- |
| POST   | `/api/v1/networking/data-networks/{name}/framed-routes` |

### Parameters

- `imsi` (string): The IMSI of the subscriber.
- `ipv4` (array of strings): IPv4 framed-route prefixes in CIDR form. Example: `["192.168.60.0/24"]`.
- `ipv6` (array of strings): IPv6 framed-route prefixes in CIDR form. Example: `["fd00:60::/64"]`.

### Sample Response

```
{
    "result": {
        "message": "Framed routes created successfully"
    }
}
```

## Update Framed Routes

This path replaces a subscriber's entire framed-route set on a data network; an empty set clears it. The same overlap and NAT rules as create apply. A change to a live session releases it so the UE re-establishes with the new routes.

| Method | Path                                                           |
| ------ | -------------------------------------------------------------- |
| PUT    | `/api/v1/networking/data-networks/{name}/framed-routes/{imsi}` |

### Parameters

- `ipv4` (array of strings): The replacement IPv4 framed-route prefixes in CIDR form.
- `ipv6` (array of strings): The replacement IPv6 framed-route prefixes in CIDR form.

### Sample Response

```
{
    "result": {
        "message": "Framed routes updated successfully"
    }
}
```

## Delete Framed Routes

This path removes all framed routes for a subscriber on a data network.

| Method | Path                                                           |
| ------ | -------------------------------------------------------------- |
| DELETE | `/api/v1/networking/data-networks/{name}/framed-routes/{imsi}` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "message": "Framed routes deleted successfully"
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
            "addresses": ["192.168.40.6"],
            "port": 38412,
            "interface": "eth0"
        },
        "n3": {
            "name": "wlp131s0",
            "addresses": ["192.168.40.6"],
            "external_address": ""
        },
        "n6": {
            "name": "lo",
            "addresses": ["10.0.0.1"]
        },
        "api": {
            "addresses": ["192.168.1.10"],
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

- `external_address` (string): The external address to be used for the N3 / S1-U interface. This address is advertised to the radio in the GTP tunnel Transport Layer Address — to a gNB in the NGAP PDU Session Resource Setup Request (5G), or to an eNB in the S1AP E-RAB Setup (4G) — and the radio uses it to set up the GTP-U tunnel. This setting is useful when Ella Core is behind a proxy or NAT and the N3 / S1-U interface address is not reachable by the radio. If not set, Ella Core will use the address of the N3 interface as defined in the config file.

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
            },
            {
                "id": 2,
                "destination": "::/0",
                "gateway": "2001:db8::1",
                "interface": "n6",
                "metric": 200,
                "source": "bgp"
            },
            {
                "id": 3,
                "destination": "fd45::/48",
                "gateway": "2001:db8:6::2",
                "interface": "n6",
                "metric": 0,
                "source": "static"
            }
        ],
        "page": 1,
        "per_page": 25,
        "total_count": 4
    }
}
```

## Create a Route

This path creates a new route.

| Method | Path                        |
| ------ | --------------------------- |
| POST   | `/api/v1/networking/routes` |

### Parameters

- `destination` (string): The destination IP address of the route in CIDR notation. Examples: `0.0.0.0/0` (IPv4) or `::/0` (IPv6).
- `gateway` (string): The IP address of the gateway of the route. Examples: `1.2.3.4` (IPv4) or `2001:db8::1` (IPv6).
- `interface` (string): The outgoing interface of the route. Allowed values: `n3`, `n6`.
- `metric` (int): The metric of the route. Must be a non-negative integer.

### Sample Response

```
{
    "result": {
        "message": "Route created successfully",
        "id": "4"
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
        "metric": 0,
        "source": "static"
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
        "enabled": true
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
                "description": "IPv4 Loopback"
            },
            {
                "prefix": "172.250.0.0/24",
                "source": "data_network",
                "description": "UE IP pool (internet)"
            },
            {
                "prefix": "192.168.40.6/32",
                "source": "interface",
                "description": "N3 interface address"
            },
            {
                "prefix": "192.168.5.0/24",
                "source": "interface",
                "description": "N6 interface subnet"
            }
        ]
    }
}
```

The `rejectedPrefixes` array lists prefixes that are always rejected by the safety filter. These are derived from the N3 interface address, N6 interface subnets, data network IP pools, and built-in prefixes (link-local, loopback, multicast). They are read-only and cannot be configured.

## Update BGP Settings

Updates the BGP configuration. Enabling BGP starts the embedded BGP speaker. Changing the local AS or router ID triggers a restart of the speaker.

| Method | Path                     |
| ------ | ------------------------ |
| PUT    | `/api/v1/networking/bgp` |

### Parameters

- `enabled` (boolean): Enable or disable BGP.
- `localAS` (integer): The local autonomous system number.
- `routerID` (string): The BGP router ID (an IPv4 or IPv6 address).
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

- `address` (string, required): The IPv4 or IPv6 address of the peer.
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

- `address` (string, required): The IPv4 or IPv6 address of the peer.
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
