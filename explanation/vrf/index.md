# Ella Core and VRFs

## What are VRFs?

VRF stands for Virtual Routing and Forwarding. On Linux, they are used to isolate routing and forwarding, using different routing tables. More information can be found in the Linux [documentation](https://docs.kernel.org/networking/vrf.html).

## How does Ella Core use VRFs?

Ella Core is VRF-aware. It does not itself manage VRFs, but detects their configuration on startup. When they are detected, sockets are bound to the right VRFs, and routes managed by Ella Core through the UI or BGP are set in the right routing tables.

N3 and N6 must always be in the same VRF. Ella Core will refuse to start otherwise.

## Configuring Ella Core to use VRFs

In the configuration file, use the name of the interface that is assigned to the VRF, or use a specific IP address that is assigned to an interface in the VRF.

## Routing isolation

In Linux by default, if a match is not found when looking up a route in a VRF, the kernel will continue looking up in the main routing table. To prevent this, add a high metric unreachable default route:

```
ip route add to unreachable 0.0.0.0/0 vrf vrf-name metric 9000
```
