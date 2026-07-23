# Use native XDP with veth interfaces

When Ella Core's N3 or N6 interface is a veth pair and XDP is set to `native` mode, you must attach a minimal XDP program to the peer side of the veth. Without it, downlink traffic will silently fail.

For an explanation of why this is needed, see [XDP redirect on veth pairs](https://docs.ellanetworks.com/explanation/user_plane_packet_processing_with_ebpf/#xdp-redirect-on-veth-pairs).

## 1. Install prerequisites

Install the BPF toolchain needed to compile the XDP program:

```
sudo apt update
sudo apt install -y clang llvm libbpf-dev linux-headers-$(uname -r)
```

## 2. Create the zero entrypoint program

Create a file called `zero_entrypoint.c` with the following content:

```
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

SEC("xdp")
int zero_entrypoint(struct xdp_md *ctx)
{
    return XDP_PASS;
}

char _license[] SEC("license") = "GPL";
```

## 3. Compile the program

```
clang -O2 -g -Wall -target bpf -c zero_entrypoint.c -o zero_entrypoint.o
```

## 4. Attach the program to the peer veth

Attach the compiled program to the peer veth interface. If the peer is in a network namespace (e.g. `n3ns`), use `ip netns exec`:

```
sudo ip netns exec n3ns ip link set dev n3-ran-veth xdpgeneric off
sudo ip netns exec n3ns ip link set dev n3-ran-veth xdp obj zero_entrypoint.o sec xdp
```

If the peer is not in a namespace:

```
sudo ip link set dev <peer-veth-name> xdp obj zero_entrypoint.o sec xdp
```

## 5. Verify

Confirm the program is attached:

```
sudo ip netns exec n3ns ip link show n3-ran-veth
```

You should see `prog/xdp` in the output.

## 6. Configure Ella Core for native XDP

Set the XDP attach mode to `native` in the Ella Core configuration file:

```
xdp:
  attach-mode: "native"
```

Restart Ella Core for the change to take effect.
