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

#include <features.h>
#include <linux/bpf.h>
#include <linux/icmp.h>
#include <linux/icmpv6.h>
#include <linux/in.h>
#include <linux/in6.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
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

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, CSUM_SCRATCH_SIZE);
	__uint(max_entries, 1);
} csum_scratch SEC(".maps");

/*
 * udpv6_csum - compute the UDP checksum for IPv6.
 *
 * The UDP checksum covers an IPv6 pseudo-header (40 bytes) plus the UDP
 * header and payload.  This is required for IPv6 (RFC 2460 / 8200) and
 * mandatory when using GTP-U over IPv6 (3GPP TS 29.281).
 *
 * Implementation strategy (verifier-friendly, no loops):
 *   1. Build the 40-byte pseudo-header on the BPF stack and checksum it
 *      with bpf_csum_diff (stack memory, fixed size — always accepted).
 *   2. Copy the UDP datagram from packet memory into the per-CPU
 *      csum_scratch map using bpf_xdp_load_bytes.
 *   3. Checksum the scratch copy with bpf_csum_diff (map memory with a
 *      bounded variable length — accepted by the verifier).
 *
 * Every step is a single BPF helper call; the total verified-instruction
 * cost is O(1) regardless of packet size.
 *
 * @saddr / @daddr: source and destination IPv6 addresses
 * @udp_off: byte offset of the UDP header from the start of the XDP packet
 * @udp_len: length of the UDP datagram (header + payload)
 * @xdp_ctx: the XDP metadata context (needed by bpf_xdp_load_bytes)
 * @returns: the computed UDP checksum (network byte order), or 0 on error
 */
static __always_inline __u16 udpv6_csum(const struct in6_addr *saddr,
					const struct in6_addr *daddr,
					__u32 udp_off, __u32 udp_len,
					struct xdp_md *xdp_ctx)
{
	/* Build the IPv6 pseudo-header on the stack (40 bytes) */
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

	/* Checksum the pseudo-header (stack memory, fixed 40 bytes) */
	__u64 csum = bpf_csum_diff(0, 0, (__be32 *)&pseudo,
				   sizeof(pseudo), 0);

	/*
	 * Look up the per-CPU scratch buffer BEFORE the bounds checks on
	 * udp_len.  Helper calls (bpf_map_lookup_elem, bpf_csum_diff above)
	 * can cause the verifier to lose tracked bounds on scalar registers.
	 * By doing all helpers first and then checking udp_len, the bounds
	 * are fresh and provable at every subsequent use.
	 */
	__u32 key = bpf_get_smp_processor_id();
	void *scratch = bpf_map_lookup_elem(&csum_scratch, &key);
	if (!scratch)
		return 0;

	/*
	 * Clamp udp_len so the verifier can prove it fits in the scratch
	 * map.  This check MUST come after the last helper call (above)
	 * so the verifier's bounds knowledge is not clobbered.
	 */
	if (udp_len < sizeof(struct udphdr) || udp_len > (CSUM_SCRATCH_SIZE - 4))
		return 0;

	/*
	 * Zero a 4-byte word at offset udp_len in the scratch buffer.
	 * When udp_len is not a multiple of 4, this clears the 1-3
	 * padding bytes that bpf_csum_diff would otherwise read as stale
	 * data.  When udp_len IS a multiple of 4, this harmlessly zeros
	 * bytes outside the checksum range.
	 *
	 * The verifier knows 8 <= udp_len <= (CSUM_SCRATCH_SIZE - 4) (65536),
	 * so the write accesses [udp_len, udp_len + 4) which is within
	 * [8, 65540) ⊆ [0, CSUM_SCRATCH_SIZE).  No subtraction or AND
	 * arithmetic is involved, so there are no tnum precision issues.
	 */
	*(__u32 *)(scratch + udp_len) = 0;

	/* Copy the UDP datagram from packet memory into the scratch buffer */
	if (bpf_xdp_load_bytes(xdp_ctx, udp_off, scratch, udp_len) < 0)
		return 0;

	/*
	 * Checksum the scratch copy.  bpf_csum_diff reads in 4-byte words,
	 * so round up to the next multiple of 4.  The AND operation can
	 * cause the verifier's tnum tracking to overestimate the max value,
	 * and the preceding bpf_xdp_load_bytes helper may have clobbered
	 * the verifier's bounds on udp_len.  The explicit check gives the
	 * verifier a fresh, provable upper bound on aligned_len.
	 */
	__u32 aligned_len = (udp_len + 3) & ~3U;
	if (aligned_len > CSUM_SCRATCH_SIZE)
		return 0;

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
