// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "bpf/utils/pdr.h"
#include "bpf/utils/pdr_maps.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/ip_addr.h"
#include "bpf/utils/sdf.h"
#include "bpf/utils/common.h"
#include <linux/in.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <bpf/bpf_helpers.h>

#define MAX_CLASSIFIER_RULES 32
#define MAX_CLASSIFIER_TARGETS 32
#define CLASSIFIER_TARGET_MASK (MAX_CLASSIFIER_TARGETS - 1)
#define CLASSIFIER_UPLINK 1
#define CLASSIFIER_DOWNLINK 2
#define CLASSIFIER_FAMILY_ANY 0
#define CLASSIFIER_FAMILY_IPV4 4
#define CLASSIFIER_FAMILY_IPV6 6
#define CLASSIFIER_NO_MATCH -1
#define CLASSIFIER_DISABLED -2

#define CLASSIFIER_PORTS_PACKET 0
#define CLASSIFIER_PORTS_RESOLVED 1
#define CLASSIFIER_PORTS_UNAVAILABLE 2

_Static_assert((MAX_CLASSIFIER_TARGETS & CLASSIFIER_TARGET_MASK) == 0,
	       "MAX_CLASSIFIER_TARGETS must be a power of two");

struct classifier_rule {
	struct in6_addr remote;
	__u32 tunnel;
	__u16 remote_port_low;
	__u16 remote_port_high;
	__u16 local_port_low;
	__u16 local_port_high;
	__u8 prefix_len;
	__u8 protocol;
	__u8 direction;
	__u8 family;
	__u8 target;
	__u8 qfi;
	__u8 pad[2];
};

struct classifier_target {
	__u32 pdr_id;
	__u32 qer_id;
	__u32 urr_id;
	__u32 pad;
	struct far_info far;
	struct qer_info qer;
};

struct sdf_classifier {
	__u8 num_rules;
	__u8 pad[7];
	struct classifier_rule rules[MAX_CLASSIFIER_RULES];
	struct classifier_target targets[MAX_CLASSIFIER_TARGETS];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, __u64);
	__type(value, struct sdf_classifier);
	__uint(max_entries, MAX_PDU_SESSIONS);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} sdf_classifiers SEC(".maps");

struct classifier_query {
	__u64 seid;
	__u64 imsi;
	__u32 filter_map_index;
	__u32 l3_off;
	__u32 tunnel;
	__u16 l3_hdr_len;
	__u16 sport;
	__u16 dport;
	__u16 local_port_override;
	__u8 enabled;
	__u8 direction;
	__u8 family;
	__u8 protocol;
	__u8 ports;
	__u8 qfi;
	__u16 remote_port;
	__u16 local_port;
	struct in6_addr remote;
	__be32 remote4;
	__be16 l4_ports[2];
	struct classifier_target target;
};

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct classifier_query);
	__uint(max_entries, 1);
} classifier_scratch SEC(".maps");

__noinline __weak int sdf_classify(struct __ctx_buff *ctx_buff,
				   struct classifier_query *q);

static __always_inline struct classifier_query *
classifier_query_for(const struct packet_context *ctx, __u64 seid,
		     __u8 direction)
{
	const __u32 zero = 0;
	struct classifier_query *q =
		bpf_map_lookup_elem(&classifier_scratch, &zero);
	if (!q)
		return NULL;

	if (ctx->ip4) {
		q->family = CLASSIFIER_FAMILY_IPV4;
		q->l3_off = ctx_frame_offset(ctx->ctx_buff, ctx->ip4);
	} else if (ctx->ip6) {
		q->family = CLASSIFIER_FAMILY_IPV6;
		q->l3_off = ctx_frame_offset(ctx->ctx_buff, ctx->ip6);
	} else {
		return NULL;
	}

	q->seid = seid;
	q->direction = direction;
	q->l3_hdr_len = ctx->l3_hdr_len;
	q->protocol = ctx->l4_proto;
	q->sport = ctx->l4_sport;
	q->dport = ctx->l4_dport;
	q->ports = ctx->l4_unavailable ? CLASSIFIER_PORTS_UNAVAILABLE :
		   ctx->l4_resolved    ? CLASSIFIER_PORTS_RESOLVED :
					 CLASSIFIER_PORTS_PACKET;
	q->local_port_override = 0;
	q->tunnel = 0;
	q->qfi = direction == CLASSIFIER_UPLINK ? ctx->qfi : 0;

	return q;
}

static __always_inline const struct classifier_query *
select_downlink(const struct packet_context *ctx, const struct pdr_info *pdr,
		__u16 local_port_override)
{
	struct classifier_query *q =
		classifier_query_for(ctx, pdr->local_seid, CLASSIFIER_DOWNLINK);
	if (!q)
		return NULL;

	q->local_port_override = local_port_override;
	q->imsi = pdr->imsi;
	q->filter_map_index = pdr->filter_map_index;
	q->enabled = pdr->flags & PDR_F_SDF;
	q->target.pdr_id = pdr->pdr_id;
	q->target.qer_id = pdr->qer_id;
	q->target.urr_id = pdr->urr_id;
	q->target.far = pdr->far;
	q->target.qer = pdr->qer;

	sdf_classify(ctx->ctx_buff, q);

	return q;
}

static __always_inline const struct classifier_query *
select_uplink(struct packet_context *ctx, const struct pdr_info *pdr,
	      __u32 teid)
{
	struct classifier_query *q =
		classifier_query_for(ctx, pdr->local_seid, CLASSIFIER_UPLINK);
	if (!q) {
		set_drop_reason(ctx, UPF_DROP_INTERNAL_MAP_LOOKUP_FAILED);
		return NULL;
	}

	q->enabled = 1;
	q->tunnel = teid;

	if (sdf_classify(ctx->ctx_buff, q) >= 0)
		return q;

	if (!(pdr->flags & PDR_F_FALLBACK)) {
		set_drop_reason(ctx, ctx->l4_unavailable ?
					     UPF_DROP_FRAGMENT_UNFILTERABLE :
					     UPF_DROP_BEARER_BINDING);
		return NULL;
	}

	if (pdr->qfi && pdr->qfi != ctx->qfi) {
		set_drop_reason(ctx, UPF_DROP_BEARER_BINDING);
		return NULL;
	}

	q->target.pdr_id = 0;

	return q;
}

static __always_inline bool classifier_port_matches(__u16 port, __u16 low,
						    __u16 high, bool unreadable)
{
	if (low == SDF_PORT_ANY && high == SDF_PORT_ANY)
		return true;

	if (unreadable)
		return false;

	return port >= low && port <= high;
}

static __always_inline int classifier_load_packet(struct __ctx_buff *ctx_buff,
						  struct classifier_query *q)
{
	const bool downlink = q->direction == CLASSIFIER_DOWNLINK;

	if (q->family == CLASSIFIER_FAMILY_IPV4) {
		if (ctx_load_bytes(
			    ctx_buff,
			    q->l3_off + (downlink ?
						 offsetof(struct iphdr, saddr) :
						 offsetof(struct iphdr, daddr)),
			    &q->remote4, sizeof(q->remote4)) < 0)
			return -1;

		ipv4_to_mapped(&q->remote, q->remote4);
	} else if (ctx_load_bytes(
			   ctx_buff,
			   q->l3_off +
				   (downlink ? offsetof(struct ipv6hdr, saddr) :
					       offsetof(struct ipv6hdr, daddr)),
			   &q->remote, sizeof(q->remote)) < 0) {
		return -1;
	}

	if (q->ports == CLASSIFIER_PORTS_PACKET &&
	    (q->protocol == IPPROTO_UDP || q->protocol == IPPROTO_TCP)) {
		if (ctx_load_bytes(ctx_buff, q->l3_off + q->l3_hdr_len,
				   q->l4_ports, sizeof(q->l4_ports)) < 0) {
			q->ports = CLASSIFIER_PORTS_UNAVAILABLE;
		} else {
			q->sport = bpf_ntohs(q->l4_ports[0]);
			q->dport = bpf_ntohs(q->l4_ports[1]);
		}
	}

	q->remote_port = downlink ? q->sport : q->dport;
	q->local_port = downlink ? q->dport : q->sport;

	if (q->local_port_override &&
	    (q->protocol == IPPROTO_UDP || q->protocol == IPPROTO_TCP))
		q->local_port = q->local_port_override;

	return 0;
}

static __always_inline bool
classifier_rule_matches(const struct classifier_rule *r,
			const struct classifier_query *q)
{
	if (r->direction != q->direction)
		return false;

	if (q->direction == CLASSIFIER_UPLINK && r->tunnel != q->tunnel)
		return false;

	if (r->qfi && r->qfi != q->qfi)
		return false;

	if (r->family != CLASSIFIER_FAMILY_ANY && r->family != q->family)
		return false;

	if (r->protocol != SDF_PROTO_ANY && r->protocol != q->protocol)
		return false;

	if (r->prefix_len > 128)
		return false;

	if (match_ipv6_prefix(&r->remote, r->prefix_len, &q->remote) < 0)
		return false;

	const bool unreadable = q->ports == CLASSIFIER_PORTS_UNAVAILABLE;

	return classifier_port_matches(q->remote_port, r->remote_port_low,
				       r->remote_port_high, unreadable) &&
	       classifier_port_matches(q->local_port, r->local_port_low,
				       r->local_port_high, unreadable);
}

__noinline __weak int sdf_classify(struct __ctx_buff *ctx_buff,
				   struct classifier_query *q)
{
	if (!ctx_buff || !q)
		return CLASSIFIER_NO_MATCH;

	if (!q->enabled)
		return CLASSIFIER_DISABLED;

	struct sdf_classifier *c =
		bpf_map_lookup_elem(&sdf_classifiers, &q->seid);
	if (!c)
		return CLASSIFIER_NO_MATCH;

	if (classifier_load_packet(ctx_buff, q) < 0)
		return CLASSIFIER_NO_MATCH;

	__u8 num = c->num_rules;

	if (num > MAX_CLASSIFIER_RULES)
		num = MAX_CLASSIFIER_RULES;

#pragma clang loop unroll(disable)
	for (__u8 i = 0; i < num; i++) {
		const struct classifier_rule *r = &c->rules[i];

		if (!classifier_rule_matches(r, q))
			continue;

		q->target = c->targets[r->target & CLASSIFIER_TARGET_MASK];

		return r->target;
	}

	return CLASSIFIER_NO_MATCH;
}
