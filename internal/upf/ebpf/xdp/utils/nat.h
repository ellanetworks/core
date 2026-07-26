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

#include "xdp/utils/csum.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/pdr.h"

#ifndef NAT_H
#define NAT_H

/* One machine access, so a field another CPU may be writing is neither
 * re-read nor split by the compiler. */
#define NAT_READ_ONCE(x) (*(volatile typeof(x) *)&(x))
#define NAT_WRITE_ONCE(x, v) (*(volatile typeof(x) *)&(x) = (v))

#define PEAK_CONNECTION_PER_UE 100
/* Two entries per connection, one per direction. */
#define NAT_CT_MAP_SIZE (2 * PEAK_CONNECTION_PER_UE * MAX_PDU_SESSIONS)
/* Fragment offset and more-fragments bits of frag_off, host order. */
#define IP4_FRAG_MASK 0x3FFF

/* PSH, ECE and CWR carry no state meaning, so they are masked off before the
 * combination is checked. */
#define NAT_TCP_FLAGS_IGNORED 0xC8U
/* Bit n set means flag byte n is a combination RFC 793 permits: SYN, RST,
 * ACK, FIN|ACK, SYN|ACK, RST|ACK, SYN|URG, ACK|URG, FIN|ACK|URG. Anything
 * else (SYN|FIN, SYN|RST, FIN|RST, no flags at all) is a probe or a forgery
 * that different stacks interpret differently. */
#define NAT_TCP_FLAGS_VALID 0x0003000400170014ULL
/* Bounded so the verifier walk of the retry loop stays within the 1M
 * instruction budget of the uplink program. */
#define NAT_PORT_RETRIES 16
#define NAT_PORT_MIN 1024

volatile const bool masquerade;
volatile const bool masquerade = false;

/* pad is named and zero-initialised with the rest: the kernel hashes and
 * compares the whole key, including bytes the compiler would otherwise be
 * free to leave undefined. */
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

/* Closing is tracked per direction: the short closed timeout applies only
 * once both sides have closed, so a half-close followed by an idle gap does
 * not reap a connection the server is still answering. */
#define NAT_CT_CLOSED_UE 0x1
#define NAT_CT_CLOSED_REMOTE 0x2

/* Which half of the handshake has been observed. Without sequence numbers
 * this is the evidence available for deciding whether a flow is real: a
 * connection is only established once the UE's SYN was answered by a SYN|ACK
 * and the UE acknowledged it. */
#define NAT_CT_HS_SYN 0x1
#define NAT_CT_HS_SYNACK 0x2

/* The ICMP messages RFC 5508 §3.1 counts as Queries, which REQ-1 requires a
 * NAT to permit from the private side. Each carries an identifier where echo
 * carries one, so all are tracked the same way. */
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

/* Each connection has two entries, written only by the uplink path: one
 * keyed by the UE tuple, one by the translated tuple, each holding the other
 * tuple in src. The UE-side entry is authoritative for state and replied.
 * The uplink path owns both; the downlink path only restores a UE-side entry
 * the LRU evicted. */
struct nat_entry {
	struct five_tuple src;
	__u64 refresh_ts;
	__u8 state;
	__u8 replied;
	__u8 ue_side;
	__u8 closed;
	__u8 handshake;
	__u8 pad[3];
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct five_tuple);
	__type(value, struct nat_entry);
	__uint(max_entries, NAT_CT_MAP_SIZE);
} nat_ct SEC(".maps");

/* The L4 header the parsers found must lie inside the IP payload the header
 * itself declares, not merely inside the frame: trailing bytes after a short
 * datagram are not part of it. */
static __always_inline bool nat_ip4_lengths_valid(const struct iphdr *ip4,
						  const void *data_end)
{
	__u16 tot_len = bpf_ntohs(ip4->tot_len);
	__u16 hdr_len = ip4->ihl * 4;

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

/* Bytes past the ICMP message are frame padding, not part of the error the
 * sender quoted, so the rewrite must stay inside the length the outer header
 * declares. */
static __always_inline const void *nat_icmp_msg_end(struct packet_context *ctx)
{
	return (const void *)ctx->ip4 + bpf_ntohs(ctx->ip4->tot_len);
}

static __always_inline bool are_five_tuple_equal(struct five_tuple a,
						 struct five_tuple b)
{
	return (a.saddr == b.saddr && a.daddr == b.daddr &&
		a.sport == b.sport && a.dport == b.dport && a.proto == b.proto);
}

// Parses and update the referenced packet in an ICMP message
// ICMP error messages contain the start of the packet that caused
// the error, so that the sender can match it to a specific flow.
// For NAT, it is required to look at that reference packet to NAT
// the ICMP packet back to the right source. It is also required to
// NAT the referenced packet inside, so the original sender can match
// it.
static __always_inline struct nat_entry *
parse_icmp_packet_ref(struct five_tuple *key, struct packet_context *ctx,
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
	/* The quoted packet is the one this UPF sent, so both of its addresses
	 * key the NAT-side entry. The error's outer source is the reporting
	 * router, which is any hop on the path, but its destination must be
	 * the address the quote was sent from: without that an off-path sender
	 * that guesses a mapping can inject an error into a PDU session. */
	if (ip4->saddr != outer_daddr) {
		return NULL;
	}

	/* A quoted fragment has payload where its L4 header would be. */
	if (ip4->frag_off & bpf_htons(IP4_FRAG_MASK)) {
		return NULL;
	}

	key->saddr = ip4->saddr;
	key->daddr = ip4->daddr;
	__u16 previous_ip_csum = ip4->check;

	int offset = ip4->ihl * 4;
	switch (ip4->protocol) {
	case IPPROTO_UDP:
		udp = detect_udp_header(ctx, offset);
		if (!udp) {
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
		ip4->saddr = nat_entry->src.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key->saddr, ip4->saddr);
		udp->source = nat_entry->src.sport;
		if (udp->source != key->sport) {
			/* The quoted port is covered by the outer checksum. */
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
		if (!tcp) {
			return NULL;
		}
		key->proto = ip4->protocol;
		key->sport = tcp->source;
		key->dport = tcp->dest;
		nat_entry = bpf_map_lookup_elem(&nat_ct, key);
		if (!nat_entry || nat_entry->ue_side) {
			return NULL;
		}
		ip4->saddr = nat_entry->src.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key->saddr, ip4->saddr);
		tcp->source = nat_entry->src.sport;
		if (tcp->source != key->sport) {
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, key->sport, tcp->source);
		}
		/* The quoted TCP checksum is past the 8 guaranteed octets. */
		struct tcphdr *tcp_full = detect_tcp_check(ctx, offset);
		if (tcp_full) {
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
		if (!icmp) {
			return NULL;
		}
		if (!nat_icmp_is_query(icmp->type)) {
			return NULL;
		}
		key->proto = ip4->protocol;
		key->identifier = icmp->un.echo.id;
		key->type = icmp->type;
		/* Queries are stored with code 0; keying on the observed code
		 * would look up an entry that was never written. */
		key->code = 0;
		nat_entry = bpf_map_lookup_elem(&nat_ct, key);
		if (!nat_entry || nat_entry->ue_side) {
			return NULL;
		}
		ip4->saddr = nat_entry->src.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key->saddr, ip4->saddr);
		if (icmp->un.echo.id != nat_entry->src.identifier) {
			__u16 previous_icmp_csum = icmp->checksum;
			__u16 previous_id = icmp->un.echo.id;
			icmp->un.echo.id = nat_entry->src.identifier;
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

/* RFC 5508 REQ-5: an ICMP error from a UE quotes the packet that triggered
 * it, addressed to the UE. The remote matches the error against that quoted
 * header, so it has to carry the translated form. The session is looked up,
 * never refreshed (REQ-6). */
static __always_inline bool nat_icmp_error_uplink(struct packet_context *ctx,
						  __u32 ue_addr, __u32 peer_addr)
{
	struct iphdr *ip4 = detect_ip4_header(ctx);
	if (!ip4 || ip4->ihl < 5) {
		return false;
	}

	/* The quote must be a packet addressed to this UE and sent by the host
	 * the error is going to. Without both checks a UE could have the UPF
	 * translate and emit an error for another subscriber's session,
	 * disclosing its mapping and letting a third party tear it down. */
	if (ip4->daddr != ue_addr || ip4->saddr != peer_addr) {
		return false;
	}

	/* A quoted fragment has payload where its L4 header would be. */
	if (ip4->frag_off & bpf_htons(IP4_FRAG_MASK)) {
		return false;
	}

	const void *msg_end = nat_icmp_msg_end(ctx);
	if ((const void *)(ip4 + 1) > msg_end) {
		return false;
	}

	struct five_tuple key = {};
	struct nat_entry *entry;
	struct udphdr *udp;
	struct tcphdr *tcp;

	key.proto = ip4->protocol;
	key.saddr = ip4->daddr;
	key.daddr = ip4->saddr;

	int offset = ip4->ihl * 4;
	__u16 previous_ip_csum = ip4->check;

	switch (ip4->protocol) {
	case IPPROTO_UDP:
		udp = detect_udp_header(ctx, offset);
		if (!udp || (const void *)(udp + 1) > msg_end) {
			return false;
		}
		key.sport = udp->dest;
		key.dport = udp->source;
		entry = bpf_map_lookup_elem(&nat_ct, &key);
		/* A NAT-side entry here would write a private address into a
		 * quote leaving on N6. */
		if (!entry || !entry->ue_side) {
			return false;
		}
		__u16 previous_udp_csum = udp->check;
		ip4->daddr = entry->src.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key.saddr, ip4->daddr);
		udp->dest = entry->src.sport;
		if (udp->dest != key.sport) {
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, key.sport, udp->dest);
		}
		if (udp->check != 0) {
			udp->check = ipv4_csum_update_u32(udp->check, key.saddr,
							  ip4->daddr);
			if (udp->dest != key.sport) {
				udp->check = ipv4_csum_update_u16(
					udp->check, key.sport, udp->dest);
			}
			if (udp->check == 0) {
				udp->check = 0xFFFF;
			}
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, previous_udp_csum,
				udp->check);
		}
		break;
	case IPPROTO_TCP:
		tcp = detect_tcp_ports(ctx, offset);
		if (!tcp || (const void *)((__u8 *)tcp + 8) > msg_end) {
			return false;
		}
		key.sport = tcp->dest;
		key.dport = tcp->source;
		entry = bpf_map_lookup_elem(&nat_ct, &key);
		if (!entry || !entry->ue_side) {
			return false;
		}
		ip4->daddr = entry->src.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key.saddr, ip4->daddr);
		tcp->dest = entry->src.sport;
		if (tcp->dest != key.sport) {
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, key.sport, tcp->dest);
		}
		struct tcphdr *tcp_full = detect_tcp_check(ctx, offset);
		if (tcp_full && (const void *)((__u8 *)tcp_full + 18) <= msg_end) {
			__u16 previous_tcp_csum = tcp_full->check;
			tcp_full->check = ipv4_csum_update_u32(
				tcp_full->check, key.saddr, ip4->daddr);
			if (tcp->dest != key.sport) {
				tcp_full->check = ipv4_csum_update_u16(
					tcp_full->check, key.sport, tcp->dest);
			}
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, previous_tcp_csum,
				tcp_full->check);
		}
		break;
	default:
		return false;
	}

	ip4->check = ipv4_csum_update_u32(ip4->check, key.saddr, ip4->daddr);
	ctx->icmp->checksum = ipv4_csum_update_u16(ctx->icmp->checksum,
						   previous_ip_csum, ip4->check);
	return true;
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
		if (!parse_icmp_packet_ref(key, ctx, outer_daddr))
			return NULL;
		return bpf_map_lookup_elem(&nat_ct, key);
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
		/* One class per segment, in the order netfilter uses: an abort
		 * outranks a new connection, which outranks a close. */
		tcp_rst = ctx->tcp->rst;
		tcp_new = !tcp_rst && ctx->tcp->syn && !ctx->tcp->ack;
		tcp_fin = !tcp_rst && !tcp_new && ctx->tcp->fin;
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			if (-1 == parse_udp(ctx)) {
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
				return false;
			}
		}
		if (nat_icmp_is_query(ctx->icmp->type)) {
			orig.identifier = ctx->icmp->un.echo.id;
			orig.type = ctx->icmp->type;
			break;
		}
		switch (ctx->icmp->type) {
		case ICMP_DEST_UNREACH:
		case ICMP_TIME_EXCEEDED:
		case ICMP_PARAMETERPROB:
			/* An error is not a flow of its own: no conntrack
			 * entry, and it is dropped when the packet it quotes
			 * has no mapping (RFC 5508 REQ-5). */
			if (!nat_icmp_error_uplink(ctx, orig.saddr,
						   orig.daddr)) {
				ctx->statistics->nat_icmp_untranslatable_drop_ip4 += 1;
				return false;
			}
			return true;
		default:
			ctx->statistics->nat_unsupported_proto_drop_ip4 += 1;
			return false;
		}
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

	/* A tracked flow is refreshed in place. Replacing a value hands the
	 * old node straight back to the LRU free list with no grace period,
	 * where another CPU can re-key it while this one still holds a
	 * pointer into it; the lookup itself already refreshes LRU recency,
	 * so a write buys nothing. Map writes below happen only where a
	 * lookup proved an entry missing or stale. */
	struct nat_entry nat_side = {};
	nat_side.src = orig;
	nat_side.refresh_ts = now;

	struct nat_entry *tracked = bpf_map_lookup_elem(&nat_ct, &orig);
	if (tracked) {
		/* Copy before any further map access: after one, this pointer
		 * may address a different flow's entry. */
		struct five_tuple mapped = tracked->src;
		__u8 state = NAT_READ_ONCE(tracked->state);
		__u8 closed = NAT_READ_ONCE(tracked->closed);
		__u8 replied = NAT_READ_ONCE(tracked->replied);

		struct nat_entry *nat_cur = bpf_map_lookup_elem(&nat_ct, &mapped);
		if (nat_cur) {
			if (!are_five_tuple_equal(nat_cur->src, orig)) {
				// The NAT-side tuple was reserved by another
				// flow after an eviction: the mapping is dead.
				bpf_map_delete_elem(&nat_ct, &orig);
				goto allocate;
			}
			/* The garbage collector ages an orphaned NAT-side
			 * entry from this timestamp. */
			NAT_WRITE_ONCE(nat_cur->refresh_ts, now);
		} else {
			// Restore the partner the LRU evicted, without
			// clobbering a reservation another flow may hold.
			if (0 != bpf_map_update_elem(&nat_ct, &mapped, &nat_side,
						     BPF_NOEXIST)) {
				struct nat_entry *other =
					bpf_map_lookup_elem(&nat_ct, &mapped);
				if (!other ||
				    !are_five_tuple_equal(other->src, orig)) {
					bpf_map_delete_elem(&nat_ct, &orig);
					goto allocate;
				}
			}
		}

		natted = mapped;
		if (natted.sport != orig.sport) {
			update_port(ctx, natted.sport);
		}

		NAT_WRITE_ONCE(tracked->refresh_ts, now);

		if (tcp_new) {
			// A new SYN reuses the tuple for a fresh connection,
			// which the remote has not answered yet.
			NAT_WRITE_ONCE(tracked->state, NAT_CT_NEW);
			NAT_WRITE_ONCE(tracked->closed, 0);
			NAT_WRITE_ONCE(tracked->replied, 0);
			NAT_WRITE_ONCE(tracked->handshake, NAT_CT_HS_SYN);
		} else {
			__u8 set = 0;
			if (tcp_fin)
				set |= NAT_CT_CLOSED_UE;
			// A UE-sourced RST aborts both directions: the source
			// address was checked against the session's UE address,
			// so it is the subscriber closing its own connection.
			// An inbound one is not trusted to (see
			// destination_nat).
			if (tcp_rst)
				set |= NAT_CT_CLOSED_UE | NAT_CT_CLOSED_REMOTE;
			if (set & ~closed) {
				NAT_WRITE_ONCE(tracked->closed, closed | set);
			}
			/* The acknowledgement completing the handshake. A
			 * flow picked up mid-stream has no SYN|ACK to observe
			 * and stays in the transitory class. */
			if (state != NAT_CT_ESTABLISHED && replied &&
			    proto == IPPROTO_TCP && ctx->tcp->ack &&
			    (NAT_READ_ONCE(tracked->handshake) &
			     NAT_CT_HS_SYNACK)) {
				NAT_WRITE_ONCE(tracked->state,
					       NAT_CT_ESTABLISHED);
			} else if (state != NAT_CT_ESTABLISHED &&
				   replied && proto != IPPROTO_TCP) {
				NAT_WRITE_ONCE(tracked->state,
					       NAT_CT_ESTABLISHED);
			}
		}
		return true;
	}

allocate:;

	// A successful BPF_NOEXIST insert of the NAT-side entry is the atomic
	// port reservation across CPUs. The loop body is a single insert so
	// the verifier walk stays cheap.
	bool reserved = false;
	if (0 == bpf_map_update_elem(&nat_ct, &natted, &nat_side,
				     BPF_NOEXIST)) {
		reserved = true;
	} else {
		// The tuple may already belong to this flow: its UE-side entry
		// was LRU-evicted and this packet is re-creating the pair.
		struct nat_entry *existing =
			bpf_map_lookup_elem(&nat_ct, &natted);
		if (existing && are_five_tuple_equal(orig, existing->src)) {
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
	ue_val.src = natted;
	ue_val.refresh_ts = now;
	ue_val.state = NAT_CT_NEW;
	ue_val.ue_side = 1;
	if (tcp_new)
		ue_val.handshake = NAT_CT_HS_SYN;
	if (tcp_fin)
		ue_val.closed = NAT_CT_CLOSED_UE;
	if (tcp_rst)
		ue_val.closed = NAT_CT_CLOSED_UE | NAT_CT_CLOSED_REMOTE;

	if (0 != bpf_map_update_elem(&nat_ct, &orig, &ue_val, BPF_NOEXIST)) {
		// A concurrent packet of the same flow inserted first: adopt
		// its mapping and release the reservation made above.
		struct nat_entry *winner = bpf_map_lookup_elem(&nat_ct, &orig);
		if (!winner) {
			bpf_map_delete_elem(&nat_ct, &natted);
			return false;
		}
		if (!are_five_tuple_equal(winner->src, natted)) {
			bpf_map_delete_elem(&nat_ct, &natted);
			update_port(ctx, winner->src.sport);
		}
	}
	return true;
}

// origin is the NAT-side entry stored under nat_key; its src tuple keys the
// authoritative UE-side entry. An inbound FIN or RST closes the remote
// direction only: it carries no sequence validation, so it must not by
// itself shorten the connection to the closed timeout.
static __always_inline void nat_ct_mark_replied(const struct five_tuple *nat_key,
						struct nat_entry *origin,
						bool closing, bool synack,
						bool abort_flow)
{
	/* Inbound traffic alone keeps a mapping alive (RFC 4787 REQ-6a): a
	 * remote can only reach one whose full tuple it already knows. */
	__u64 now = bpf_ktime_get_ns();
	NAT_WRITE_ONCE(origin->refresh_ts, now);

	struct five_tuple ue_key = origin->src;
	struct nat_entry *ue = bpf_map_lookup_elem(&nat_ct, &ue_key);
	if (!ue) {
		// The UE-side entry was evicted: restore the pair so a
		// download with no uplink traffic keeps its mapping.
		/* Only the mapping is known here. Promoting the restored entry
		 * would hand an unestablished, or an already closed, connection
		 * the established lifetime. */
		struct nat_entry restored = {};
		restored.src = *nat_key;
		restored.refresh_ts = now;
		restored.state = NAT_CT_NEW;
		restored.ue_side = 1;
		if (closing)
			restored.closed = NAT_CT_CLOSED_REMOTE;
		// NOEXIST: if the uplink re-created the entry first its version
		// is the accurate one.
		bpf_map_update_elem(&nat_ct, &ue_key, &restored, BPF_NOEXIST);
		return;
	}
	NAT_WRITE_ONCE(ue->replied, 1);
	NAT_WRITE_ONCE(ue->refresh_ts, now);
	if (synack && (NAT_READ_ONCE(ue->handshake) & NAT_CT_HS_SYN)) {
		NAT_WRITE_ONCE(ue->handshake,
			       NAT_CT_HS_SYN | NAT_CT_HS_SYNACK);
	}
	/* A reset for a flow with no observed handshake is evidence the
	 * connection never existed, so both directions close; one that did
	 * complete a handshake has too much to lose to an unvalidated
	 * segment, and only the remote direction closes. */
	if (abort_flow && NAT_READ_ONCE(ue->state) != NAT_CT_ESTABLISHED) {
		NAT_WRITE_ONCE(ue->closed,
			       NAT_CT_CLOSED_UE | NAT_CT_CLOSED_REMOTE);
		return;
	}
	/* Like the reset above, a segment that closes a direction is trusted in
	 * proportion to the evidence the connection is real. */
	if ((closing || abort_flow) &&
	    NAT_READ_ONCE(ue->state) == NAT_CT_ESTABLISHED) {
		/* A byte has no atomic OR in BPF, so a simultaneous uplink
		 * write can drop this bit; the cost is a connection that keeps
		 * its longer class, never a shorter one. */
		NAT_WRITE_ONCE(ue->closed,
			       NAT_READ_ONCE(ue->closed) | NAT_CT_CLOSED_REMOTE);
	}
}

/* Fields destination_nat rewrote, so an ICMP error generated further down the
 * downlink path quotes the packet as the sender sent it. */
struct nat_undo {
	__u32 daddr;
	__u16 dport;
	__u16 l4_check;
	__u16 proto;
	bool valid;
	bool has_l4;
};

// Restores the pre-translation destination and L4 header.
static __always_inline void nat_undo_apply(struct packet_context *ctx,
					   const struct nat_undo *undo)
{
	if (!undo->valid) {
		return;
	}

	ctx->ip4->check = ipv4_csum_update_u32(ctx->ip4->check,
					       ctx->ip4->daddr, undo->daddr);
	ctx->ip4->daddr = undo->daddr;

	switch (undo->proto) {
	case IPPROTO_TCP:
		if (ctx->tcp && undo->has_l4) {
			ctx->tcp->dest = undo->dport;
			ctx->tcp->check = undo->l4_check;
		}
		break;
	case IPPROTO_UDP:
		if (ctx->udp && undo->has_l4) {
			ctx->udp->dest = undo->dport;
			ctx->udp->check = undo->l4_check;
		}
		break;
	case IPPROTO_ICMP:
		if (ctx->icmp && undo->has_l4) {
			ctx->icmp->un.echo.id = undo->dport;
			ctx->icmp->checksum = undo->l4_check;
		}
		break;
	}
}

// Returns true when the packet was translated back to a tracked UE flow.
static __always_inline bool destination_nat(struct packet_context *ctx,
					    struct nat_undo *undo)
{
	__u16 proto = ctx->ip4->protocol;
	struct nat_entry *origin;
	struct five_tuple key = {};
	if (!nat_ip4_lengths_valid(ctx->ip4, ctx->data_end)) {
		ctx->statistics->nat_malformed_drop_ip4 += 1;
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

		if (nat_icmp_query_for_reply(ctx->icmp->type)) {
			nat_ct_mark_replied(&key, origin, false, false, false);
		}

		/* Only a reply carries an identifier; in an error the same
		 * field holds the unused word or the next-hop MTU. */
		if (nat_icmp_query_for_reply(ctx->icmp->type) &&
		    ctx->icmp->un.echo.id != origin->src.identifier) {
			undo->dport = ctx->icmp->un.echo.id;
			undo->l4_check = ctx->icmp->checksum;
			undo->has_l4 = true;
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, ctx->icmp->un.echo.id,
				origin->src.identifier);
			ctx->icmp->un.echo.id = origin->src.identifier;
		}
		ctx->ip4->daddr = origin->src.saddr;
		break;
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			if (-1 == parse_tcp(ctx)) {
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
			return false;
		}

		nat_ct_mark_replied(&key, origin, ctx->tcp->fin,
				    ctx->tcp->syn && ctx->tcp->ack,
				    ctx->tcp->rst);

		undo->dport = ctx->tcp->dest;
		undo->l4_check = ctx->tcp->check;
		undo->has_l4 = true;
		ctx->ip4->daddr = origin->src.saddr;
		ctx->tcp->check = ipv4_csum_update_u32(
			ctx->tcp->check, key.saddr, ctx->ip4->daddr);
		ctx->tcp->dest = origin->src.sport;
		if (ctx->tcp->dest != key.sport) {
			ctx->tcp->check = ipv4_csum_update_u16(
				ctx->tcp->check, key.sport, ctx->tcp->dest);
		}
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			if (-1 == parse_udp(ctx)) {
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
			return false;
		}

		nat_ct_mark_replied(&key, origin, false, false, false);

		undo->dport = ctx->udp->dest;
		undo->l4_check = ctx->udp->check;
		undo->has_l4 = true;
		ctx->ip4->daddr = origin->src.saddr;
		/* A datagram sent without a checksum keeps none (RFC 768). */
		bool udp_summed = ctx->udp->check != 0;
		ctx->udp->dest = origin->src.sport;
		if (udp_summed) {
			ctx->udp->check = ipv4_csum_update_u32(
				ctx->udp->check, key.saddr, ctx->ip4->daddr);
			if (ctx->udp->dest != key.sport) {
				ctx->udp->check = ipv4_csum_update_u16(
					ctx->udp->check, key.sport,
					ctx->udp->dest);
			}
			if (ctx->udp->check == 0) {
				ctx->udp->check = 0xFFFF;
			}
		}
		break;
	default:
		return false;
	}
	ctx->ip4->check = ipv4_csum_update_u32(ctx->ip4->check, key.saddr,
					       ctx->ip4->daddr);
	undo->daddr = key.saddr;
	undo->proto = proto;
	/* An ICMP error also had its quoted packet rewritten, which the undo
	 * cannot reverse; such a packet must not itself be quoted. */
	undo->valid = undo->has_l4 || proto != IPPROTO_ICMP;
	return true;
}

#endif
