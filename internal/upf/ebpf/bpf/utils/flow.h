// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "bpf/utils/ip_addr.h"
#include "bpf/utils/packet_context.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "bpf/utils/nat.h"
#include <linux/icmp.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/in.h>
#include <sys/cdefs.h>

#define MAX_FLOW_PER_UE 100
#define FLOWACC_MAP_SIZE (MAX_PDU_SESSIONS * MAX_FLOW_PER_UE)

#define ALLOW 0
#define DROP 1

/* From the call site: N3 and N6 may share an interface or a master, so the
 * ingress ifindex cannot tell the sides apart. */
#define FLOW_UPLINK 0
#define FLOW_DOWNLINK 1

volatile const bool flowact;
volatile const bool flowact = false;

struct flow {
	__u64 imsi;
	struct in6_addr saddr;
	struct in6_addr daddr;
	__u32 ingress_ifindex;
	__u32 egress_ifindex;
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
	__u8 proto;
	__u8 dscp;
	__u8 action;
	__u8 direction; /* FLOW_UPLINK or FLOW_DOWNLINK; fills the trailing pad */
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

#define FLOW_PORTS_PACKET 0
#define FLOW_PORTS_RESOLVED 1
#define FLOW_PORTS_UNAVAILABLE 2
#define FLOW_ICMP_HDR_LEN 8

struct flow_query {
	__u32 l3_off;
	__u16 l3_hdr_len;
	__u8 ports;
	__u8 pad;
	__u8 hdr[FLOW_ICMP_HDR_LEN];
	struct flow key;
	struct flow_stats fresh;
};

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct flow_query);
	__uint(max_entries, 1);
} flow_scratch SEC(".maps");

__noinline __weak int flow_record(struct __ctx_buff *ctx_buff,
				  struct flow_query *fq)
{
	if (!ctx_buff || !fq)
		return 0;

	struct flow *f = &fq->key;

	switch (f->proto) {
	case IPPROTO_TCP:
	case IPPROTO_UDP:
		if (fq->ports == FLOW_PORTS_UNAVAILABLE)
			return 0;

		if (fq->ports == FLOW_PORTS_PACKET &&
		    ctx_load_bytes(ctx_buff, fq->l3_off + fq->l3_hdr_len,
				   &f->sport, 2 * sizeof(__u16)) < 0)
			return 0;
		break;
	case IPPROTO_ICMP:
		if (fq->ports != FLOW_PORTS_PACKET)
			return 0;

		if (ctx_load_bytes(ctx_buff, fq->l3_off + fq->l3_hdr_len,
				   fq->hdr, FLOW_ICMP_HDR_LEN) < 0)
			return 0;

		f->type = fq->hdr[offsetof(struct icmphdr, type)];

		if (f->type == ICMP_ECHO || f->type == ICMP_ECHOREPLY ||
		    f->type == ICMP_TIMESTAMP || f->type == ICMP_TIMESTAMPREPLY)
			__builtin_memcpy(
				&f->identifier,
				&fq->hdr[offsetof(struct icmphdr, un.echo.id)],
				sizeof(f->identifier));
		else
			f->code = fq->hdr[offsetof(struct icmphdr, code)];
		break;
	default:
		break;
	}

	const __u64 ts = bpf_ktime_get_ns();
	const __u64 packet_size = ctx_full_len(ctx_buff);

	struct flow_stats *flow_entry = bpf_map_lookup_elem(&flow_stats, f);
	if (flow_entry) {
		flow_entry->last_ts = ts;
		__sync_fetch_and_add(&flow_entry->bytes, packet_size);
		__sync_fetch_and_add(&flow_entry->packets, 1);
		return 0;
	}

	fq->fresh.first_ts = ts;
	fq->fresh.last_ts = ts;
	fq->fresh.bytes = packet_size;
	fq->fresh.packets = 1;

	bpf_map_update_elem(&flow_stats, f, &fq->fresh, BPF_ANY);

	return 0;
}

static __always_inline void account_flow(struct packet_context *ctx,
					 __u32 egress_ifindex, __u64 imsi,
					 __u8 ip_ver, __u8 direction,
					 __u8 action)
{
	if (!flowact)
		return;

	const __u32 zero = 0;
	struct flow_query *fq = bpf_map_lookup_elem(&flow_scratch, &zero);
	if (!fq)
		return;

	struct flow *f = &fq->key;

	__builtin_memset(f, 0, sizeof(*f));

	if (ip_ver == 4) {
		if (!ctx->ip4)
			return;

		ipv4_to_mapped(&f->saddr, ctx->ip4->saddr);
		ipv4_to_mapped(&f->daddr, ctx->ip4->daddr);
		f->proto = ctx->ip4->protocol;
		f->dscp = ctx->ip4->tos >> 2;
		fq->l3_off = ctx_frame_offset(ctx->ctx_buff, ctx->ip4);
	} else {
		if (!ctx->ip6)
			return;

		f->saddr = ctx->ip6->saddr;
		f->daddr = ctx->ip6->daddr;
		f->proto = ctx->l4_proto;
		f->dscp = (__u8)((ctx->ip6->priority << 2) |
				 (ctx->ip6->flow_lbl[0] >> 6));
		fq->l3_off = ctx_frame_offset(ctx->ctx_buff, ctx->ip6);
	}

	f->imsi = imsi;
	f->ingress_ifindex = ctx_ingress_ifindex(ctx->ctx_buff);
	f->egress_ifindex = egress_ifindex;
	f->action = action;
	f->direction = direction;
	f->sport = bpf_htons(ctx->l4_sport);
	f->dport = bpf_htons(ctx->l4_dport);

	fq->l3_hdr_len = ctx->l3_hdr_len;
	fq->ports = ctx->l4_unavailable ? FLOW_PORTS_UNAVAILABLE :
		    ctx->l4_resolved	? FLOW_PORTS_RESOLVED :
					  FLOW_PORTS_PACKET;

	flow_record(ctx->ctx_buff, fq);
}
