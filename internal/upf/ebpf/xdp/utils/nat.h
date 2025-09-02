// Copyright 2025 Ella Networks

#include "xdp/utils/csum.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <stdbool.h>
#include <sys/cdefs.h>

#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"

#ifndef NAT_H
#define NAT_H

#define NAT_CT_MAP_SIZE 4096

volatile const bool masquerade;
volatile const bool masquerade = false;

struct three_tuple {
	__u32 addr;
	__u16 port;
	__u16 proto;
};

struct nat_entry {
	struct three_tuple src;
	__u64 refresh_ts;
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct three_tuple);
	__type(value, struct nat_entry);
	__uint(max_entries, NAT_CT_MAP_SIZE);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} nat_ct SEC(".maps");

static __always_inline void destination_nat(struct packet_context *ctx)
{
	__u16 proto = ctx->ip4->protocol;
	struct three_tuple key = {};
	key.proto = proto;
	key.addr = ctx->ip4->daddr;
	switch (proto) {
	case IPPROTO_ICMP:
		key.port = 0;
		struct nat_entry *origin = bpf_map_lookup_elem(&nat_ct, &key);
		if (!origin)
			return;

		ctx->ip4->daddr = origin->src.addr;
		break;
	case IPPROTO_UDP:
	case IPPROTO_TCP:
	default:
		return;
	}
	ctx->ip4->check = 0;
	ctx->ip4->check = ipv4_csum(ctx->ip4, sizeof(*ctx->ip4));
}

#endif
