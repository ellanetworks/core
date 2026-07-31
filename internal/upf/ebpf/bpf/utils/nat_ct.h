// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#include <linux/bpf.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <linux/icmp.h>
#include <linux/in.h>
#include <linux/ip.h>
#include <linux/udp.h>
#include <stdbool.h>
#include <sys/cdefs.h>

#include "bpf/utils/common.h"
#include "bpf/utils/csum.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/parsers.h"
#include "bpf/utils/pdr.h"

#ifndef NAT_CT_H
#define NAT_CT_H

#define NAT_READ_ONCE(x) (*(volatile typeof(x) *)&(x))
#define NAT_WRITE_ONCE(x, v) (*(volatile typeof(x) *)&(x) = (v))

#define PEAK_CONNECTION_PER_UE 100
#define NAT_CT_MAP_SIZE (2 * PEAK_CONNECTION_PER_UE * MAX_PDU_SESSIONS)
/* More-fragments bit and fragment offset. */
#define IP4_FRAG_MASK 0x3FFF

/* PSH, ECE and CWR carry no connection-state meaning. */
#define NAT_TCP_FLAGS_IGNORED 0xC8U
/* Bit n set means flag byte n is a combination RFC 793 permits: SYN, RST,
 * ACK, FIN|ACK, SYN|ACK, RST|ACK, SYN|URG, ACK|URG, FIN|ACK|URG. */
#define NAT_TCP_FLAGS_VALID 0x0003000400170014ULL
/* Bounded so the verifier walk of the retry loop stays within the 1M
 * instruction budget of the uplink program. */
#define NAT_PORT_RETRIES 16
/* Lowest ICMP identifier kept from the UE. */
#define NAT_ID_MIN 1024

volatile const bool masquerade;
volatile const bool masquerade = false;

/* Clear of the kernel's ephemeral range: a host socket is invisible to nat_ct,
 * so a shared port would let the host's reply resolve to a UE. */
volatile const __u16 nat_port_min;
volatile const __u16 nat_port_min = 1024;
volatile const __u16 nat_port_max;
volatile const __u16 nat_port_max = 32767;

static __always_inline bool nat_port_in_range(__u16 port_be)
{
	__u16 port = bpf_ntohs(port_be);

	return port >= nat_port_min && port <= nat_port_max;
}

/* An ICMP identifier is not drawn from the host's ephemeral range, so the
 * masquerade range neither constrains nor protects it. */
static __always_inline bool nat_id_reusable(__u16 proto, __u16 id_be)
{
	if (proto == IPPROTO_ICMP)
		return bpf_ntohs(id_be) >= NAT_ID_MIN;

	return nat_port_in_range(id_be);
}

/* Explicit padding: the kernel compares the whole key, so no byte may be
 * left undefined. */
struct five_tuple {
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
	__u16 proto;
	__u16 pad;
};

enum nat_ct_state {
	NAT_CT_NEW = 0,
	NAT_CT_ESTABLISHED = 1,
};

/* Set only by the uplink path: an inbound FIN or RST carries no sequence
 * validation, so it must not shorten a live mapping. */
#define NAT_CT_CLOSED 0x1

static __always_inline bool nat_icmp_is_query(__u8 type)
{
	return type == ICMP_ECHO || type == ICMP_TIMESTAMP ||
	       type == ICMP_INFO_REQUEST || type == ICMP_ADDRESS;
}

static __always_inline __u8 nat_icmp_query_for_reply(__u8 type)
{
	switch (type) {
	case ICMP_ECHOREPLY:
		return ICMP_ECHO;
	case ICMP_TIMESTAMPREPLY:
		return ICMP_TIMESTAMP;
	case ICMP_INFO_REPLY:
		return ICMP_INFO_REQUEST;
	case ICMP_ADDRESSREPLY:
		return ICMP_ADDRESS;
	}

	return 0;
}

/* One entry per direction, each keyed by one tuple and holding the other in
 * peer; the UE-side entry is authoritative for state and replied. */
struct nat_entry {
	struct five_tuple peer;
	__u64 refresh_ts;
	__u8 state;
	__u8 replied;
	__u8 ue_side;
	__u8 closed;
	__u8 pad[4];
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct five_tuple);
	__type(value, struct nat_entry);
	__uint(max_entries, NAT_CT_MAP_SIZE);
} nat_ct SEC(".maps");

/* A kernel that does not track a byte swap's bounds rejects packet-pointer
 * arithmetic on tot_len, so the mask has to reach the verifier. */
static __always_inline __u32 nat_ip4_tot_len(const struct iphdr *ip4)
{
	__u32 tot_len = bpf_ntohs(ip4->tot_len);

	barrier_var(tot_len);

	return tot_len & 0xFFFF;
}

/* Trailing frame bytes past tot_len are not part of the datagram. */
static __always_inline bool nat_ip4_lengths_valid(struct __ctx_buff *ctx_buff,
						  const struct iphdr *ip4,
						  const void *data_end)
{
	__u32 tot_len = nat_ip4_tot_len(ip4);
	__u32 hdr_len = ((__u32)ip4->ihl * 4) & 0x3C;

	if (tot_len < hdr_len) {
		return false;
	}

	return ctx_frame_holds(ctx_buff, data_end, ip4, tot_len);
}

static __always_inline bool nat_tcp_valid(const struct iphdr *ip4,
					  const struct tcphdr *tcp)
{
	__u16 tot_len = bpf_ntohs(ip4->tot_len);
	__u16 hdr_len = ip4->ihl * 4;

	if (tcp->doff < 5 || hdr_len + tcp->doff * 4 > tot_len) {
		return false;
	}

	__u8 flags = ((const __u8 *)tcp)[13] & ~NAT_TCP_FLAGS_IGNORED;

	return (NAT_TCP_FLAGS_VALID >> flags) & 1;
}

static __always_inline bool nat_udp_valid(const struct iphdr *ip4,
					  const struct udphdr *udp)
{
	__u16 len = bpf_ntohs(udp->len);

	return len >= sizeof(*udp) &&
	       ip4->ihl * 4 + len <= bpf_ntohs(ip4->tot_len);
}

static __always_inline bool nat_icmp_valid(const struct iphdr *ip4)
{
	__u16 tot_len = bpf_ntohs(ip4->tot_len);
	__u16 hdr_len = ip4->ihl * 4;

	return hdr_len + (__u16)sizeof(struct icmphdr) <= tot_len;
}

/* Bytes past the ICMP message are frame padding. */
static __always_inline const void *nat_icmp_msg_end(struct packet_context *ctx)
{
	return (const void *)ctx->ip4 + nat_ip4_tot_len(ctx->ip4);
}

static __always_inline bool are_five_tuple_equal(struct five_tuple a,
						 struct five_tuple b)
{
	return (a.saddr == b.saddr && a.daddr == b.daddr &&
		a.sport == b.sport && a.dport == b.dport && a.proto == b.proto);
}

#endif
