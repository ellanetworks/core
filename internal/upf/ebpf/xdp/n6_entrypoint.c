/**
 * Copyright 2023 Edgecom LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS"
 * BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#include <linux/in.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <sys/socket.h>

#include "xdp/program_array.h"
#include "xdp/statistics.h"
#include "xdp/qer.h"
#include "xdp/pdr_n6.h"
#include "xdp/sdf_filter.h"

#include "xdp/utils/common.h"
#include "xdp/utils/trace.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/csum.h"
#include "xdp/utils/gtp_utils.h" // Declares update_gtp_tunnel but not send_to_gtp_tunnel
#include "xdp/utils/routing.h"
#include "xdp/utils/icmp.h"
// Optionally include profile helpers if you use them:
// #include "xdp/utils/profile.h"

#define DEFAULT_XDP_ACTION XDP_PASS

/* Forward declarations for functions used in this file */
static __always_inline enum xdp_action handle_ip4(struct packet_context *ctx);
static __always_inline enum xdp_action handle_ip6(struct packet_context *ctx);
static __always_inline enum xdp_action handle_n6_packet_ipv4(struct packet_context *ctx);
static __always_inline enum xdp_action process_n6_packet(struct packet_context *ctx);

/* These functions are not used in the N6 program.
   Mark them as unused to silence compiler warnings. */
static __always_inline enum xdp_action send_to_gtp_tunnel(struct packet_context *ctx,
                                                          int srcip, int dstip,
                                                          __u8 tos, __u8 qfi,
                                                          int teid) __attribute__((unused));
static __always_inline enum xdp_action handle_gtp_packet(struct packet_context *ctx) __attribute__((unused));

/* Process an N6 packet: route IPv4, IPv6, or pass ARP packets */
static __always_inline enum xdp_action process_n6_packet(struct packet_context *ctx)
{
    __u16 l3_proto = parse_ethernet(ctx);
    switch (l3_proto)
    {
    case ETH_P_IPV6:
        increment_counter(ctx->counters, rx_ip6);
        return handle_ip6(ctx);
    case ETH_P_IP:
        increment_counter(ctx->counters, rx_ip4);
        return handle_ip4(ctx);
    case ETH_P_ARP:
        increment_counter(ctx->counters, rx_arp);
        upf_printk("upf: arp received. passing to kernel");
        return XDP_PASS;
    default:
        return DEFAULT_XDP_ACTION;
    }
}

static __always_inline enum xdp_action handle_ip4(struct packet_context *ctx)
{
    int l4_protocol = parse_ip4(ctx);
    switch (l4_protocol)
    {
    case IPPROTO_ICMP:
        increment_counter(ctx->counters, rx_icmp);
        break;
    case IPPROTO_UDP:
        increment_counter(ctx->counters, rx_udp);
        /* If a packet is detected on the GTP-UDP port, it should be processed by the N3 program.
           In N6 we simply drop the packet.
        */
        if (GTP_UDP_PORT == parse_udp(ctx))
        {
            upf_printk("upf: gtp-u received on N6, dropping packet");
            return XDP_DROP;
        }
        break;
    case IPPROTO_TCP:
        increment_counter(ctx->counters, rx_tcp);
        break;
    default:
        increment_counter(ctx->counters, rx_other);
        return DEFAULT_XDP_ACTION;
    }
    increment_counter(ctx->n3_n6_counter, rx_n6);
    return handle_n6_packet_ipv4(ctx);
}

static __always_inline enum xdp_action handle_ip6(struct packet_context *ctx)
{
    int l4_protocol = parse_ip6(ctx);
    switch (l4_protocol)
    {
    case IPPROTO_ICMPV6: // Let kernel stack take care
        upf_printk("upf: icmp received. passing to kernel");
        increment_counter(ctx->counters, rx_icmp6);
        return XDP_PASS;
    case IPPROTO_UDP:
        increment_counter(ctx->counters, rx_udp);
        break;
    case IPPROTO_TCP:
        increment_counter(ctx->counters, rx_tcp);
        break;
    default:
        increment_counter(ctx->counters, rx_other);
        return DEFAULT_XDP_ACTION;
    }
    increment_counter(ctx->n3_n6_counter, rx_n6);
    return handle_n6_packet_ipv4(ctx);
}

/* N6 processing for IPv4: for now, simply route the packet */
static __always_inline enum xdp_action handle_n6_packet_ipv4(struct packet_context *ctx)
{
    return route_ipv4(ctx->xdp_ctx, ctx->eth, ctx->ip4);
}

/* Local version of send_to_gtp_tunnel (unused in N6) */
static __always_inline enum xdp_action send_to_gtp_tunnel(struct packet_context *ctx,
                                                          int srcip, int dstip,
                                                          __u8 tos, __u8 qfi,
                                                          int teid)
{
    if (-1 == add_gtp_over_ip4_headers(ctx, srcip, dstip, tos, qfi, teid))
        return XDP_ABORTED;
    upf_printk("upf: send gtp pdu %pI4 -> %pI4", &ctx->ip4->saddr, &ctx->ip4->daddr);
    increment_counter(ctx->n3_n6_counter, tx_n3);
    return route_ipv4(ctx->xdp_ctx, ctx->eth, ctx->ip4);
}

SEC("xdp/upf_n6_entrypoint")
int upf_n6_entrypoint_func(struct xdp_md *ctx)
{
    const __u32 key = 0;
    struct upf_statistic *statistic = bpf_map_lookup_elem(&upf_ext_stat, &key);
    if (!statistic)
    {
        const struct upf_statistic initval = {};
        bpf_map_update_elem(&upf_ext_stat, &key, &initval, BPF_ANY);
        statistic = bpf_map_lookup_elem(&upf_ext_stat, &key);
        if (!statistic)
            return XDP_ABORTED;
    }

    struct packet_context context = {
        .data = (char *)(long)ctx->data,
        .data_end = (const char *)(long)ctx->data_end,
        .xdp_ctx = ctx,
        .counters = &statistic->upf_counters,
        .n3_n6_counter = &statistic->upf_n3_n6_counter,
    };

    enum xdp_action action = process_n6_packet(&context);
    statistic->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;
    return action;
}

char _license[] SEC("license") = "GPL";
