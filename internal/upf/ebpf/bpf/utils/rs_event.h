/**
 * SPDX-FileCopyrightText: Ella Networks Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include <linux/in6.h>
#include <linux/ipv6.h>
#include "bpf/ctx/ctx.h"
#include "bpf/utils/common.h"
#include "bpf/utils/ringbuf_lost.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/parsers.h"

/* ICMPv6 type for Router Solicitation (RFC 4861 section 4.1). */
#define ICMPV6_TYPE_ROUTER_SOLICITATION 133

#define RS_HOP_LIMIT 255
#define RS_HDR_LEN 8
#define RS_MSG_MAX 1240

/*
 * Event emitted to Go when an IPv6 Router Solicitation is detected
 * inside a GTP-U packet on the N3 uplink path.
 */
struct rs_event {
	__u32 teid; /* GTP-U TEID of the originating tunnel */
	struct in6_addr ue_ipv6; /* UE source IPv6 address from inner packet */
	struct in6_addr dst_ipv6;
	__u16 msg_len;
	__u8 msg[RS_MSG_MAX];
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(key, 0);
	__uint(value, 0);
	__uint(max_entries, 65536);
} rs_event_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, __u32);
	__type(value, struct rs_event);
	__uint(max_entries, 1);
} rs_event_scratch SEC(".maps");

static __always_inline __s32 rs_icmp_len(const struct ipv6hdr *ip6,
					 __u32 l3_hdr_len)
{
	return (__s32)(sizeof(struct ipv6hdr) + bpf_ntohs(ip6->payload_len)) -
	       (__s32)l3_hdr_len;
}

static __always_inline bool rs_has_frag_hdr(struct packet_context *ctx,
					    __u32 ip6_off)
{
	if (ctx->l3_hdr_len == sizeof(struct ipv6hdr))
		return false;

	struct ipv6_chain chain = {};

	return ipv6_walk_chain(ctx->ctx_buff, ip6_off, ctx->ip6->nexthdr,
			       &chain) < 0 ||
	       chain.has_frag_hdr;
}

__noinline __weak int rs_event_submit(struct __ctx_buff *ctx_buff, __u32 teid,
				      __u32 ip6_off, __u32 icmp6_off,
				      __u32 icmp_len)
{
	if (!ctx_buff)
		return -1;

	__u64 len = icmp_len;
	barrier_var(len);
	if (len < RS_HDR_LEN || len > RS_MSG_MAX)
		return -1;

	const __u32 cpu = bpf_get_smp_processor_id();
	struct rs_event *ev = bpf_map_lookup_elem(&rs_event_scratch, &cpu);
	if (!ev) {
		ringbuf_count_lost(RINGBUF_RS_EVENT);
		return 0;
	}

	if (ctx_load_bytes(ctx_buff, ip6_off + offsetof(struct ipv6hdr, saddr),
			   &ev->ue_ipv6, sizeof(ev->ue_ipv6)) < 0 ||
	    ctx_load_bytes(ctx_buff, ip6_off + offsetof(struct ipv6hdr, daddr),
			   &ev->dst_ipv6, sizeof(ev->dst_ipv6)) < 0 ||
	    ctx_load_bytes(ctx_buff, icmp6_off, ev->msg, len) < 0)
		return -1;

	ev->teid = teid;
	ev->msg_len = (__u16)len;

	if (bpf_ringbuf_output(&rs_event_map, ev,
			       offsetof(struct rs_event, msg) + len, 0) != 0)
		ringbuf_count_lost(RINGBUF_RS_EVENT);

	return 0;
}
