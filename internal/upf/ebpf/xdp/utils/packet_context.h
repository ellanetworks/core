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

#include "xdp/utils/statistics.h"
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/types.h>
#include <linux/udp.h>
#include <linux/tcp.h>
#include <linux/icmp.h>
#include "xdp/utils/gtpu.h"

#define INTERFACE_N3 0x0
#define INTERFACE_N6 0x1

struct vlan_hdr {
	__be16 h_vlan_TCI;
	__be16 h_vlan_encapsulated_proto;
};

/* Header cursor to keep track of current parsing position */
struct packet_context {
	void *data;
	const void *data_end;
	struct upf_statistic *uplink_statistics;
	struct upf_statistic *downlink_statistics;
	struct counters *counter;
	struct xdp_md *xdp_ctx;
	struct ethhdr *eth;
	struct iphdr *ip4;
	struct ipv6hdr *ip6;
	struct udphdr *udp;
	struct tcphdr *tcp;
	struct gtpuhdr *gtp;
	struct icmphdr *icmp;
	struct vlan_hdr *vlan;
	__u8 interface : 1;
};
