// Copyright 2026 Ella Networks

#pragma once

#include <linux/in6.h>
#include <linux/types.h>
#include <bpf/bpf_endian.h>

#define IPV4 4
#define IPV6 6

/*
 * IPv4-mapped IPv6 address helpers
 *
 * IPv4 addresses are stored in in6_addr as ::ffff:x.x.x.x
 * (bytes 10-11 = 0xff, bytes 12-15 = IPv4 address in network order).
 */

static __always_inline int is_ipv4_mapped_ipv6(const struct in6_addr *addr)
{
	const __u32 *w = addr->in6_u.u6_addr32;
	__u32 diff;

	/* ::ffff:x.x.x.x -> [0]=0, [1]=0, [2]=0000ffff in network order */
	diff = w[0] | w[1] | (w[2] ^ bpf_htonl(0x0000ffff));
	return diff == 0;
}

static __always_inline __u32 ipv4_from_mapped(const struct in6_addr *addr)
{
	return addr->in6_u.u6_addr32[3];
}

static __always_inline void ipv4_to_mapped(struct in6_addr *addr, __u32 ip4_be)
{
	addr->in6_u.u6_addr32[0] = 0;
	addr->in6_u.u6_addr32[1] = 0;
	addr->in6_u.u6_addr32[2] = bpf_htonl(0x0000ffff);
	addr->in6_u.u6_addr32[3] = ip4_be;
}
