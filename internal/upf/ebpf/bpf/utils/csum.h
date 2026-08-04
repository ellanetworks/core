/**
 * Copyright 2023 Edgecom LLC
 * SPDX-FileCopyrightText: Ella Networks Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Modified by Ella Networks.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 * 
 *     http://www.apache.org/licenses/LICENSE-2.0
 * 
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#pragma once

#include "bpf/ctx/ctx.h"
#include "bpf/utils/common.h"
#include "bpf/utils/trace.h"
#include <features.h>
#include <linux/bpf.h>
#include <linux/icmp.h>
#include <linux/icmpv6.h>
#include <linux/in.h>
#include <linux/in6.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/types.h>
#include <linux/udp.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>

static __always_inline __u16 csum_fold_helper(__u64 csum)
{
#pragma unroll
	for (int i = 0; i < 4; i++) {
		csum = (csum & 0xffff) + (csum >> 16);
	}

	return ~csum;
}

static __always_inline __u64 ipv4_csum(void *data_start, __u32 data_size)
{
	__u64 csum = bpf_csum_diff(0, 0, data_start, data_size, 0);
	return csum_fold_helper(csum);
}

static __always_inline void icmp_csum_replace(__u16 *sum, __u16 old_field,
					      __u16 new_field)
{
	__u16 csum = ~*sum;
	csum += ~old_field;
	csum += csum < (__u16)~old_field;
	csum += new_field;
	csum += csum < (__u16)new_field;
	*sum = ~csum;
}

static __always_inline __u16 ipv4_csum_update_u32(__u16 csum, __u32 orig,
						  __u32 new)
{
	__u32 nbo_orig = bpf_htonl(orig);
	__u32 nbo_new = bpf_htonl(new);
	__u32 new_sum = (__u32)bpf_htons(csum);

	new_sum = ~new_sum & 0xFFFF;
	new_sum += ~(nbo_orig >> 16) & 0xFFFF;
	new_sum += ~nbo_orig & 0xFFFF;
	new_sum += (nbo_new >> 16) & 0xFFFF;
	new_sum += nbo_new & 0xFFFF;
	/* Five 16-bit addends can carry twice (RFC 1624). */
	new_sum = (new_sum & 0xFFFF) + (new_sum >> 16);
	new_sum = (new_sum & 0xFFFF) + (new_sum >> 16);
	new_sum = ~new_sum;

	return bpf_ntohs(new_sum);
}

static __always_inline __u16 ipv4_csum_update_u16(__u16 csum, __u16 orig,
						  __u16 new)
{
	__u32 nbo_orig = (__u32)bpf_htons(orig);
	__u32 nbo_new = (__u32)bpf_htons(new);
	__u32 new_sum = (__u32)bpf_htons(csum);

	new_sum = ~new_sum & 0xFFFF;
	new_sum += ~nbo_orig & 0xFFFF;
	new_sum += nbo_new & 0xFFFF;
	/* Three 16-bit addends can carry twice (RFC 1624). */
	new_sum = (new_sum & 0xFFFF) + (new_sum >> 16);
	new_sum = (new_sum & 0xFFFF) + (new_sum >> 16);
	new_sum = ~new_sum;

	return bpf_ntohs(new_sum);
}

static __always_inline void recompute_ipv4_csum(struct iphdr *ip)
{
	__u32 csum = 0;
	ip->check = 0;
	__u16 *word = (__u16 *)ip;
	for (int i = 0; i < (int)sizeof(*ip) >> 1; i++) {
		csum += *word++;
	}
	ip->check = csum_fold_helper(csum);
}

static __always_inline void recompute_icmp_csum(struct icmphdr *icmp, int len)
{
	__u32 csum = 0;
	icmp->checksum = 0;
	__u16 *word = (__u16 *)icmp;
	for (int i = 0; i < len >> 1; i++) {
		csum += *word++;
	}
	icmp->checksum = csum_fold_helper(csum);
}

/*
 * Per-CPU scratch buffer for udpv6_csum.
 *
 * bpf_csum_diff() cannot operate on packet memory with a variable length
 * (the BPF verifier rejects the access).  We work around this by first
 * copying the UDP datagram into this per-CPU map with ctx_load_bytes(),
 * then running bpf_csum_diff() on the map value.  Both helpers are O(1)
 * in verified instructions, so this approach keeps the verifier cost
 * minimal regardless of packet size.
 *
 * The buffer is sized to hold the maximum UDP datagram (65535 bytes, the
 * largest value representable in the 16-bit UDP length field) plus 4 bytes
 * of padding for 4-byte alignment when checksumming with bpf_csum_diff.
 *
 * We use a regular BPF_MAP_TYPE_ARRAY (not PERCPU_ARRAY) because the
 * per-CPU allocator rejects values larger than ~32 KB.  Instead, we
 * create one entry per possible CPU and index by bpf_get_smp_processor_id().
 * XDP programs run with preemption and migration disabled, so each CPU
 * exclusively owns its slot — identical isolation to a per-CPU map.
 *
 * max_entries is set to 1 here as a placeholder; the Go loader overwrites
 * it with the actual CPU count at load time.
 */
#define CSUM_SCRATCH_SIZE 65540

/*
 * Upper bound on the UDP datagram length summed by udpv6_csum. Sized for jumbo
 * frames (9000-byte MTU) with margin.
 *
 * It must not be (CSUM_SCRATCH_SIZE - 4): the compiler proves a __u16-sourced
 * length cannot exceed 65536 and drops the upper-bound check, leaving the
 * offset register without a umax and the scratch pointer arithmetic
 * unverifiable.
 */
#define MAX_L4_DATAGRAM 9000

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, CSUM_SCRATCH_SIZE);
	__uint(max_entries, 1);
} csum_scratch SEC(".maps");

#define L4_CSUM_CHUNK 512U
#define L4_CSUM_MAX_CHUNKS \
	((MAX_L4_DATAGRAM + L4_CSUM_CHUNK - 1) / L4_CSUM_CHUNK)

// Bounds the per-chunk offset for the verifier, which cannot infer a bound
// from the bpf_loop callback index. The mask exceeds MAX_L4_DATAGRAM, so at
// runtime (off <= 8704) it never alters a real offset, while keeping
// off + chunk within CSUM_SCRATCH_SIZE.
#define L4_CSUM_OFF_MASK 0x3FFFU

struct l4_csum_ctx {
	void *scratch;
	__u32 aligned_len;
	__s64 sum;
	int err;
};

/*
 * bpf_loop() callback: fold one <=512-byte window of the scratch buffer into
 * the running sum. Returns 1 to stop (window past the datagram, or a helper
 * error), 0 to continue.
 *
 * bpf_csum_diff() rejects buffers larger than MAX_BPF_STACK (512 bytes), hence
 * the windowing. Each chunk is a multiple of 4 bytes, keeping every window
 * 16-bit aligned so the partial sums chain through the seed.
 */
static long l4_csum_chunk(__u32 index, void *vctx)
{
	struct l4_csum_ctx *c = vctx;

	// Mask rather than bounds-check: the AND constrains the exact register
	// used for the access (which the verifier tracks); a compare binds a copy
	// the compiler then leaves out of the pointer arithmetic.
	__u32 off = (index * L4_CSUM_CHUNK) & L4_CSUM_OFF_MASK;

	if (off >= c->aligned_len)
		return 1;

	__u32 chunk = c->aligned_len - off;
	if (chunk > L4_CSUM_CHUNK)
		chunk = L4_CSUM_CHUNK;

	__s64 s = bpf_csum_diff(0, 0, (__be32 *)(c->scratch + off), chunk,
				(__wsum)c->sum);
	if (s < 0) {
		c->err = 1;
		return 1;
	}

	c->sum = s;

	return 0;
}

/*
 * Sum aligned_len bytes of the per-CPU scratch buffer into seed and fold the
 * result into a 16-bit L4 checksum. bpf_loop() walks the windows, so the
 * verifier checks the callback body once regardless of MAX_L4_DATAGRAM.
 *
 * Returns the folded checksum (0..0xffff), or -1 on helper error.
 */
static __always_inline int l4_csum_finalize(void *scratch, __u32 aligned_len,
					    __wsum seed)
{
	struct l4_csum_ctx c = {
		.scratch = scratch,
		.aligned_len = aligned_len,
		.sum = seed,
		.err = 0,
	};

	bpf_loop(L4_CSUM_MAX_CHUNKS, l4_csum_chunk, &c, 0);

	if (c.err)
		return -1;

	return csum_fold_helper((__u64)c.sum);
}

/* A valid L4 region — header, payload and its own check field — sums to
 * ~pseudo_header: with F = ~fold(pseudo + S), the region sums to S + F. That
 * is true of the bytes on the wire whether the sender wrote F or the kernel
 * completes it at egress, so an outer checksum built by substituting
 * ~pseudo_inner for the whole inner L4 region needs no access to ip_summed,
 * which BPF cannot read. */

/* In the order the checksum covers it. */
struct ip4_pseudo {
	__be32 saddr;
	__be32 daddr;
	__u8 zero;
	__u8 proto;
	__be16 l4_len;
};

struct ip6_pseudo {
	struct in6_addr src;
	struct in6_addr dst;
	__be32 upper_len;
	__u8 zero[3];
	__u8 next_hdr;
};

/* SCTP carries a CRC32c rather than a ones-complement checksum, so the
 * identity does not hold for it. */
static __always_inline int inner_l4_is_summable(__u8 proto)
{
	return proto == IPPROTO_TCP || proto == IPPROTO_UDP ||
	       proto == IPPROTO_ICMPV6;
}

/* ~pseudo_inner for an inner IPv4 packet, or -1 when the identity does not
 * apply: a fragment has no L4 header of its own, and UDP with a zero checksum
 * has nothing to substitute. Both are always CHECKSUM_NONE, so the caller's
 * byte sum is correct for them. */
static __always_inline __s32 inner_pseudo_ip4(struct __ctx_buff *ctx,
					      const struct iphdr *ip4,
					      const void *data_end)
{
	if ((const void *)(ip4 + 1) > data_end)
		return -1;

	if (ip4->ihl != 5 || (ip4->frag_off & bpf_htons(0x3fff)) != 0)
		return -1;

	if (!inner_l4_is_summable(ip4->protocol))
		return -1;

	__u32 tot_len = bounded_u16(bpf_ntohs(ip4->tot_len));
	if (tot_len < sizeof(struct iphdr))
		return -1;

	/* The declared length has to be backed by real bytes. The outer
	 * headers were sized from it, so a shorter frame is already
	 * self-inconsistent; falling back to the byte sum turns that into a
	 * failed read and a drop. */
	if (!ctx_frame_holds(ctx, data_end, ip4, tot_len))
		return -1;

	if (ip4->protocol == IPPROTO_UDP) {
		const struct udphdr *udp = (const struct udphdr *)(ip4 + 1);

		if ((const void *)(udp + 1) > data_end)
			return -1;

		if (udp->check == 0)
			return -1;
	}

	struct ip4_pseudo pseudo = {
		.saddr = ip4->saddr,
		.daddr = ip4->daddr,
		.zero = 0,
		.proto = ip4->protocol,
		.l4_len = bpf_htons((__u16)(tot_len - sizeof(struct iphdr))),
	};

	return (__s32)csum_fold_helper(
		(__u64)bpf_csum_diff(0, 0, (__be32 *)&pseudo, sizeof(pseudo), 0));
}

/* Extension headers are not walked: an inner next-header that is not the L4
 * protocol falls back. */
static __always_inline __s32 inner_pseudo_ip6(struct __ctx_buff *ctx,
					      const struct ipv6hdr *ip6,
					      const void *data_end)
{
	if ((const void *)(ip6 + 1) > data_end)
		return -1;

	if (!inner_l4_is_summable(ip6->nexthdr))
		return -1;

	__u32 payload_len = bounded_u16(bpf_ntohs(ip6->payload_len));

	/* As in inner_pseudo_ip4. */
	if (!ctx_frame_holds(ctx, data_end, ip6, sizeof(*ip6) + payload_len))
		return -1;

	if (ip6->nexthdr == IPPROTO_UDP) {
		const struct udphdr *udp = (const struct udphdr *)(ip6 + 1);

		if ((const void *)(udp + 1) > data_end)
			return -1;

		if (udp->check == 0)
			return -1;
	}

	struct ip6_pseudo pseudo = {
		.upper_len = bpf_htonl(payload_len),
		.zero = { 0, 0, 0 },
		.next_hdr = ip6->nexthdr,
	};

	__builtin_memcpy(&pseudo.src, &ip6->saddr, sizeof(struct in6_addr));
	__builtin_memcpy(&pseudo.dst, &ip6->daddr, sizeof(struct in6_addr));

	return (__s32)csum_fold_helper(
		(__u64)bpf_csum_diff(0, 0, (__be32 *)&pseudo, sizeof(pseudo), 0));
}

/* Outer UDP-over-IPv6 checksum built from headers only. `tunnel_len` is the
 * GTP-U header including any extension chain, and must be a compile-time
 * constant so the bounded read is provable. Returns -1 when the inner packet
 * does not satisfy the identity, leaving the caller to sum the bytes. */
static __always_inline int
udpv6_csum_from_headers(struct __ctx_buff *ctx, const struct in6_addr *saddr,
			const struct in6_addr *daddr, const struct udphdr *udp,
			__u32 udp_len, const void *tunnel, __u32 tunnel_len,
			int inner_is_ip6, const void *data_end)
{
	const void *inner = (const void *)((const __u8 *)tunnel + tunnel_len);

	__s32 not_pseudo_inner =
		inner_is_ip6 ? inner_pseudo_ip6(ctx, inner, data_end) :
			       inner_pseudo_ip4(ctx, inner, data_end);
	if (not_pseudo_inner < 0)
		return -1;

	struct ip6_pseudo pseudo = {
		.upper_len = bpf_htonl(udp_len),
		.zero = { 0, 0, 0 },
		.next_hdr = IPPROTO_UDP,
	};

	__builtin_memcpy(&pseudo.src, saddr, sizeof(struct in6_addr));
	__builtin_memcpy(&pseudo.dst, daddr, sizeof(struct in6_addr));

	__s64 sum = bpf_csum_diff(0, 0, (__be32 *)&pseudo, sizeof(pseudo), 0);
	if (sum < 0)
		return -1;

	/* udp->check is still zero here; the caller writes the result. */
	if ((const void *)(udp + 1) > data_end)
		return -1;

	sum = bpf_csum_diff(0, 0, (__be32 *)udp, sizeof(*udp), (__wsum)sum);
	if (sum < 0)
		return -1;

	if ((const void *)((const __u8 *)tunnel + tunnel_len) > data_end)
		return -1;

	sum = bpf_csum_diff(0, 0, (__be32 *)tunnel, tunnel_len, (__wsum)sum);
	if (sum < 0)
		return -1;

	__u32 inner_hdr_len = inner_is_ip6 ? sizeof(struct ipv6hdr) :
					     sizeof(struct iphdr);
	if (inner + inner_hdr_len > data_end)
		return -1;

	sum = bpf_csum_diff(0, 0, (__be32 *)inner, inner_hdr_len, (__wsum)sum);
	if (sum < 0)
		return -1;

	/* The inner L4 region, as one 16-bit word. */
	__be16 substitute[2] = { (__be16)not_pseudo_inner, 0 };

	sum = bpf_csum_diff(0, 0, (__be32 *)substitute, sizeof(substitute),
			    (__wsum)sum);
	if (sum < 0)
		return -1;

	__u16 check = csum_fold_helper((__u64)sum);

	/* RFC 8200: a zero checksum is invalid over IPv6. */
	return check == 0 ? 0xFFFF : check;
}

// udpv6_csum computes the full UDP-over-IPv6 checksum from packet bytes via the
// chunked bpf_loop helper above. The IPv6 UDP checksum is mandatory, so it is
// computed fresh over the payload during GTP-over-IPv6 encapsulation (gtp.h).
__attribute__((noinline, used)) static int
udpv6_csum(const struct in6_addr *saddr, const struct in6_addr *daddr,
	   __u32 udp_off, __u32 udp_len, struct __ctx_buff *ctx_buff)
{
	struct {
		struct in6_addr src;
		struct in6_addr dst;
		__be32 upper_len;
		__u8 zero[3];
		__u8 next_hdr;
	} pseudo;

	__builtin_memcpy(&pseudo.src, saddr, sizeof(struct in6_addr));
	__builtin_memcpy(&pseudo.dst, daddr, sizeof(struct in6_addr));
	pseudo.upper_len = bpf_htonl(udp_len);
	pseudo.zero[0] = 0;
	pseudo.zero[1] = 0;
	pseudo.zero[2] = 0;
	pseudo.next_hdr = IPPROTO_UDP;

	__u64 csum = bpf_csum_diff(0, 0, (__be32 *)&pseudo, sizeof(pseudo), 0);

	__u32 key = bpf_get_smp_processor_id();
	void *scratch = bpf_map_lookup_elem(&csum_scratch, &key);
	if (!scratch) {
		upf_printk("upf: could not read scratch buffer");
		return -1;
	}

	// Two-sided conditional-assignment clamp: an early return leaves
	// the verifier without a tracked bound on the value used later by
	// ctx_load_bytes. A malformed-short length is clamped to the
	// header size rather than rejected — produces a wrong checksum
	// that the receiver drops, no worse than the malformed packet.
	if (udp_len > MAX_L4_DATAGRAM)
		udp_len = MAX_L4_DATAGRAM;
	if (udp_len < sizeof(struct udphdr))
		udp_len = sizeof(struct udphdr);

	*(__u32 *)(scratch + udp_len) = 0;

	if (ctx_load_bytes(ctx_buff, udp_off, scratch, udp_len) < 0) {
		upf_printk("upf: couldn't load packet into scratch buffer");
		return -1;
	}

	__u32 aligned_len = (udp_len + 3) & ~3U;

	int check = l4_csum_finalize(scratch, aligned_len, (__wsum)csum);
	if (check < 0)
		return -1;

	/* RFC 768: a computed zero is transmitted as all ones. Over IPv6 a zero
	 * checksum is not "no checksum" but invalid, and the receiver drops the
	 * datagram (RFC 8200). */
	return check == 0 ? 0xFFFF : check;
}

/* Bytes of the trigger quoted back in an ICMP error, starting at its IP
 * header; a compile-time constant because the checksum helpers walk it with an
 * unrolled loop. Well within RFC 792's 576 bytes and RFC 4443's minimum MTU.
 *
 * The size is what clears the floor bpf_skb_change_tail enforces on a
 * CHECKSUM_PARTIAL skb: __bpf_skb_min_len (net/core/filter.c) refuses a trim
 * below csum_start + csum_offset + 2, 80 bytes for an IPv4 TCP trigger and 120
 * for IPv6. At 28 the resize failed and no error was emitted at all.
 *
 * The reply still carries the trigger's csum_start, now inside the quote, and
 * no helper clears CHECKSUM_PARTIAL: bpf_skb_adjust_room resets only
 * CHECKSUM_UNNECESSARY, bpf_skb_change_tail only CHECKSUM_COMPLETE. An egress
 * that completes the checksum therefore corrupts the quote. A partial trigger
 * implies a sender on this host, so the reply is delivered here, where
 * skb_csum_unnecessary() holds for CHECKSUM_PARTIAL and nothing completes
 * it. */
#define ICMP_QUOTE_LEN 128

/*
 * icmpv6_ptb_csum - compute the ICMPv6 checksum for a Packet Too Big message.
 *
 * The ICMPv6 checksum covers an IPv6 pseudo-header (40 bytes) plus the entire
 * ICMPv6 message.  For the Packet Too Big message we include:
 *   icmp6hdr (8) + ICMP_QUOTE_LEN bytes of the original packet.
 *
 * @saddr / @daddr: source and destination addresses on the *new* IPv6 header
 *                  (already swapped relative to the original packet).
 * @icmp6: pointer to the ICMPv6 header in packet memory; must be followed by
 *         at least ICMP_QUOTE_LEN bytes within packet bounds.
 */
static __always_inline __u16 icmpv6_ptb_csum(const struct in6_addr *saddr,
					     const struct in6_addr *daddr,
					     struct icmp6hdr *icmp6)
{
	static const __u32 icmp6_msg_len =
		sizeof(struct icmp6hdr) + ICMP_QUOTE_LEN;

	/* Build the IPv6 pseudo-header on the BPF stack (40 bytes) */
	struct {
		struct in6_addr src;
		struct in6_addr dst;
		__be32 upper_len;
		__u8 zero[3];
		__u8 next_hdr;
	} pseudo;

	__builtin_memcpy(&pseudo.src, saddr, sizeof(struct in6_addr));
	__builtin_memcpy(&pseudo.dst, daddr, sizeof(struct in6_addr));
	pseudo.upper_len = bpf_htonl(icmp6_msg_len);
	pseudo.zero[0] = 0;
	pseudo.zero[1] = 0;
	pseudo.zero[2] = 0;
	pseudo.next_hdr = IPPROTO_ICMPV6;

	__u64 csum = bpf_csum_diff(0, 0, (__be32 *)&pseudo, sizeof(pseudo), 0);
	csum = bpf_csum_diff(0, 0, (__be32 *)icmp6, icmp6_msg_len, csum);
	return csum_fold_helper(csum);
}
