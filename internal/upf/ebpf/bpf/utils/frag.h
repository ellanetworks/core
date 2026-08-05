// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "bpf/ctx/ctx.h"
#include "bpf/utils/common.h"
#include "bpf/utils/ip_addr.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/pdr.h"
#include <linux/in.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/types.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>

/* Only the first fragment carries the upper-layer header. Its ports are keyed
 * on the datagram's reassembly identity and recovered by the rest.
 *
 * One entry per fragmented datagram in flight, not per connection. */
#define FRAG_MAP_SIZE (2 * MAX_PDU_SESSIONS)

struct frag_key4 {
	__u32 saddr;
	__u32 daddr;
	__be16 id;
	__u8 proto;
	__u8 pad;
};

/* Laid out over struct ipv6hdr: the first eight bytes cover the fields the key
 * does not need, and the addresses fall where the header already has them, so
 * frag_track6 can build the key in the packet instead of on the stack. */
struct frag_key6 {
	__be32 id;
	__u32 pad;
	struct in6_addr saddr;
	struct in6_addr daddr;
} __attribute__((packed));

_Static_assert(sizeof(struct frag_key6) == sizeof(struct ipv6hdr),
	       "frag_key6 must overlay ipv6hdr exactly");
_Static_assert(__builtin_offsetof(struct frag_key6, saddr) ==
		       __builtin_offsetof(struct ipv6hdr, saddr) &&
		       __builtin_offsetof(struct frag_key6, daddr) ==
			       __builtin_offsetof(struct ipv6hdr, daddr),
	       "frag_key6 addresses must sit where ipv6hdr has them");

/* nat_id: the identification source_nat assigned, so every fragment leaves
 * under the same one. Zero when nothing translated the datagram. */
struct frag_ports {
	__u16 sport;
	__u16 dport;
	__be16 nat_id;
	__u16 pad;
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct frag_key4);
	__type(value, struct frag_ports);
	__uint(max_entries, FRAG_MAP_SIZE);
} frag_ports_ip4 SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct frag_key6);
	__type(value, struct frag_ports);
	__uint(max_entries, FRAG_MAP_SIZE);
} frag_ports_ip6 SEC(".maps");

/* Partitioned by CPU so the counter needs no atomic. Beyond
 * 1 << FRAG_NAT_ID_CPU_BITS CPUs partitions are shared, and the 16-bit field
 * wraps under sustained load either way (RFC 4963). Never zero. */
#define FRAG_NAT_ID_CPU_BITS 4
#define FRAG_NAT_ID_SEQ_MASK ((1U << (16 - FRAG_NAT_ID_CPU_BITS)) - 1)

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, __u32);
	__uint(max_entries, 1);
} frag_nat_id_seq SEC(".maps");

static __always_inline __be16 frag_next_nat_id(void)
{
	const __u32 key = 0;

	__u32 *seq = bpf_map_lookup_elem(&frag_nat_id_seq, &key);
	if (!seq)
		return 0;

	const __u32 cpu = bpf_get_smp_processor_id();
	__u16 id = (__u16)((cpu << (16 - FRAG_NAT_ID_CPU_BITS)) |
			   (*seq & FRAG_NAT_ID_SEQ_MASK));

	*seq += 1;

	return bpf_htons(id == 0 ? 1 : id);
}

static __always_inline void frag_key4_init(struct frag_key4 *key,
					   const struct iphdr *ip4)
{
	__builtin_memset(key, 0, sizeof(*key));
	key->saddr = ip4->saddr;
	key->daddr = ip4->daddr;
	key->id = ip4->id;
	key->proto = ip4->protocol;
}

/* First-wins (RFC 6274 §4.1.2.6): a forged second first-fragment must not
 * replace the ports the rest of the datagram is matched on. */
static __always_inline void frag_record4(const struct iphdr *ip4,
					 __u32 wire_saddr, __u32 wire_daddr,
					 __u16 sport, __u16 dport)
{
	struct frag_key4 key;
	struct frag_ports ports = {
		.sport = sport,
		.dport = dport,
	};

	frag_key4_init(&key, ip4);
	/* The addresses the next fragment will arrive with: translation may
	 * already have rewritten the header this key is built from. */
	key.saddr = wire_saddr;
	key.daddr = wire_daddr;

	bpf_map_update_elem(&frag_ports_ip4, &key, &ports, BPF_NOEXIST);
}

/* A miss leaves l4_unavailable set: whatever needs the ports decides what that
 * costs, and a session with neither port rules nor NAT needs none. */
static __always_inline void frag_resolve4(struct packet_context *ctx)
{
	struct frag_key4 key;

	if (!ctx->ip4 || !ctx->is_fragment || !ctx->l4_unavailable)
		return;

	frag_key4_init(&key, ctx->ip4);

	struct frag_ports *ports = bpf_map_lookup_elem(&frag_ports_ip4, &key);
	if (!ports) {
		ctx->statistics->packet_counters.frag_unresolved++;
		return;
	}

	ctx->l4_sport = ports->sport;
	ctx->l4_dport = ports->dport;
	ctx->frag_nat_id = ports->nat_id;
	ctx->l4_unavailable = 0;
	/* The bytes at ctx->data are still payload. */
	ctx->l4_resolved = 1;
	ctx->frag_recovered = 1;
}

/* The key is built over the packet's own IPv6 header: a 40-byte key does not
 * fit on a stack shared with ipv6_walk_chain's frame, which the verifier sums
 * against one 512-byte budget. The eight bytes it overwrites are saved and put
 * back on the single exit; every caller has already made the header writable.
 */
static __always_inline void frag_resolve6(struct packet_context *ctx)
{
	union frag_ip6_key {
		__u64 head;
		struct frag_key6 key;
		struct ipv6hdr ip6;
	};

	if (!ctx->ip6 || !ctx->is_fragment || !ctx->l4_unavailable)
		return;

	union frag_ip6_key *u = (union frag_ip6_key *)ctx->ip6;
	const __u64 saved = u->head;
	__u16 sport = 0;
	__u16 dport = 0;
	bool recovered = false;

	u->head = 0; /* Also clears the key's padding. */
	u->key.id = ctx->frag_id6;

	struct frag_ports *ports =
		bpf_map_lookup_elem(&frag_ports_ip6, &u->key);

	if (ports) {
		sport = ports->sport;
		dport = ports->dport;
		recovered = true;
	}

	u->head = saved;

	if (!recovered) {
		ctx->statistics->packet_counters.frag_unresolved++;
		return;
	}

	ctx->l4_sport = sport;
	ctx->l4_dport = dport;
	ctx->l4_unavailable = 0;
	ctx->frag_recovered = 1;
}

/* Deferred until a session owns the frame: an insert from unclaimed N6 traffic
 * would evict subscribers' entries from an LRU sized for sessions. Same
 * in-header key as frag_resolve6, and IPv6 is not translated, so the addresses
 * still read as they did at parse. */
static __always_inline void frag_record6(struct packet_context *ctx)
{
	union frag_ip6_key {
		__u64 head;
		struct frag_key6 key;
		struct ipv6hdr ip6;
	};

	if (!ctx->ip6 || !ctx->is_fragment || ctx->l4_unavailable ||
	    ctx->frag_recovered)
		return;

	union frag_ip6_key *u = (union frag_ip6_key *)ctx->ip6;
	const __u64 saved = u->head;
	struct frag_ports record = {
		.sport = ctx->l4_sport,
		.dport = ctx->l4_dport,
	};

	u->head = 0; /* Also clears the key's padding. */
	u->key.id = ctx->frag_id6;

	bpf_map_update_elem(&frag_ports_ip6, &u->key, &record, BPF_NOEXIST);

	u->head = saved;
}

/* Claims nat_id for this datagram and returns the identification in effect,
 * which is the one already stored if the datagram has one. The caller stamps
 * what it gets back: generating an identification and storing it are separate
 * outcomes, and a fragment leaving under one the map does not hold splits the
 * datagram across two identifications. */
static __always_inline __be16 frag_claim_nat_id(const struct iphdr *ip4,
						__be32 orig_saddr,
						__be16 orig_id, __be16 nat_id)
{
	struct frag_key4 key;

	frag_key4_init(&key, ip4);
	/* Recorded before translation. */
	key.saddr = orig_saddr;
	key.id = orig_id;

	struct frag_ports *ports = bpf_map_lookup_elem(&frag_ports_ip4, &key);
	if (!ports)
		return nat_id;

	/* First-wins, as the record is. A duplicated first fragment, or an
	 * identification reused while the entry lives, has to keep leaving
	 * under the identification its later fragments will resolve to. */
	if (ports->nat_id != 0)
		return ports->nat_id;

	ports->nat_id = nat_id;

	return nat_id;
}
