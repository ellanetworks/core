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
 * Upper bound on UDP/TCP datagram length for the L4-checksum recompute
 * helpers. Sized for jumbo frames (9000-byte MTU) with margin.
 *
 * The bound must not be (CSUM_SCRATCH_SIZE - 4) because the compiler
 * proves `__u16-sourced udp_len > 65536` is always false and optimizes
 * the upper-bound check away. The verifier then sees no umax on the
 * offset register, which makes the pointer arithmetic against the
 * scratch map invalid.
 */
#define MAX_L4_DATAGRAM 9000

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, CSUM_SCRATCH_SIZE);
	__uint(max_entries, 1);
} csum_scratch SEC(".maps");

// Recompute the full L4 checksum from packet bytes. Safe regardless of
// CHECKSUM_PARTIAL input state, which a hardware-offload sender (veth,
// virtio, real NIC) may leave on the wire as a pseudo-header sum.
//
// Helpers must run before the l4_len bounds check — otherwise the
// verifier loses tracked scalar bounds across helper calls.
__attribute__((noinline, used)) static int
udpv4_csum(__be32 saddr, __be32 daddr, __u32 udp_off, __u32 udp_len,
	   struct xdp_md *xdp_ctx)
{
	struct {
		__be32 src;
		__be32 dst;
		__u8 zero;
		__u8 proto;
		__be16 len;
	} __attribute__((packed)) pseudo;

	pseudo.src = saddr;
	pseudo.dst = daddr;
	pseudo.zero = 0;
	pseudo.proto = IPPROTO_UDP;
	pseudo.len = bpf_htons(udp_len);

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
	if (aligned_len > CSUM_SCRATCH_SIZE) {
		upf_printk("upf: bad aligned_len: %d", aligned_len);
		return -1;
	}

	csum = bpf_csum_diff(0, 0, (__be32 *)scratch, aligned_len, csum);
	return csum_fold_helper(csum);
}

__attribute__((noinline, used)) static int
tcpv4_csum(__be32 saddr, __be32 daddr, __u32 tcp_off, __u32 tcp_len,
	   struct xdp_md *xdp_ctx)
{
	struct {
		__be32 src;
		__be32 dst;
		__u8 zero;
		__u8 proto;
		__be16 len;
	} __attribute__((packed)) pseudo;

	pseudo.src = saddr;
	pseudo.dst = daddr;
	pseudo.zero = 0;
	pseudo.proto = IPPROTO_TCP;
	pseudo.len = bpf_htons(tcp_len);

	__u64 csum = bpf_csum_diff(0, 0, (__be32 *)&pseudo, sizeof(pseudo), 0);

	__u32 key = bpf_get_smp_processor_id();
	void *scratch = bpf_map_lookup_elem(&csum_scratch, &key);
	if (!scratch) {
		upf_printk("upf: could not read scratch buffer");
		return -1;
	}

	if (tcp_len > MAX_L4_DATAGRAM)
		tcp_len = MAX_L4_DATAGRAM;
	if (tcp_len < sizeof(struct tcphdr))
		tcp_len = sizeof(struct tcphdr);

	*(__u32 *)(scratch + tcp_len) = 0;

	if (bpf_xdp_load_bytes(xdp_ctx, tcp_off, scratch, tcp_len) < 0) {
		upf_printk("upf: couldn't load packet into scratch buffer");
		return -1;
	}

	__u32 aligned_len = (tcp_len + 3) & ~3U;
	if (aligned_len > CSUM_SCRATCH_SIZE) {
		upf_printk("upf: bad aligned_len: %d", aligned_len);
		return -1;
	}

	csum = bpf_csum_diff(0, 0, (__be32 *)scratch, aligned_len, csum);
	return csum_fold_helper(csum);
}

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
	if (aligned_len > CSUM_SCRATCH_SIZE) {
		upf_printk("upf: bad aligned_len: %d", aligned_len);
		return -1;
	}

	csum = bpf_csum_diff(0, 0, (__be32 *)scratch, aligned_len, csum);
	return csum_fold_helper(csum);
}

__attribute__((noinline, used)) static int
tcpv6_csum(const struct in6_addr *saddr, const struct in6_addr *daddr,
	   __u32 tcp_off, __u32 tcp_len, struct xdp_md *xdp_ctx)
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
	pseudo.upper_len = bpf_htonl(tcp_len);
	pseudo.zero[0] = 0;
	pseudo.zero[1] = 0;
	pseudo.zero[2] = 0;
	pseudo.next_hdr = IPPROTO_TCP;

	__u64 csum = bpf_csum_diff(0, 0, (__be32 *)&pseudo, sizeof(pseudo), 0);

	__u32 key = bpf_get_smp_processor_id();
	void *scratch = bpf_map_lookup_elem(&csum_scratch, &key);
	if (!scratch) {
		upf_printk("upf: could not read scratch buffer");
		return -1;
	}

	if (tcp_len > MAX_L4_DATAGRAM)
		tcp_len = MAX_L4_DATAGRAM;
	if (tcp_len < sizeof(struct tcphdr))
		tcp_len = sizeof(struct tcphdr);

	*(__u32 *)(scratch + tcp_len) = 0;

	if (bpf_xdp_load_bytes(xdp_ctx, tcp_off, scratch, tcp_len) < 0) {
		upf_printk("upf: couldn't load packet into scratch buffer");
		return -1;
	}

	__u32 aligned_len = (tcp_len + 3) & ~3U;
	if (aligned_len > CSUM_SCRATCH_SIZE) {
		upf_printk("upf: bad aligned_len: %d", aligned_len);
		return -1;
	}

	csum = bpf_csum_diff(0, 0, (__be32 *)scratch, aligned_len, csum);
	return csum_fold_helper(csum);
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
