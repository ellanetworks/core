// Copyright 2023 Edgecom LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#pragma once

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/in.h>
#include <linux/ip.h>
#include <linux/types.h>
#include <linux/icmp.h>

#include "xdp/utils/csum.h"
#include "xdp/utils/n6_parsers.h"
#include "xdp/utils/n6_packet_context.h"

static __always_inline void fill_icmp_header(struct icmphdr *icmp)
{
    icmp->type = ICMP_TIME_EXCEEDED;
    icmp->code = ICMP_EXC_TTL;
    icmp->un.gateway = 0;
    icmp->checksum = 0;
}

static __always_inline __u32 prepare_icmp_echo_reply(struct n6_packet_context *ctx, int saddr, int daddr)
{
    if (!ctx->ip4)
        return -1;

    struct ethhdr *eth = ctx->eth;
    swap_mac(eth);

    const char *data_end = (const char *)(long)ctx->xdp_ctx->data_end;
    struct iphdr *ip = ctx->ip4;
    if ((const char *)(ip + 1) > data_end)
        return -1;

    swap_ip(ip);

    struct icmphdr *icmp = (struct icmphdr *)(ip + 1);
    if ((const char *)(icmp + 1) > data_end)
        return -1;

    if (icmp->type != ICMP_ECHO)
        return -1;

    __u16 old = *(__u16 *)&icmp->type;
    icmp->type = ICMP_ECHOREPLY;
    icmp->code = 0;

    ipv4_csum_replace(&icmp->checksum, old, *(__u16 *)&icmp->type);

    return 0;
}
