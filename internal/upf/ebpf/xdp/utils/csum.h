/**
 * Copyright 2023 Edgecom LLC
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

#include "xdp/utils/trace.h"
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
	ip->check = ~((csum & 0xffff) + (csum >> 16));
}

static __always_inline void recompute_icmp_csum(struct icmphdr *icmp, int len)
{
	__u32 csum = 0;
	icmp->checksum = 0;
	__u16 *word = (__u16 *)icmp;
	for (int i = 0; i < len >> 1; i++) {
		csum += *word++;
	}
	icmp->checksum = ~((csum & 0xffff) + (csum >> 16));
}

/*
 * Per-CPU scratch buffer for udpv6_csum.
 *
 * bpf_csum_diff() cannot operate on packet memory with a variable length
 * (the BPF verifier rejects the access).  We work around this by first
 * copying the UDP datagram into this per-CPU map with bpf_xdp_load_bytes(),
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

// udpv6_csum computes the full UDP-over-IPv6 checksum from packet bytes via the
// chunked bpf_loop helper above. The IPv6 UDP checksum is mandatory, so it is
// computed fresh over the payload during GTP-over-IPv6 encapsulation (gtp.h).
__attribute__((noinline, used)) static int
udpv6_csum(const struct in6_addr *saddr, const struct in6_addr *daddr,
	   __u32 udp_off, __u32 udp_len, struct xdp_md *xdp_ctx)
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
	// bpf_xdp_load_bytes. A malformed-short length is clamped to the
	// header size rather than rejected — produces a wrong checksum
	// that the receiver drops, no worse than the malformed packet.
	if (udp_len > MAX_L4_DATAGRAM)
		udp_len = MAX_L4_DATAGRAM;
	if (udp_len < sizeof(struct udphdr))
		udp_len = sizeof(struct udphdr);

	*(__u32 *)(scratch + udp_len) = 0;

	if (bpf_xdp_load_bytes(xdp_ctx, udp_off, scratch, udp_len) < 0) {
		upf_printk("upf: couldn't load packet into scratch buffer");
		return -1;
	}

	__u32 aligned_len = (udp_len + 3) & ~3U;
	return l4_csum_finalize(scratch, aligned_len, (__wsum)csum);
}

/*
 * icmpv6_ptb_csum - compute the ICMPv6 checksum for a Packet Too Big message.
 *
 * The ICMPv6 checksum covers an IPv6 pseudo-header (40 bytes) plus the entire
 * ICMPv6 message.  For the Packet Too Big message we include:
 *   icmp6hdr (8) + original ipv6hdr (40) + first 8 bytes of original payload
 * = 56 bytes total ICMPv6 message.
 *
 * @saddr / @daddr: source and destination addresses on the *new* IPv6 header
 *                  (already swapped relative to the original packet).
 * @icmp6: pointer to the ICMPv6 header in packet memory; must be followed by
 *         at least (sizeof(ipv6hdr) + 8) bytes within packet bounds.
 */
static __always_inline __u16 icmpv6_ptb_csum(const struct in6_addr *saddr,
					     const struct in6_addr *daddr,
					     struct icmp6hdr *icmp6)
{
	/* Fixed ICMPv6 message length: 8 + 40 + 8 = 56 bytes */
	static const __u32 icmp6_msg_len =
		sizeof(struct icmp6hdr) + sizeof(struct ipv6hdr) + 8;

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
