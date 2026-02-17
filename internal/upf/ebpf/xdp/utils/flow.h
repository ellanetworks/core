// Copyright 2026 Ella Networks

#pragma once

#include "xdp/utils/packet_context.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "xdp/utils/nat.h"
#include <linux/icmp.h>
#include <linux/in.h>
#include <sys/cdefs.h>

#define MAX_FLOW_PER_UE 100
#define FLOWACC_MAP_SIZE (MAX_PDU_SESSIONS * MAX_FLOW_PER_UE)

volatile const bool flowact;
volatile const bool flowact = false;

struct flow {
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
	__u32 ingress_ifindex;
	__u32 egress_ifindex;
	__u8 proto;
	__u8 tos;
};

struct flow_stats {
	__u64 first_ts;
	__u64 last_ts;
	__u64 bytes;
	__u64 packets;
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct flow);
	__type(value, struct flow_stats);
	__uint(max_entries, FLOWACC_MAP_SIZE);
} flow_stats SEC(".maps");

static __always_inline void account_flow(struct packet_context *ctx,
					 __u32 egress_ifindex)
{
	if (!flowact) return;

	struct flow f = {};
	f.saddr = ctx->ip4->saddr;
	f.daddr = ctx->ip4->daddr;
	f.proto = ctx->ip4->protocol;
	f.tos = ctx->ip4->tos;
	f.ingress_ifindex = ctx->xdp_ctx->ingress_ifindex;
	f.egress_ifindex = egress_ifindex;

	switch (f.proto) {
	case IPPROTO_TCP:
		if (!ctx->tcp) {
			if (-1 == parse_tcp(ctx)) {
				return;
			}
		}
		f.sport = ctx->tcp->source;
		f.dport = ctx->tcp->dest;
		break;
	case IPPROTO_UDP:
		if (!ctx->udp) {
			if (-1 == parse_udp(ctx)) {
				return;
			}
		}
		f.sport = ctx->udp->source;
		f.dport = ctx->udp->dest;
		break;
	case IPPROTO_ICMP:
		if (!ctx->icmp) {
			if (-1 == parse_icmp(ctx)) {
				return;
			}
		}
		if (ctx->icmp->type == ICMP_ECHO ||
		    ctx->icmp->type == ICMP_ECHOREPLY ||
		    ctx->icmp->type == ICMP_TIMESTAMP ||
		    ctx->icmp->type == ICMP_TIMESTAMPREPLY) {
			f.identifier = ctx->icmp->un.echo.id;
			f.type = ctx->icmp->type;
		} else {
			f.identifier = 0;
			f.type = ctx->icmp->type;
			f.code = ctx->icmp->code;
		}
		break;
	default:
		f.sport = 0;
		f.dport = 0;
	}

	__u64 ts = bpf_ktime_get_ns();
	__u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;

	struct flow_stats *flow_entry = bpf_map_lookup_elem(&flow_stats, &f);
	if (flow_entry) {
		flow_entry->last_ts = ts;
		__sync_fetch_and_add(&flow_entry->bytes, packet_size);
		__sync_fetch_and_add(&flow_entry->packets, 1);
		return;
	}

	struct flow_stats new_stats = {};
	new_stats.first_ts = ts;
	new_stats.last_ts = ts;
	new_stats.bytes = packet_size;
	new_stats.packets = 1;

	bpf_map_update_elem(&flow_stats, &f, &new_stats, BPF_ANY);
}
