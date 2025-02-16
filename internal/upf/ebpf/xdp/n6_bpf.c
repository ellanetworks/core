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

#include "xdp/utils/statistics.h"
#include "xdp/utils/qer.h"
#include "xdp/utils/pdr.h"
#include "xdp/utils/sdf_filter.h"

#include "xdp/utils/common.h"
#include "xdp/utils/trace.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/csum.h"
#include "xdp/utils/gtp_utils.h"
#include "xdp/utils/routing.h"
#include "xdp/utils/icmp.h"

#define DEFAULT_XDP_ACTION XDP_PASS

/*
 * This function adds the necessary outer headers for downlink encapsulation
 * and then routes the packet. Note that the transmit counter is now updated
 * using the downlink counter (tx_n6).
 */
static __always_inline enum xdp_action send_to_gtp_tunnel(struct packet_context *ctx,
                                                          int srcip,
                                                          int dstip,
                                                          __u8 tos,
                                                          __u8 qfi,
                                                          int teid)
{
    if (-1 == add_gtp_over_ip4_headers(ctx, srcip, dstip, tos, qfi, teid))
        return XDP_ABORTED;
    upf_printk("upf: send gtp pdu %pI4 -> %pI4", &ctx->ip4->saddr, &ctx->ip4->daddr);
    increment_counter(ctx->n3_n6_counter, tx_n6);
    return route_ipv4(ctx->xdp_ctx, ctx->eth, ctx->ip4);
}

/*
 * Downlink processing for IPv4 packets.
 * Looks up the downlink session using the destination IP address.
 */
static __always_inline __u16 handle_n6_packet_ipv4(struct packet_context *ctx)
{
    const struct iphdr *ip4 = ctx->ip4;
    struct pdr_info *pdr = bpf_map_lookup_elem(&pdr_map_downlink_ip4, &ip4->daddr);
    if (!pdr)
    {
        upf_printk("upf: no downlink session for ip:%pI4", &ip4->daddr);
        return DEFAULT_XDP_ACTION;
    }

    __u32 far_id = pdr->far_id;
    __u32 qer_id = pdr->qer_id;
    if (pdr->sdf_mode)
    {
        struct sdf_filter *sdf = &pdr->sdf_rules.sdf_filter;
        if (match_sdf_filter_ipv4(ctx, sdf))
        {
            upf_printk("Packet with source ip:%pI4 and destination ip:%pI4 matches SDF filter",
                       &ip4->saddr, &ip4->daddr);
            far_id = pdr->sdf_rules.far_id;
            qer_id = pdr->sdf_rules.qer_id;
        }
        else if (pdr->sdf_mode & 1)
        {
            return DEFAULT_XDP_ACTION;
        }
    }

    struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
    if (!far)
    {
        upf_printk("upf: no downlink session far for ip:%pI4 far:%d", &ip4->daddr, far_id);
        return XDP_DROP;
    }

    upf_printk("upf: downlink session for ip:%pI4 far:%d action:%d", &ip4->daddr, far_id, far->action);
    if (!(far->action & FAR_FORW))
        return XDP_DROP;
    if (!(far->outer_header_creation & OHC_GTP_U_UDP_IPv4))
        return XDP_DROP;

    struct qer_info *qer = bpf_map_lookup_elem(&qer_map, &qer_id);
    if (!qer)
    {
        upf_printk("upf: no downlink session qer for ip:%pI4 qer:%d", &ip4->daddr, qer_id);
        return XDP_DROP;
    }

    upf_printk("upf: qer:%d gate_status:%d mbr:%d", qer_id, qer->dl_gate_status, qer->dl_maximum_bitrate);
    if (qer->dl_gate_status != GATE_STATUS_OPEN)
        return XDP_DROP;

    const __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
    if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->dl_start, qer->dl_maximum_bitrate))
        return XDP_DROP;

    __u8 tos = far->transport_level_marking >> 8;
    upf_printk("upf: use mapping %pI4 -> TEID:%d", &ip4->daddr, far->teid);

    /* Update downlink traffic counter */
    {
        struct upf_statistic *statistic = bpf_map_lookup_elem(&upf_ext_stat, &(__u32){0});
        if (statistic)
        {
            __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
            statistic->upf_counters.dl_bytes += packet_size; // Count downlink traffic
        }
    }
    return send_to_gtp_tunnel(ctx, far->localip, far->remoteip, tos, qer->qfi, far->teid);
}

/*
 * Downlink processing for IPv6 packets.
 */
static __always_inline enum xdp_action handle_n6_packet_ipv6(struct packet_context *ctx)
{
    const struct ipv6hdr *ip6 = ctx->ip6;
    struct pdr_info *pdr = bpf_map_lookup_elem(&pdr_map_downlink_ip6, &ip6->daddr);
    if (!pdr)
    {
        upf_printk("upf: no downlink session for ip:%pI6c", &ip6->daddr);
        return DEFAULT_XDP_ACTION;
    }

    __u32 far_id = pdr->far_id;
    __u32 qer_id = pdr->qer_id;
    if (pdr->sdf_mode)
    {
        struct sdf_filter *sdf = &pdr->sdf_rules.sdf_filter;
        if (match_sdf_filter_ipv6(ctx, sdf))
        {
            upf_printk("Packet with source ip:%pI6c and destination ip:%pI6c matches SDF filter",
                       &ip6->saddr, &ip6->daddr);
            far_id = pdr->sdf_rules.far_id;
            qer_id = pdr->sdf_rules.qer_id;
        }
        else if (pdr->sdf_mode & 1)
        {
            return DEFAULT_XDP_ACTION;
        }
    }

    struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
    if (!far)
    {
        upf_printk("upf: no downlink session far for ip:%pI6c far:%d", &ip6->daddr, far_id);
        return XDP_DROP;
    }

    upf_printk("upf: downlink session for ip:%pI6c far:%d action:%d", &ip6->daddr, far_id, far->action);
    if (!(far->action & FAR_FORW))
        return XDP_DROP;
    if (!(far->outer_header_creation & OHC_GTP_U_UDP_IPv4))
        return XDP_DROP;

    struct qer_info *qer = bpf_map_lookup_elem(&qer_map, &qer_id);
    if (!qer)
    {
        upf_printk("upf: no downlink session qer for ip:%pI6c qer:%d", &ip6->daddr, qer_id);
        return XDP_DROP;
    }

    upf_printk("upf: qer:%d gate_status:%d mbr:%d", qer_id, qer->dl_gate_status, qer->dl_maximum_bitrate);
    if (qer->dl_gate_status != GATE_STATUS_OPEN)
        return XDP_DROP;

    const __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
    if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->dl_start, qer->dl_maximum_bitrate))
        return XDP_DROP;

    __u8 tos = far->transport_level_marking >> 8;
    upf_printk("upf: use mapping %pI6c -> TEID:%d", &ip6->daddr, far->teid);
    return send_to_gtp_tunnel(ctx, far->localip, far->remoteip, tos, qer->qfi, far->teid);
}

/*
 * IPv4 handler: process L4 protocol, update counters,
 * then forward the packet to the downlink IPv4 processing function.
 */
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

/*
 * IPv6 handler.
 */
static __always_inline enum xdp_action handle_ip6(struct packet_context *ctx)
{
    int l4_protocol = parse_ip6(ctx);
    switch (l4_protocol)
    {
    case IPPROTO_ICMPV6:
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
    return handle_n6_packet_ipv6(ctx);
}

/*
 * Process the Ethernet header and dispatch to the appropriate handler.
 */
static __always_inline enum xdp_action process_packet(struct packet_context *ctx)
{
    __u16 l3_protocol = parse_ethernet(ctx);
    switch (l3_protocol)
    {
    case ETH_P_IPV6:
        increment_counter(ctx->counters, rx_ip6);
        return handle_ip6(ctx);
    case ETH_P_IP:
        increment_counter(ctx->counters, rx_ip4);
        return handle_ip4(ctx);
    case ETH_P_ARP:
        upf_printk("upf: arp received. passing to kernel");
        increment_counter(ctx->counters, rx_arp);
        return XDP_PASS;
    }
    return DEFAULT_XDP_ACTION;
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

    enum xdp_action action = process_packet(&context);
    statistic->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;

    return action;
}

char _license[] SEC("license") = "GPL";
