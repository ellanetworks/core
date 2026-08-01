// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#include <linux/bpf.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <linux/icmp.h>
#include <linux/in.h>
#include <linux/ip.h>
#include <linux/udp.h>
#include <stdbool.h>

#include "bpf/utils/csum.h"
#include "bpf/utils/nat_ct.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/parsers.h"

#ifndef NAT_H
#define NAT_H

/* Address translation: rewriting packets against the conntrack table in
 * nat_ct.h. The split keeps the table's ownership and lifetime rules apart
 * from the header edits they authorise. */

// An ICMP error quotes the packet that triggered it; that quote is both the
// lookup key and part of what has to be translated.
//
// The ICMP checksum edits below stay direct RFC 1624 arithmetic on both hooks,
// unlike the rest of this file: no NIC offloads the ICMPv4 checksum, so Linux
// emits ICMPv4 as CHECKSUM_NONE and the field always holds a full checksum.
// Every changed field is folded into the outer checksum — the quoted saddr,
// the quoted port, the quoted L4 checksum and the quoted IP checksum.
static __always_inline struct nat_entry *
nat_translate_icmp_quote(struct five_tuple *key, struct packet_context *ctx,
			 __u32 outer_daddr)
{
	struct iphdr *ip4;
	struct udphdr *udp;
	struct tcphdr *tcp;
	struct icmphdr *icmp;
	struct nat_entry *nat_entry = NULL;

	ip4 = detect_ip4_header(ctx);
	if (!ip4) {
		return NULL;
	}
	if (ip4->ihl < 5) {
		return NULL;
	}
	/* The quote must have been sent from the address the error is
	 * addressed to; otherwise an off-path sender can inject errors into a
	 * session. */
	if (ip4->saddr != outer_daddr) {
		return NULL;
	}

	/* A quoted fragment has payload where its L4 header would be. */
	if (ip4->frag_off & bpf_htons(IP4_FRAG_MASK)) {
		return NULL;
	}

	const void *msg_end = nat_icmp_msg_end(ctx);
	if ((const void *)(ip4 + 1) > msg_end) {
		return NULL;
	}

	key->saddr = ip4->saddr;
	key->daddr = ip4->daddr;
	__u16 previous_ip_csum = ip4->check;

	int offset = ip4->ihl * 4;
	switch (ip4->protocol) {
	case IPPROTO_UDP:
		udp = detect_udp_header(ctx, offset);
		if (!udp || (const void *)(udp + 1) > msg_end) {
			return NULL;
		}
		if (!nat_port_in_range(udp->source)) {
			return NULL;
		}
		key->proto = ip4->protocol;
		key->sport = udp->source;
		key->dport = udp->dest;
		nat_entry = bpf_map_lookup_elem(&nat_ct, key);
		if (!nat_entry || nat_entry->ue_side) {
			return NULL;
		}
		__u16 previous_udp_csum = udp->check;
		ip4->saddr = nat_entry->peer.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key->saddr, ip4->saddr);
		udp->source = nat_entry->peer.sport;
		if (udp->source != key->sport) {
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, key->sport, udp->source);
		}
		if (udp->check != 0) {
			udp->check = ipv4_csum_update_u32(
				udp->check, key->saddr, ip4->saddr);
			if (udp->source != key->sport) {
				udp->check = ipv4_csum_update_u16(
					udp->check, key->sport, udp->source);
			}
			if (udp->check == 0) {
				udp->check = 0xFFFF;
			}
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, previous_udp_csum,
				udp->check);
		}
		ip4->check = ipv4_csum_update_u32(ip4->check, key->saddr,
						  ip4->saddr);
		break;
	case IPPROTO_TCP:
		tcp = detect_tcp_ports(ctx, offset);
		if (!tcp || (const void *)((__u8 *)tcp + 8) > msg_end) {
			return NULL;
		}
		if (!nat_port_in_range(tcp->source)) {
			return NULL;
		}
		key->proto = ip4->protocol;
		key->sport = tcp->source;
		key->dport = tcp->dest;
		nat_entry = bpf_map_lookup_elem(&nat_ct, key);
		if (!nat_entry || nat_entry->ue_side) {
			return NULL;
		}
		ip4->saddr = nat_entry->peer.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key->saddr, ip4->saddr);
		tcp->source = nat_entry->peer.sport;
		if (tcp->source != key->sport) {
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, key->sport, tcp->source);
		}
		struct tcphdr *tcp_full = detect_tcp_check(ctx, offset);
		if (tcp_full &&
		    (const void *)((__u8 *)tcp_full + 18) <= msg_end) {
			__u16 previous_tcp_csum = tcp_full->check;
			tcp_full->check = ipv4_csum_update_u32(
				tcp_full->check, key->saddr, ip4->saddr);
			if (tcp->source != key->sport) {
				tcp_full->check = ipv4_csum_update_u16(
					tcp_full->check, key->sport,
					tcp->source);
			}
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, previous_tcp_csum,
				tcp_full->check);
		}
		ip4->check = ipv4_csum_update_u32(ip4->check, key->saddr,
						  ip4->saddr);
		break;
	case IPPROTO_ICMP:
		icmp = detect_icmp_header(ctx, offset);
		if (!icmp || (const void *)(icmp + 1) > msg_end) {
			return NULL;
		}
		if (!nat_icmp_is_query(icmp->type)) {
			return NULL;
		}
		key->proto = ip4->protocol;
		key->identifier = icmp->un.echo.id;
		key->type = icmp->type;
		/* Queries are stored with code 0. */
		key->code = 0;
		nat_entry = bpf_map_lookup_elem(&nat_ct, key);
		if (!nat_entry || nat_entry->ue_side) {
			return NULL;
		}
		ip4->saddr = nat_entry->peer.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key->saddr, ip4->saddr);
		if (icmp->un.echo.id != nat_entry->peer.identifier) {
			__u16 previous_icmp_csum = icmp->checksum;
			__u16 previous_id = icmp->un.echo.id;
			icmp->un.echo.id = nat_entry->peer.identifier;
			icmp->checksum = ipv4_csum_update_u16(
				icmp->checksum, previous_id, icmp->un.echo.id);
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, previous_id,
				icmp->un.echo.id);
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, previous_icmp_csum,
				icmp->checksum);
		}
		ip4->check = ipv4_csum_update_u32(ip4->check, key->saddr,
						  ip4->saddr);
		break;
	}
	ctx->icmp->checksum = ipv4_csum_update_u16(
		ctx->icmp->checksum, previous_ip_csum, ip4->check);
	return nat_entry;
}

static __always_inline struct nat_entry *
find_origin_for_icmp(struct five_tuple *key, struct packet_context *ctx,
		     __u32 outer_daddr)
{
	__u8 query = nat_icmp_query_for_reply(key->type);
	if (query) {
		key->type = query;
		return bpf_map_lookup_elem(&nat_ct, key);
	}

	switch (key->type) {
	case ICMP_DEST_UNREACH:
	case ICMP_TIME_EXCEEDED:
	case ICMP_PARAMETERPROB:
		/* The entry it resolved is returned directly: looking it up
		 * again could miss after the quote has already been rewritten. */
		return nat_translate_icmp_quote(key, ctx, outer_daddr);
	}
	return NULL;
}

static __always_inline void update_port(struct packet_context *ctx,
					__u16 new_port)
{
	/* The checksum is derived from the field before it is overwritten; the
	 * TC build corrects it with bpf_l4_csum_replace after the writes
	 * instead, so the arithmetic compiles out there. */
	switch (ctx->ip4->protocol) {
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			return;
		}

		if (!CTX_L4_CSUM_VIA_HELPERS) {
			ctx->tcp->check = ipv4_csum_update_u16(
				ctx->tcp->check, ctx->tcp->source, new_port);
		}

		ctx->tcp->source = new_port;
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			return;
		}

		if (!CTX_L4_CSUM_VIA_HELPERS && ctx->udp->check != 0) {
			ctx->udp->check = ipv4_csum_update_u16(
				ctx->udp->check, ctx->udp->source, new_port);
			/* Zero means "no checksum" in IPv4 UDP (RFC 768). */
			if (ctx->udp->check == 0) {
				ctx->udp->check = 0xFFFF;
			}
		}

		ctx->udp->source = new_port;
		break;
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			return;
		}

		if (!CTX_L4_CSUM_VIA_HELPERS) {
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, ctx->icmp->un.echo.id,
				new_port);
		}

		ctx->icmp->un.echo.id = new_port;
		break;
	}
}

/* TC variant of the source_nat checksum work: the address and port deltas are
 * withheld from the field writes and applied here through ctx_l4_csum_replace,
 * so skb->csum is carried along under CHECKSUM_COMPLETE (the IPv4 header
 * checksum needs no such care — its own delta cancels the address delta over
 * the bytes skb->csum covers). The helper calls invalidate packet pointers,
 * so the context is re-derived before returning. */
static __always_inline long
source_nat_apply_csum_helpers(struct packet_context *ctx, __u32 old_saddr,
			      __u32 new_saddr, __u16 old_port, __u16 new_port)
{
	void *data = ctx_data(ctx->ctx_buff);
	__u32 csum_off;
	__u64 flags = 0;
	bool pseudo_hdr = true;

	switch (ctx->ip4->protocol) {
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			return -1;
		}
		csum_off = (__u32)((__u8 *)&ctx->tcp->check - (__u8 *)data);
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			return -1;
		}
		csum_off = (__u32)((__u8 *)&ctx->udp->check - (__u8 *)data);
		/* Zero means "no checksum" in IPv4 UDP (RFC 768): the helper
		 * skips a zero field and mangles a zero result. */
		flags = BPF_F_MARK_MANGLED_0;
		break;
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			return -1;
		}
		csum_off = (__u32)((__u8 *)&ctx->icmp->checksum - (__u8 *)data);
		/* ICMP carries no pseudo-header, so the address delta does not
		 * enter its checksum. */
		pseudo_hdr = false;
		break;
	default:
		return -1;
	}

	/* A failed update ships a packet whose address and port were rewritten
	 * but whose L4 checksum was not. */
	long err = 0;

	if (pseudo_hdr && old_saddr != new_saddr) {
		err |= ctx_l4_csum_replace(ctx->ctx_buff, csum_off, old_saddr,
					   new_saddr,
					   4 | BPF_F_PSEUDO_HDR | flags);
	}

	if (old_port != new_port) {
		err |= ctx_l4_csum_replace(ctx->ctx_buff, csum_off, old_port,
					   new_port, 2 | flags);
	}

	context_reinit(ctx, ctx_data(ctx->ctx_buff),
		       ctx_data_end(ctx->ctx_buff));

	return err;
}
static __always_inline __u16 nat_random_port(void)
{
	__u16 min = nat_port_min;
	__u16 max = nat_port_max;

	if (max < min)
		return bpf_htons(min);

	__u32 span = (__u32)(max - min) + 1;

	return bpf_htons(min + (__u16)(bpf_get_prandom_u32() % span));
}

static __always_inline __u16 nat_next_port(__u16 port_be)
{
	__u16 port = bpf_ntohs(port_be) + 1;

	if (port < nat_port_min || port > nat_port_max)
		port = nat_port_min;

	return bpf_htons(port);
}

static __always_inline bool source_nat(struct packet_context *ctx,
				       struct bpf_fib_lookup *fib_params)
{
	__u16 proto = ctx->ip4->protocol;

	/* A fragment has no usable L4 header: the bytes at the L4 offset are
	 * payload, and rewriting them corrupts the datagram. */
	if (ctx->ip4->frag_off & bpf_htons(IP4_FRAG_MASK)) {
		set_drop_reason(ctx, UPF_DROP_NAT_FRAGMENT);
		return false;
	}

	if (!nat_ip4_lengths_valid(ctx->ctx_buff, ctx->ip4, ctx->data_end)) {
		set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
		return false;
	}

	struct five_tuple orig = {};
	orig.saddr = ctx->ip4->saddr;
	orig.daddr = ctx->ip4->daddr;
	orig.proto = proto;

	/* Incremental update: a recompute would have to cover the options a
	 * header with ihl > 5 carries. */
	ctx->ip4->saddr = fib_params->ipv4_src;
	ctx->ip4->check = ipv4_csum_update_u32(ctx->ip4->check, orig.saddr,
					       ctx->ip4->saddr);

	bool tcp_new = false;
	bool tcp_fin = false;
	bool tcp_rst = false;

	switch (proto) {
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			if (-1 == parse_tcp(ctx)) {
				set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
				return false;
			}
		}
		if (!nat_tcp_valid(ctx->ip4, ctx->tcp)) {
			set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
			return false;
		}
		orig.sport = ctx->tcp->source;
		orig.dport = ctx->tcp->dest;
		if (!CTX_L4_CSUM_VIA_HELPERS) {
			ctx->tcp->check = ipv4_csum_update_u32(
				ctx->tcp->check, orig.saddr, ctx->ip4->saddr);
		}
		/* One class per segment: an abort outranks a new connection,
		 * which outranks a close. */
		tcp_rst = ctx->tcp->rst;
		tcp_new = !tcp_rst && ctx->tcp->syn && !ctx->tcp->ack;
		tcp_fin = !tcp_rst && !tcp_new && ctx->tcp->fin;
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			if (-1 == parse_udp(ctx)) {
				set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
				return false;
			}
		}
		if (!nat_udp_valid(ctx->ip4, ctx->udp)) {
			set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
			return false;
		}
		orig.sport = ctx->udp->source;
		orig.dport = ctx->udp->dest;
		if (!CTX_L4_CSUM_VIA_HELPERS && ctx->udp->check != 0) {
			ctx->udp->check = ipv4_csum_update_u32(
				ctx->udp->check, orig.saddr, ctx->ip4->saddr);
			/* Zero means "no checksum" in IPv4 UDP (RFC 768). */
			if (ctx->udp->check == 0) {
				ctx->udp->check = 0xFFFF;
			}
		}
		break;
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			if (-1 == parse_icmp(ctx)) {
				set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
				return false;
			}
		}
		if (!nat_icmp_valid(ctx->ip4)) {
			set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
			return false;
		}
		if (nat_icmp_is_query(ctx->icmp->type)) {
			orig.identifier = ctx->icmp->un.echo.id;
			orig.type = ctx->icmp->type;
			break;
		}
		/* An error is not a flow of its own, so no entry is created. */
		return true;
	default:
		/* Translating a protocol with no port to renumber would
		 * collapse every UE onto one mapping per remote host. */
		set_drop_reason(ctx, UPF_DROP_NAT_UNSUPPORTED_PROTO);
		return false;
	}

	struct five_tuple natted = {};
	natted.saddr = fib_params->ipv4_src;
	natted.sport = nat_id_reusable(proto, orig.sport) ? orig.sport :
							    nat_random_port();
	natted.daddr = ctx->ip4->daddr;
	natted.dport = orig.dport;
	natted.proto = proto;

	__u64 now = bpf_ktime_get_ns();

	/* Refreshed in place: replacing a value frees the old node to the LRU
	 * list with no grace period, where another CPU can re-key it. A lookup
	 * already refreshes recency. */
	struct nat_entry nat_side = {};
	nat_side.peer = orig;
	nat_side.refresh_ts = now;

	struct nat_entry *tracked = bpf_map_lookup_elem(&nat_ct, &orig);
	if (tracked) {
		/* Copy before any further map access: one lookup later this
		 * pointer may address a recycled node. */
		struct five_tuple mapped = tracked->peer;
		__u8 state = NAT_READ_ONCE(tracked->state);
		__u8 closed = NAT_READ_ONCE(tracked->closed);
		__u8 replied = NAT_READ_ONCE(tracked->replied);

		if (mapped.saddr != fib_params->ipv4_src) {
			/* The reservation on the old address is only ours to
			 * release while the entry still names this flow. */
			struct nat_entry *stale =
				bpf_map_lookup_elem(&nat_ct, &mapped);
			if (stale && are_five_tuple_equal(stale->peer, orig)) {
				bpf_map_delete_elem(&nat_ct, &mapped);
			}

			bpf_map_delete_elem(&nat_ct, &orig);
			goto allocate;
		}

		/* A SYN on a flow the remote has never answered does not renew
		 * the timeout: retransmissions to a dead destination would
		 * otherwise hold the port for as long as they continue. */
		if (!tcp_new || replied) {
			NAT_WRITE_ONCE(tracked->refresh_ts, now);
		}

		if (tcp_new && state != NAT_CT_ESTABLISHED) {
			/* A SYN starts a fresh connection, except on a flow
			 * carrying traffic both ways, where a stray or
			 * duplicate SYN cannot be told from a tuple reuse and
			 * demoting would let one off-path packet reap a live
			 * connection. */
			NAT_WRITE_ONCE(tracked->state, NAT_CT_NEW);
			NAT_WRITE_ONCE(tracked->closed, 0);
			NAT_WRITE_ONCE(tracked->replied, 0);
		} else {
			if ((tcp_fin || tcp_rst) && !closed) {
				NAT_WRITE_ONCE(tracked->closed, NAT_CT_CLOSED);
			}

			if (state != NAT_CT_ESTABLISHED && replied) {
				NAT_WRITE_ONCE(tracked->state,
					       NAT_CT_ESTABLISHED);
			}
		}

		struct nat_entry *nat_cur =
			bpf_map_lookup_elem(&nat_ct, &mapped);
		if (nat_cur) {
			if (!are_five_tuple_equal(nat_cur->peer, orig)) {
				// The NAT-side tuple was reserved by another
				// flow after an eviction: the mapping is dead.
				bpf_map_delete_elem(&nat_ct, &orig);
				goto allocate;
			}
			/* The collector ages an orphaned entry from this. */
			NAT_WRITE_ONCE(nat_cur->refresh_ts, now);
		} else {
			// NOEXIST: another flow may hold this reservation.
			if (0 != bpf_map_update_elem(&nat_ct, &mapped,
						     &nat_side, BPF_NOEXIST)) {
				struct nat_entry *other =
					bpf_map_lookup_elem(&nat_ct, &mapped);
				if (!other ||
				    !are_five_tuple_equal(other->peer, orig)) {
					bpf_map_delete_elem(&nat_ct, &orig);
					goto allocate;
				}
				NAT_WRITE_ONCE(other->refresh_ts, now);
			}
		}

		natted = mapped;
		if (natted.sport != orig.sport) {
			update_port(ctx, natted.sport);
		}

		if (CTX_L4_CSUM_VIA_HELPERS &&
		    source_nat_apply_csum_helpers(ctx, orig.saddr,
						  ctx->ip4->saddr, orig.sport,
						  natted.sport) != 0) {
			set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
			return false;
		}

		return true;
	}

allocate:;

	// A successful BPF_NOEXIST insert is the atomic port reservation across
	// CPUs.
	bool reserved = false;
	if (0 ==
	    bpf_map_update_elem(&nat_ct, &natted, &nat_side, BPF_NOEXIST)) {
		reserved = true;
	} else {
		// The tuple may already belong to this flow: its UE-side entry
		// was LRU-evicted and this packet is re-creating the pair.
		struct nat_entry *existing =
			bpf_map_lookup_elem(&nat_ct, &natted);
		if (existing && are_five_tuple_equal(orig, existing->peer)) {
			existing->refresh_ts = now;
			reserved = true;
		} else {
			__u16 port = nat_random_port();
			for (int i = 0; i < NAT_PORT_RETRIES - 1; i++) {
				natted.sport = port;
				if (0 == bpf_map_update_elem(&nat_ct, &natted,
							     &nat_side,
							     BPF_NOEXIST)) {
					reserved = true;
					break;
				}
				port = nat_next_port(port);
			}
		}
	}
	if (!reserved) {
		set_drop_reason(ctx, UPF_DROP_NAT_PORT_EXHAUSTED);
		return false;
	}

	if (natted.sport != orig.sport) {
		update_port(ctx, natted.sport);
	}

	struct nat_entry ue_val = {};
	ue_val.peer = natted;
	ue_val.refresh_ts = now;
	ue_val.state = NAT_CT_NEW;
	ue_val.ue_side = 1;
	if (tcp_fin || tcp_rst)
		ue_val.closed = NAT_CT_CLOSED;

	if (0 != bpf_map_update_elem(&nat_ct, &orig, &ue_val, BPF_NOEXIST)) {
		// A concurrent packet of the same flow inserted first.
		struct nat_entry *winner = bpf_map_lookup_elem(&nat_ct, &orig);
		if (!winner) {
			bpf_map_delete_elem(&nat_ct, &natted);
			set_drop_reason(ctx, UPF_DROP_NAT_PORT_EXHAUSTED);
			return false;
		}
		struct five_tuple winner_src = winner->peer;
		if (!are_five_tuple_equal(winner_src, natted)) {
			bpf_map_delete_elem(&nat_ct, &natted);
			update_port(ctx, winner_src.sport);
			natted.sport = winner_src.sport;
		}
	}

	if (CTX_L4_CSUM_VIA_HELPERS &&
	    source_nat_apply_csum_helpers(ctx, orig.saddr, ctx->ip4->saddr,
					  orig.sport, natted.sport) != 0) {
		set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
		return false;
	}

	return true;
}

// origin is the NAT-side entry stored under nat_key; its peer tuple keys the
// UE-side entry.
static __always_inline void
nat_ct_mark_replied(const struct five_tuple *nat_key, struct nat_entry *origin)
{
	__u64 now = bpf_ktime_get_ns();

	struct five_tuple ue_key = origin->peer;
	struct nat_entry *ue = bpf_map_lookup_elem(&nat_ct, &ue_key);
	if (!ue) {
		/* Only the mapping is known here; the long class waits for an
		 * uplink packet. */
		struct nat_entry restored = {};
		restored.peer = *nat_key;
		restored.refresh_ts = now;
		restored.state = NAT_CT_NEW;
		restored.replied = 1;
		restored.ue_side = 1;
		// NOEXIST: if the uplink re-created the entry first its version
		// is the accurate one.
		bpf_map_update_elem(&nat_ct, &ue_key, &restored, BPF_NOEXIST);
		return;
	}
	NAT_WRITE_ONCE(ue->replied, 1);

	/* A closed connection is not kept alive by further traffic. */
	if (!NAT_READ_ONCE(ue->closed)) {
		NAT_WRITE_ONCE(ue->refresh_ts, now);
		NAT_WRITE_ONCE(origin->refresh_ts, now);
	}
}

/* Resolved but not applied: an earlier drop or ICMP error must see the packet
 * as the sender sent it. */
struct nat_xlate {
	__u32 daddr;
	__u16 l4_id;
	__u16 proto;
	bool valid;
	bool has_l4_id;
};

/* TC variant of destination_nat_apply: all checksum deltas go through
 * ctx_l4_csum_replace so every ip_summed state is handled (the IPv4 header
 * checksum is software-only and stays direct arithmetic). Field writes come
 * first; the helper calls invalidate packet pointers, so the caller must
 * re-derive them after. */
static __always_inline void
destination_nat_apply_csum_helpers(struct packet_context *ctx,
				   const struct nat_xlate *x)
{
	void *data = ctx_data(ctx->ctx_buff);
	const __u32 old_daddr = ctx->ip4->daddr;

	ctx->ip4->daddr = x->daddr;
	ctx->ip4->check =
		ipv4_csum_update_u32(ctx->ip4->check, old_daddr, x->daddr);

	__u32 csum_off;
	__u16 old_id;
	bool id_change;
	__u64 flags = 0;

	switch (x->proto) {
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			return;
		}
		csum_off = (__u32)((__u8 *)&ctx->tcp->check - (__u8 *)data);
		old_id = ctx->tcp->dest;
		id_change = x->has_l4_id && old_id != x->l4_id;
		if (id_change) {
			ctx->tcp->dest = x->l4_id;
		}
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			return;
		}
		csum_off = (__u32)((__u8 *)&ctx->udp->check - (__u8 *)data);
		old_id = ctx->udp->dest;
		id_change = x->has_l4_id && old_id != x->l4_id;
		if (x->has_l4_id) {
			ctx->udp->dest = x->l4_id;
		}
		/* Zero means "no checksum" in IPv4 UDP (RFC 768): the helper
		 * skips a zero field and mangles a zero result. */
		flags = BPF_F_MARK_MANGLED_0;
		break;
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			return;
		}
		csum_off = (__u32)((__u8 *)&ctx->icmp->checksum - (__u8 *)data);
		old_id = ctx->icmp->un.echo.id;
		id_change = x->has_l4_id && old_id != x->l4_id;
		if (id_change) {
			ctx->icmp->un.echo.id = x->l4_id;
		}
		break;
	default:
		return;
	}

	/* The ICMP checksum has no pseudo-header, so the address delta does
	 * not enter it. */
	long err = 0;

	if (x->proto != IPPROTO_ICMP) {
		err |= ctx_l4_csum_replace(ctx->ctx_buff, csum_off, old_daddr,
					   x->daddr,
					   4 | BPF_F_PSEUDO_HDR | flags);
	}

	if (id_change) {
		err |= ctx_l4_csum_replace(ctx->ctx_buff, csum_off, old_id,
					   x->l4_id, 2 | flags);
	}

	/* Re-derive the context in the same flow as the helper calls — every
	 * packet pointer is invalid past this point, and keying the re-parse
	 * on anything the verifier must correlate across branches leaves it a
	 * poisoned-pointer path. A failed re-parse surfaces to the caller as
	 * ctx->ip4 == NULL, and a failed checksum update takes the same exit
	 * rather than shipping a rewritten packet with a stale checksum. */
	context_reinit(ctx, ctx_data(ctx->ctx_buff),
		       ctx_data_end(ctx->ctx_buff));

	if (err) {
		ctx->ip4 = NULL;
		ctx->ip6 = NULL;
	}
}

// Rewrites the destination the lookup resolved, and nothing else.
static __always_inline void destination_nat_apply(struct packet_context *ctx,
						  const struct nat_xlate *x)
{
	if (!x->valid) {
		return;
	}

	if (CTX_L4_CSUM_VIA_HELPERS) {
		destination_nat_apply_csum_helpers(ctx, x);
		return;
	}

	const __u32 old_daddr = ctx->ip4->daddr;

	ctx->ip4->daddr = x->daddr;
	ctx->ip4->check =
		ipv4_csum_update_u32(ctx->ip4->check, old_daddr, x->daddr);

	switch (x->proto) {
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			return;
		}
		ctx->tcp->check = ipv4_csum_update_u32(ctx->tcp->check,
						       old_daddr, x->daddr);
		if (x->has_l4_id && ctx->tcp->dest != x->l4_id) {
			ctx->tcp->check = ipv4_csum_update_u16(
				ctx->tcp->check, ctx->tcp->dest, x->l4_id);
			ctx->tcp->dest = x->l4_id;
		}
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			return;
		}
		/* A datagram sent without a checksum keeps none (RFC 768). */
		if (ctx->udp->check != 0) {
			ctx->udp->check = ipv4_csum_update_u32(
				ctx->udp->check, old_daddr, x->daddr);
			if (x->has_l4_id && ctx->udp->dest != x->l4_id) {
				ctx->udp->check = ipv4_csum_update_u16(
					ctx->udp->check, ctx->udp->dest,
					x->l4_id);
			}
			if (ctx->udp->check == 0) {
				ctx->udp->check = 0xFFFF;
			}
		}
		if (x->has_l4_id) {
			ctx->udp->dest = x->l4_id;
		}
		break;
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			return;
		}
		if (x->has_l4_id && ctx->icmp->un.echo.id != x->l4_id) {
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, ctx->icmp->un.echo.id,
				x->l4_id);
			ctx->icmp->un.echo.id = x->l4_id;
		}
		break;
	}
}

/* Resolves the mapping without touching the packet, except an ICMP error's
 * quote, which is both the lookup key and part of what must be translated. */
static __always_inline bool destination_nat_lookup(struct packet_context *ctx,
						   struct nat_xlate *x,
						   bool *counted)
{
	__u16 proto = ctx->ip4->protocol;
	struct nat_entry *origin;
	struct five_tuple key = {};
	if (!nat_ip4_lengths_valid(ctx->ctx_buff, ctx->ip4, ctx->data_end)) {
		set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
		*counted = true;
		return false;
	}

	key.proto = proto;
	key.saddr = ctx->ip4->daddr;
	key.daddr = ctx->ip4->saddr;
	const __u32 outer_daddr = ctx->ip4->daddr;
	switch (proto) {
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			if (-1 == parse_icmp(ctx)) {
				set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
				*counted = true;
				return false;
			}
		}
		if (!nat_icmp_valid(ctx->ip4)) {
			set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
			*counted = true;
			return false;
		}
		key.identifier = ctx->icmp->un.echo.id;
		key.type = ctx->icmp->type;
		key.code = 0;
		origin = find_origin_for_icmp(&key, ctx, outer_daddr);
		if (!origin || origin->ue_side) {
			return false;
		}

		/* Copied before mark_replied: its map accesses may leave this
		 * pointer addressing a recycled node. */
		struct five_tuple ue_icmp = origin->peer;

		if (nat_icmp_query_for_reply(ctx->icmp->type)) {
			nat_ct_mark_replied(&key, origin);
			/* Only a reply carries an identifier; in an error the
			 * same field holds the unused word or the next-hop
			 * MTU. */
			x->l4_id = ue_icmp.identifier;
			x->has_l4_id = true;
		}
		x->daddr = ue_icmp.saddr;
		break;
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			if (-1 == parse_tcp(ctx)) {
				set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
				*counted = true;
				return false;
			}
		}
		if (!nat_port_in_range(ctx->tcp->dest)) {
			return false;
		}

		key.sport = ctx->tcp->dest;
		key.dport = ctx->tcp->source;
		origin = bpf_map_lookup_elem(&nat_ct, &key);
		if (!origin || origin->ue_side) {
			return false;
		}

		if (!nat_tcp_valid(ctx->ip4, ctx->tcp)) {
			set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
			*counted = true;
			return false;
		}

		struct five_tuple ue_tcp = origin->peer;

		nat_ct_mark_replied(&key, origin);

		x->daddr = ue_tcp.saddr;
		x->l4_id = ue_tcp.sport;
		x->has_l4_id = true;
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			if (-1 == parse_udp(ctx)) {
				set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
				*counted = true;
				return false;
			}
		}
		if (!nat_port_in_range(ctx->udp->dest)) {
			return false;
		}

		key.sport = ctx->udp->dest;
		key.dport = ctx->udp->source;
		origin = bpf_map_lookup_elem(&nat_ct, &key);
		if (!origin || origin->ue_side) {
			return false;
		}

		if (!nat_udp_valid(ctx->ip4, ctx->udp)) {
			set_drop_reason(ctx, UPF_DROP_NAT_MALFORMED);
			*counted = true;
			return false;
		}

		struct five_tuple ue_udp = origin->peer;

		nat_ct_mark_replied(&key, origin);

		x->daddr = ue_udp.saddr;
		x->l4_id = ue_udp.sport;
		x->has_l4_id = true;
		break;
	default:
		return false;
	}

	x->proto = proto;
	x->valid = true;
	return true;
}

#endif
