---
description: Exchange routes through BGP
---

This guide describes the steps required to install and configure FRR to
exchange routes through BGP with other routers on your network.

## Install FRR on the Ella Core host

Install FRR with this command:

```bash
sudo apt install frr
```

## Enable the bgpd daemon

Edit the file `/etc/frr/daemons`. Change the line for bgpd to enable it:

```
bgpd=yes
```

## Configure BGPd

In this guide, we assume a simple deployment with Ella Core on one host, and
a single peer router. The following information represents the example deployment:

|           | Ella Core    | Router      |
| --------- | ------------ | ----------- |
| IP        | 192.168.5.10 | 192.168.5.1 |
| AS Number | 64513        | 64512       |

In Ella Core, we will assume that all the configured data networks will use subnets
of `10.45.0.0/16`, with minimums of `/24`. The currently configured data networks will
be:

- `10.45.0.0/24`
- `10.45.1.0/24`

Edit the file `/etc/frr/frr.conf` to the following content:

```
frr version 8.4.4
frr defaults traditional
hostname ellacore
log syslog informational
no ip forwarding
no ipv6 forwarding
service integrated-vtysh-config
!
router bgp 64513
 bgp router-id 192.168.5.10
 no bgp network import-check
 neighbor 192.168.5.1 remote-as 64512
 !
 address-family ipv4 unicast
  network 10.45.0.0/24
  network 10.45.1.0/24
  neighbor 192.168.5.1 next-hop-self
  neighbor 192.168.5.1 prefix-list UNIVERSE in
  neighbor 192.168.5.1 prefix-list ELLA out
 exit-address-family
exit
!
ip prefix-list UNIVERSE seq 5 permit any
ip prefix-list ELLA seq 5 permit 10.45.0.0/16 le 24
!
```

Restart FRR:

```bash
sudo systemctl restart frr
```

## Configure the BGP peer

The router should be configured to peer with Ella Core at this point. Follow
your router's documentation for this step.

## Validate the configuration

You can use the command `vtysh` to query the state of BGP:

```bash
sudo vtysh
```

```
ellacore# show bgp summary
IPv4 Unicast Summary (VRF default):
BGP router identifier 192.168.5.10, local AS number 64513 vrf-id 0
BGP table version 3
RIB entries 4, using 768 bytes of memory
Peers 1, using 724 KiB of memory

Neighbor          V         AS   MsgRcvd   MsgSent   TblVer  InQ OutQ  Up/Down State/PfxRcd   PfxSnt Desc
192.168.5.1       4      64512       109       110        0    0    0 01:45:13            1        2 N/A

Total number of neighbors 1

ellacore# show bgp ipv4
BGP table version is 3, local router ID is 192.168.5.10, vrf id 0
Default local pref 100, local AS 64513
Status codes:  s suppressed, d damped, h history, * valid, > best, = multipath,
               i internal, r RIB-failure, S Stale, R Removed
Nexthop codes: @NNN nexthop's vrf id, < announce-nh-self
Origin codes:  i - IGP, e - EGP, ? - incomplete
RPKI validation codes: V valid, I invalid, N Not found

   Network          Next Hop            Metric LocPrf Weight Path
*> 0.0.0.0/0        192.168.5.1               0             0 64512 i
*> 10.46.0.0/24     0.0.0.0                  0         32768 i
*> 10.46.1.0/24     0.0.0.0                  0         32768 i

Displayed  3 routes and 3 total paths
```

## Adding a new data network to BGP

Adding a new data network in Ella Core will not automatically add it to BGP at this time.
These steps describe how to add the new subnet:

```bash
sudo vtysh
```

```
configure
router bgp 64513
network 10.46.2.0/24
exit
exit
write
exit
```
