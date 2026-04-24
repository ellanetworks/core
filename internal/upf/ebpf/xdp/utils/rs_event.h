/**
 * Copyright 2026 Ella Networks
 */

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/in6.h>

/* ICMPv6 type for Router Solicitation (RFC 4861 section 4.1). */
#define ICMPV6_TYPE_ROUTER_SOLICITATION 133

/*
 * Event emitted to Go when an IPv6 Router Solicitation is detected
 * inside a GTP-U packet on the N3 uplink path.
 */
struct rs_event {
	__u32 teid; /* GTP-U TEID of the originating tunnel */
	struct in6_addr ue_ipv6; /* UE source IPv6 address from inner packet */
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(key, 0);
	__uint(value, 0);
	__uint(max_entries, 16384);
} rs_event_map SEC(".maps");
