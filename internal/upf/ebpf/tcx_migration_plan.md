# UPF datapath: native XDP + TCX migration

## Goal

Attach the UPF datapath as **native XDP** where the NIC driver supports it, else
**TCX** (kernel ≥ 6.6, guaranteed in our fleet). **Drop generic XDP entirely** —
it is prototyping-only and silently corrupts `CHECKSUM_PARTIAL` skbs on
`bpf_xdp_adjust_head` (`bpf_prog_run_generic_xdp()` in `net/core/dev.c` fixes
`mac_header`/`network_header` but never `csum_start`/`ip_summed`; confirmed by
netdev maintainer J. Kicinski).

Attach preference: **native XDP → TCX**. No generic fallback.

## Approach

One C source, compiled to **two objects / two program types** behind a context
shim — the pattern Cilium uses (`bpf/ctx/xdp.h` vs `bpf/ctx/skb.h`). Native and
TCX are the only targets; native and (dropped) generic would have shared the XDP
object, so removing generic costs nothing.

| Runtime | Object | Prog type | Attach |
|---|---|---|---|
| native XDP | `n3n6_xdp.o` | `BPF_PROG_TYPE_XDP` | driver, `link.AttachXDP` DRV mode |
| TCX | `n3n6_tc.o` | `BPF_PROG_TYPE_SCHED_CLS` | `link.AttachTCX` |

## What's confirmed portable (audited)

- **No blocker anywhere.** Zero use of XDP-only primitives with no TC equivalent:
  no AF_XDP/XSKMAP, DEVMAP, CPUMAP, `data_meta`, multi-buffer frags, or
  `XDP_REDIRECT`-to-map.
- **Maps** are all HASH/PERCPU/ARRAY/LRU/LPM/PROG_ARRAY/RINGBUF — valid in TC.
- **Tail-call array** (`upf_calls`) stays homogeneous: all stage progs share one
  `SEC` prefix, so the TC build is uniformly `SCHED_CLS`.
- **`ingress_ifindex`** exists on `__sk_buff`; `bpf_fib_lookup`, `bpf_check_mtu`,
  `bpf_redirect`, ringbuf all work in TC unchanged.

## The one real risk

Outer **UDPv6 checksum** (`udpv6_csum`, `utils/csum.h:247`) computed from scratch
over inner bytes. On the TC egress path an skb can be `CHECKSUM_PARTIAL` or a GSO
super-frame → the from-scratch sum is wrong. This failure mode **cannot occur in
XDP** (pre-stack). Native-XDP nodes never hit it; only the TCX path does.

Mitigation: let the stack own the tunnel checksum via
`bpf_skb_adjust_room(..., BPF_F_ADJ_ROOM_ENCAP_L4_UDP | BPF_F_ADJ_ROOM_ENCAP_L3_IPV6 |
BPF_F_ADJ_ROOM_ENCAP_L4_CSUM)`, or do encap on ingress. **Spike this before
committing to the port.**

## Work breakdown

1. **ctx shim** — introduce `__ctx_buff` typedef + accessors in
   `utils/packet_context.h` (today hard-codes `struct xdp_md *xdp_ctx`, line 48).
   Primitives: `ctx_data/ctx_data_end`, `ctx_adjust_head/tail`,
   `ctx_redirect_out(ifindex)`, `ctx_ingress_ifindex`, verdict codes. XDP build
   expands to today's `bpf_xdp_*`; **no behavior change** — lands risk-free.
2. **Route datapath through the shim** — mechanical rewrite of `gtp.h`,
   `routing.h`, `frag_needed.h`, `nat.h`, `parsers.h`, `csum.h` to call `ctx_*`
   instead of `bpf_xdp_*` / `XDP_*`. Internal neutral enum for
   `statistics->xdp_actions[]`.
3. **TC build target** — add `-DCTX_TYPE_TC` build + `SEC("tc")` variant + skb
   expansions: `bpf_skb_adjust_room`/`change_tail`, `bpf_skb_load_bytes`,
   `bpf_skb_pull_data` before direct parses (non-linear skbs), `XDP_TX` →
   `bpf_redirect(ingress_ifindex, 0)`. Second `go:generate` line in `objects.go`
   (:32) and `veth_objects.go` (:8).
4. **Checksum spike** — resolve the UDPv6/GSO risk above; test under GRO/offload.
5. **Loader** — in `upf.go`: try `AttachXDP` DRV mode → on failure `AttachTCX`.
   Delete `XDPGenericMode` from `StringToXDPAttachMode` (:357). Preserve atomic
   reload (`link.Update`). Same for `ra_responder.go` (:118) and `veth_bpf.c`.
6. **Remove generic** — drop the generic attach path and any `xdpgeneric` config.

## Testing

- Per-mode functional pass (native + TCX): GTP-U encap/decap, echo, error
  indication, frag-needed/PTB, NAT, N3↔N6 routing.
- **TCX correctness under offload**: GRO/GSO and `CHECKSUM_PARTIAL` traffic
  through encap/decap — the risk area. Verify outer checksums on the wire.
- Capability probe: node without native-XDP NIC lands on TCX; node with it lands
  on native.

## Rollback

Native XDP object is unchanged from today's proven datapath. If TCX shows issues,
native-capable nodes are unaffected; TCX-only nodes can pin to the prior release.

## References

- netdev maintainer email (generic XDP `CHECKSUM_PARTIAL` corruption; "use TCX").
- `net/core/dev.c:bpf_prog_run_generic_xdp` — no `csum_start` reconciliation.
- Kernel `Fixes:` lineage: `065af3554705` (2019 `network_header`),
  `020f0c8b3d39` (2025 enetc geometry).
</content>
</invoke>
