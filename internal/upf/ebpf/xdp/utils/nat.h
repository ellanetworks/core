// Copyright 2025 Ella Networks

#include <linux/bpf.h>
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

#define PEAK_CONNECTION_PER_UE 500
#define NAT_CT_MAP_SIZE (PEAK_CONNECTION_PER_UE * MAX_PDU_SESSIONS)
#define MAX_PORT_ATTEMPT 5

volatile const bool masquerade;
volatile const bool masquerade = false;

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
};

struct nat_entry {
	struct five_tuple src;
	__u64 refresh_ts;
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct five_tuple);
	__type(value, struct nat_entry);
	__uint(max_entries, NAT_CT_MAP_SIZE);
} nat_ct SEC(".maps");

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
parse_icmp_packet_ref(struct five_tuple *key, struct packet_context *ctx)
{
	struct iphdr *ip4;
	struct udphdr *udp;
	struct tcphdr *tcp;
	struct nat_entry *nat_entry = NULL;

	ip4 = detect_ip4_header(ctx);
	if (!ip4) {
		return NULL;
	}
	key->saddr = ip4->saddr;
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
		if (!nat_entry) {
			return NULL;
		}
		__u16 previous_udp_csum = udp->check;
		ip4->saddr = nat_entry->src.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key->saddr, ip4->saddr);
		udp->source = nat_entry->src.sport;
		if (udp->check != 0) {
			udp->check = ipv4_csum_update_u32(
				udp->check, key->saddr, ip4->saddr);
			if (udp->source != key->sport) {
				udp->check = ipv4_csum_update_u16(
					udp->check, key->sport, udp->source);
			}
			ctx->icmp->checksum = ipv4_csum_update_u16(
				ctx->icmp->checksum, previous_udp_csum,
				udp->check);
		}
		ip4->check = 0;
		ip4->check = ipv4_csum(ip4, sizeof(*ip4));
		break;
	case IPPROTO_TCP:
		tcp = detect_tcp_header(ctx, offset);
		if (!tcp) {
			return NULL;
		}
		key->proto = ip4->protocol;
		key->sport = tcp->source;
		key->dport = tcp->dest;
		nat_entry = bpf_map_lookup_elem(&nat_ct, key);
		if (!nat_entry) {
			return NULL;
		}
		__u16 previous_tcp_csum = tcp->check;
		ip4->saddr = nat_entry->src.saddr;
		ctx->icmp->checksum = ipv4_csum_update_u32(
			ctx->icmp->checksum, key->saddr, ip4->saddr);
		tcp->check = ipv4_csum_update_u32(tcp->check, key->saddr,
						  ip4->saddr);
		tcp->source = nat_entry->src.sport;
		if (tcp->source != key->sport) {
			tcp->check = ipv4_csum_update_u16(
				tcp->check, key->sport, tcp->source);
		}
		ctx->icmp->checksum = ipv4_csum_update_u16(
			ctx->icmp->checksum, previous_tcp_csum, tcp->check);
		ip4->check = 0;
		ip4->check = ipv4_csum(ip4, sizeof(*ip4));
		break;
	}
	ctx->icmp->checksum = ipv4_csum_update_u16(
		ctx->icmp->checksum, previous_ip_csum, ip4->check);
	return nat_entry;
}

static __always_inline struct nat_entry *
find_origin_for_icmp(struct five_tuple *key, struct packet_context *ctx)
{
	switch (key->type) {
	case ICMP_ECHOREPLY:
		key->type = ICMP_ECHO;
		return bpf_map_lookup_elem(&nat_ct, key);
	case ICMP_TIMESTAMPREPLY:
		key->type = ICMP_TIMESTAMP;
		return bpf_map_lookup_elem(&nat_ct, key);
	case ICMP_DEST_UNREACH:
	case ICMP_TIME_EXCEEDED:
		if (!parse_icmp_packet_ref(key, ctx))
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
static __always_inline bool source_nat(struct packet_context *ctx,
				       struct bpf_fib_lookup *fib_params)
{
	__u16 proto = ctx->ip4->protocol;
	struct five_tuple orig = {};
	orig.saddr = ctx->ip4->saddr;
	orig.daddr = ctx->ip4->daddr;
	orig.proto = proto;

	ctx->ip4->saddr = fib_params->ipv4_src;
	ctx->ip4->check = 0;
	ctx->ip4->check = ipv4_csum(ctx->ip4, sizeof(*ctx->ip4));

	switch (proto) {
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			if (-1 == parse_tcp(ctx)) {
				return false;
			}
		}
		orig.sport = ctx->tcp->source;
		orig.dport = ctx->tcp->dest;
		ctx->tcp->check = ipv4_csum_update_u32(
			ctx->tcp->check, orig.saddr, ctx->ip4->saddr);
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			if (-1 == parse_udp(ctx)) {
				return false;
			}
		}
		orig.sport = ctx->udp->source;
		orig.dport = ctx->udp->dest;
		if (ctx->udp->check != 0) {
			ctx->udp->check = ipv4_csum_update_u32(
				ctx->udp->check, orig.saddr, ctx->ip4->saddr);
		}
		break;
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			if (-1 == parse_icmp(ctx)) {
				return false;
			}
		}
		if (ctx->icmp->type == ICMP_ECHO ||
		    ctx->icmp->type == ICMP_TIMESTAMP) {
			orig.identifier = ctx->icmp->un.echo.id;
			orig.type = ctx->icmp->type;
		} else {
			orig.identifier = 0;
			orig.type = ctx->icmp->type;
			orig.code = ctx->icmp->code;
		}
		break;
	default:
		return false;
	}

	struct five_tuple natted = {};
	natted.saddr = fib_params->ipv4_src;
	natted.sport = orig.sport;
	natted.daddr = ctx->ip4->daddr;
	natted.dport = orig.dport;
	natted.proto = proto;

	// Check if we need to also NAT the source port. This should be rare,
	// only occuring if another UE somehow connects to the same destination
	// using the same source port.
	// We first check if we are already tracking this flow, and if the
	// port needs to be changed.
	// Otherwise, we check if the new source we plan to use is already tracked
	// for a different flow. In that case, we try to find a free random
	// source port.
	struct nat_entry *tracked = bpf_map_lookup_elem(&nat_ct, &orig);
	if (tracked && !are_five_tuple_equal(natted, tracked->src)) {
		// This flow is known and uses port NAT, we change it here
		natted.sport = tracked->src.sport;
		update_port(ctx, tracked->src.sport);
	} else {
		struct nat_entry *existing =
			bpf_map_lookup_elem(&nat_ct, &natted);
		if (existing && !are_five_tuple_equal(orig, existing->src)) {
			// The source port cannot be used as is, find a random
			// free one.
			for (int i = 0; i < MAX_PORT_ATTEMPT; i++) {
				natted.sport = bpf_get_prandom_u32();
				existing =
					bpf_map_lookup_elem(&nat_ct, &natted);
				if (!existing) {
					update_port(ctx, natted.sport);
					break;
				}
			}
			if (existing) {
				return false;
			}
		}
	}

	// At this point, the packet is fully modified. We save
	// the tracking information.
	struct nat_entry from_nat = {};
	from_nat.src = orig;
	struct nat_entry to_nat = {};
	to_nat.src = natted;
	to_nat.refresh_ts = bpf_ktime_get_ns();
	from_nat.refresh_ts = to_nat.refresh_ts;

	bpf_map_update_elem(&nat_ct, &orig, &to_nat, BPF_ANY);
	bpf_map_update_elem(&nat_ct, &natted, &from_nat, BPF_ANY);
	return true;
}

static __always_inline void destination_nat(struct packet_context *ctx)
{
	__u16 proto = ctx->ip4->protocol;
	struct nat_entry *origin;
	struct five_tuple key = {};
	key.proto = proto;
	key.saddr = ctx->ip4->daddr;
	key.daddr = ctx->ip4->saddr;
	switch (proto) {
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			if (-1 == parse_icmp(ctx)) {
				return;
			}
		}
		key.identifier = ctx->icmp->un.echo.id;
		key.type = ctx->icmp->type;
		key.code = ctx->icmp->code;
		origin = find_origin_for_icmp(&key, ctx);
		if (!origin) {
			return;
		}

		if (origin->src.proto == IPPROTO_ICMP) {
			ctx->icmp->un.echo.id = origin->src.identifier;
		}
		ctx->ip4->daddr = origin->src.saddr;
		break;
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			if (-1 == parse_tcp(ctx)) {
				return;
			}
		}
		key.sport = ctx->tcp->dest;
		key.dport = ctx->tcp->source;
		origin = bpf_map_lookup_elem(&nat_ct, &key);
		if (!origin) {
			return;
		}

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
				return;
			}
		}
		key.sport = ctx->udp->dest;
		key.dport = ctx->udp->source;
		origin = bpf_map_lookup_elem(&nat_ct, &key);
		if (!origin) {
			return;
		}

		ctx->ip4->daddr = origin->src.saddr;
		if (ctx->udp->check != 0) {
			ctx->udp->check = ipv4_csum_update_u32(
				ctx->udp->check, key.saddr, ctx->ip4->daddr);
		}
		ctx->udp->dest = origin->src.sport;
		if (ctx->udp->dest != key.sport && ctx->udp->check != 0) {
			ctx->udp->check = ipv4_csum_update_u16(
				ctx->udp->check, key.sport, ctx->udp->dest);
		}
		break;
	default:
		return;
	}
	ctx->ip4->check = 0;
	ctx->ip4->check = ipv4_csum(ctx->ip4, sizeof(*ctx->ip4));
}

#endif
