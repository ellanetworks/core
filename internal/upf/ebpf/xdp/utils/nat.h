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
#include <sys/cdefs.h>

#include "xdp/utils/common.h"
#include "xdp/utils/csum.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/pdr.h"

#ifndef NAT_H
#define NAT_H

#define NAT_READ_ONCE(x) (*(volatile typeof(x) *)&(x))
#define NAT_WRITE_ONCE(x, v) (*(volatile typeof(x) *)&(x) = (v))

#define PEAK_CONNECTION_PER_UE 100
#define NAT_CT_MAP_SIZE (2 * PEAK_CONNECTION_PER_UE * MAX_PDU_SESSIONS)
/* More-fragments bit and fragment offset. */
#define IP4_FRAG_MASK 0x3FFF

/* PSH, ECE and CWR carry no connection-state meaning. */
#define NAT_TCP_FLAGS_IGNORED 0xC8U
/* Bit n set means flag byte n is a combination RFC 793 permits: SYN, RST,
 * ACK, FIN|ACK, SYN|ACK, RST|ACK, SYN|URG, ACK|URG, FIN|ACK|URG. */
#define NAT_TCP_FLAGS_VALID 0x0003000400170014ULL
/* Bounded so the verifier walk of the retry loop stays within the 1M
 * instruction budget of the uplink program. */
#define NAT_PORT_RETRIES 16
#define NAT_PORT_MIN 1024

volatile const bool masquerade;
volatile const bool masquerade = false;

/* Explicit padding: the kernel compares the whole key, so no byte may be
 * left undefined. */
struct five_tuple {
	__u32 saddr;
	__u32 daddr;
	union {
		__u16 sport;
		__u16 identifier;
	};
	union {
		__u16 dport;
		struct {
			__u8 type;
			__u8 code;
		};
	};
	__u16 proto;
	__u16 pad;
};

enum nat_ct_state {
	NAT_CT_NEW = 0,
	NAT_CT_ESTABLISHED = 1,
};

/* Set only by the uplink path: an inbound FIN or RST carries no sequence
 * validation, so it must not shorten a live mapping. */
#define NAT_CT_CLOSED 0x1

static __always_inline bool nat_icmp_is_query(__u8 type)
{
	return type == ICMP_ECHO || type == ICMP_TIMESTAMP ||
	       type == ICMP_INFO_REQUEST || type == ICMP_ADDRESS;
}

static __always_inline __u8 nat_icmp_query_for_reply(__u8 type)
{
	switch (type) {
	case ICMP_ECHOREPLY:
		return ICMP_ECHO;
	case ICMP_TIMESTAMPREPLY:
		return ICMP_TIMESTAMP;
	case ICMP_INFO_REPLY:
		return ICMP_INFO_REQUEST;
	case ICMP_ADDRESSREPLY:
		return ICMP_ADDRESS;
	}

	return 0;
}

/* One entry per direction, each keyed by one tuple and holding the other in
 * peer; the UE-side entry is authoritative for state and replied. */
struct nat_entry {
	struct five_tuple peer;
	__u64 refresh_ts;
	__u8 state;
	__u8 replied;
	__u8 ue_side;
	__u8 closed;
	__u8 pad[4];
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct five_tuple);
	__type(value, struct nat_entry);
	__uint(max_entries, NAT_CT_MAP_SIZE);
} nat_ct SEC(".maps");

/* A kernel that does not track a byte swap's bounds rejects packet-pointer
 * arithmetic on tot_len, so the mask has to reach the verifier. */
static __always_inline __u32 nat_ip4_tot_len(const struct iphdr *ip4)
{
	__u32 tot_len = bpf_ntohs(ip4->tot_len);

	barrier_var(tot_len);

	return tot_len & 0xFFFF;
}

/* Trailing frame bytes past tot_len are not part of the datagram. */
static __always_inline bool nat_ip4_lengths_valid(const struct iphdr *ip4,
						  const void *data_end)
{
	__u32 tot_len = nat_ip4_tot_len(ip4);
	__u32 hdr_len = ((__u32)ip4->ihl * 4) & 0x3C;

	if (tot_len < hdr_len) {
		return false;
	}

	return (const void *)ip4 + tot_len <= data_end;
}

static __always_inline bool nat_tcp_valid(const struct iphdr *ip4,
					  const struct tcphdr *tcp)
{
	__u16 tot_len = bpf_ntohs(ip4->tot_len);
	__u16 hdr_len = ip4->ihl * 4;

	if (tcp->doff < 5 || hdr_len + tcp->doff * 4 > tot_len) {
		return false;
	}

	__u8 flags = ((const __u8 *)tcp)[13] & ~NAT_TCP_FLAGS_IGNORED;

	return (NAT_TCP_FLAGS_VALID >> flags) & 1;
}

static __always_inline bool nat_udp_valid(const struct iphdr *ip4,
					  const struct udphdr *udp)
{
	__u16 len = bpf_ntohs(udp->len);

	return len >= sizeof(*udp) &&
	       ip4->ihl * 4 + len <= bpf_ntohs(ip4->tot_len);
}

/* Bytes past the ICMP message are frame padding. */
static __always_inline const void *nat_icmp_msg_end(struct packet_context *ctx)
{
	return (const void *)ctx->ip4 + nat_ip4_tot_len(ctx->ip4);
}

static __always_inline bool are_five_tuple_equal(struct five_tuple a,
						 struct five_tuple b)
{
	return (a.saddr == b.saddr && a.daddr == b.daddr &&
		a.sport == b.sport && a.dport == b.dport && a.proto == b.proto);
}

// An ICMP error quotes the packet that triggered it; that quote is both the
// lookup key and part of what has to be translated.
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
		if (tcp_full && (const void *)((__u8 *)tcp_full + 18) <= msg_end) {
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
	__u16 old_port;
	switch (ctx->ip4->protocol) {
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			return;
		}
		old_port = ctx->tcp->source;
		ctx->tcp->source = new_port;
		ctx->tcp->check = ipv4_csum_update_u16(ctx->tcp->check,
						       old_port, new_port);
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			return;
		}
		old_port = ctx->udp->source;
		ctx->udp->source = new_port;
		if (ctx->udp->check != 0) {
			ctx->udp->check = ipv4_csum_update_u16(
				ctx->udp->check, old_port, new_port);
			/* Zero means "no checksum" in IPv4 UDP (RFC 768). */
			if (ctx->udp->check == 0) {
				ctx->udp->check = 0xFFFF;
			}
		}
		break;
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			return;
		}
		old_port = ctx->icmp->un.echo.id;
		ctx->icmp->un.echo.id = new_port;
		ctx->icmp->checksum = ipv4_csum_update_u16(ctx->icmp->checksum,
							   old_port, new_port);
		break;
	}
}
static __always_inline __u16 nat_random_port(void)
{
	return bpf_htons(NAT_PORT_MIN +
			 (__u16)(bpf_get_prandom_u32() %
				 (65536 - NAT_PORT_MIN)));
}

static __always_inline __u16 nat_next_port(__u16 port_be)
{
	__u16 port = bpf_ntohs(port_be) + 1;
	if (port < NAT_PORT_MIN)
		port = NAT_PORT_MIN;
	return bpf_htons(port);
}

static __always_inline bool source_nat(struct packet_context *ctx,
				       struct bpf_fib_lookup *fib_params)
{
	__u16 proto = ctx->ip4->protocol;

	/* A fragment has no usable L4 header: the bytes at the L4 offset are
	 * payload, and rewriting them corrupts the datagram. */
	if (ctx->ip4->frag_off & bpf_htons(IP4_FRAG_MASK)) {
		ctx->statistics->nat_fragment_drop_ip4 += 1;
		return false;
	}

	if (!nat_ip4_lengths_valid(ctx->ip4, ctx->data_end)) {
		ctx->statistics->nat_malformed_drop_ip4 += 1;
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
				ctx->statistics->nat_malformed_drop_ip4 += 1;
				return false;
			}
		}
		if (!nat_tcp_valid(ctx->ip4, ctx->tcp)) {
			ctx->statistics->nat_malformed_drop_ip4 += 1;
			return false;
		}
		orig.sport = ctx->tcp->source;
		orig.dport = ctx->tcp->dest;
		ctx->tcp->check = ipv4_csum_update_u32(
			ctx->tcp->check, orig.saddr, ctx->ip4->saddr);
		/* One class per segment: an abort outranks a new connection,
		 * which outranks a close. */
		tcp_rst = ctx->tcp->rst;
		tcp_new = !tcp_rst && ctx->tcp->syn && !ctx->tcp->ack;
		tcp_fin = !tcp_rst && !tcp_new && ctx->tcp->fin;
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			if (-1 == parse_udp(ctx)) {
				ctx->statistics->nat_malformed_drop_ip4 += 1;
				return false;
			}
		}
		if (!nat_udp_valid(ctx->ip4, ctx->udp)) {
			ctx->statistics->nat_malformed_drop_ip4 += 1;
			return false;
		}
		orig.sport = ctx->udp->source;
		orig.dport = ctx->udp->dest;
		if (ctx->udp->check != 0) {
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
				ctx->statistics->nat_malformed_drop_ip4 += 1;
				return false;
			}
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
		ctx->statistics->nat_unsupported_proto_drop_ip4 += 1;
		return false;
	}

	struct five_tuple natted = {};
	natted.saddr = fib_params->ipv4_src;
	/* Keeping a source port below the allocation range would publish a
	 * mapping on a privileged port of the UPF's own address. */
	natted.sport = bpf_ntohs(orig.sport) >= NAT_PORT_MIN ? orig.sport :
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
			/* Both halves go: the NAT-side entry holds a
			 * reservation on the old address. */
			bpf_map_delete_elem(&nat_ct, &orig);
			bpf_map_delete_elem(&nat_ct, &mapped);
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

		struct nat_entry *nat_cur = bpf_map_lookup_elem(&nat_ct, &mapped);
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
			if (0 != bpf_map_update_elem(&nat_ct, &mapped, &nat_side,
						     BPF_NOEXIST)) {
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

		return true;
	}

allocate:;

	// A successful BPF_NOEXIST insert is the atomic port reservation across
	// CPUs.
	bool reserved = false;
	if (0 == bpf_map_update_elem(&nat_ct, &natted, &nat_side,
				     BPF_NOEXIST)) {
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
		ctx->statistics->nat_port_exhausted_drop_ip4 += 1;
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
			ctx->statistics->nat_port_exhausted_drop_ip4 += 1;
			return false;
		}
		struct five_tuple winner_src = winner->peer;
		if (!are_five_tuple_equal(winner_src, natted)) {
			bpf_map_delete_elem(&nat_ct, &natted);
			update_port(ctx, winner_src.sport);
		}
	}
	return true;
}

// origin is the NAT-side entry stored under nat_key; its peer tuple keys the
// UE-side entry.
static __always_inline void nat_ct_mark_replied(const struct five_tuple *nat_key,
						struct nat_entry *origin)
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

// Rewrites the destination the lookup resolved, and nothing else.
static __always_inline void destination_nat_apply(struct packet_context *ctx,
						  const struct nat_xlate *x)
{
	if (!x->valid) {
		return;
	}

	const __u32 old_daddr = ctx->ip4->daddr;

	ctx->ip4->daddr = x->daddr;
	ctx->ip4->check = ipv4_csum_update_u32(ctx->ip4->check, old_daddr,
					       x->daddr);

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
	if (!nat_ip4_lengths_valid(ctx->ip4, ctx->data_end)) {
		ctx->statistics->nat_malformed_drop_ip4 += 1;
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
				ctx->statistics->nat_malformed_drop_ip4 += 1;
				*counted = true;
				return false;
			}
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
				ctx->statistics->nat_malformed_drop_ip4 += 1;
				*counted = true;
				return false;
			}
		}
		key.sport = ctx->tcp->dest;
		key.dport = ctx->tcp->source;
		origin = bpf_map_lookup_elem(&nat_ct, &key);
		if (!origin || origin->ue_side) {
			return false;
		}

		if (!nat_tcp_valid(ctx->ip4, ctx->tcp)) {
			ctx->statistics->nat_malformed_drop_ip4 += 1;
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
				ctx->statistics->nat_malformed_drop_ip4 += 1;
				*counted = true;
				return false;
			}
		}
		key.sport = ctx->udp->dest;
		key.dport = ctx->udp->source;
		origin = bpf_map_lookup_elem(&nat_ct, &key);
		if (!origin || origin->ue_side) {
			return false;
		}

		if (!nat_udp_valid(ctx->ip4, ctx->udp)) {
			ctx->statistics->nat_malformed_drop_ip4 += 1;
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
