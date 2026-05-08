// Copyright 2026 Ella Networks

#pragma once

#include <linux/in6.h>
#include <linux/types.h>

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
	/* Check bytes 0-9 are zero and bytes 10-11 are 0xff */
	const __u8 *b = addr->in6_u.u6_addr8;
	return (b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0 && b[4] == 0 &&
		b[5] == 0 && b[6] == 0 && b[7] == 0 && b[8] == 0 && b[9] == 0 &&
		b[10] == 0xff && b[11] == 0xff);
}

static __always_inline __u32 ipv4_from_mapped(const struct in6_addr *addr)
{
	/* bytes 12-15 hold the IPv4 address in network byte order */
	return *(__u32 *)(&addr->in6_u.u6_addr8[12]);
}

static __always_inline void ipv4_to_mapped(struct in6_addr *addr, __u32 ip4_be)
{
	__builtin_memset(addr, 0, sizeof(*addr));
	addr->in6_u.u6_addr8[10] = 0xff;
	addr->in6_u.u6_addr8[11] = 0xff;
	*(__u32 *)(&addr->in6_u.u6_addr8[12]) = ip4_be;
}
