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

#include "xdp/program_array.h"
#include "xdp/statistics.h"
#include "xdp/qer.h"
#include "xdp/pdr.h"
#include "xdp/sdf_filter.h"

#include "xdp/utils/common.h"
#include "xdp/utils/trace.h"
#include "xdp/utils/packet_context.h"
#include "xdp/utils/parsers.h"
#include "xdp/utils/csum.h"
#include "xdp/utils/gtp_utils.h"
#include "xdp/utils/routing.h"
#include "xdp/utils/icmp.h"

#define DEFAULT_XDP_ACTION XDP_PASS

/* This helper encapsulates an IP packet in a GTP-U tunnel.
 * (Used for N6→N3 forwarding.)
 */
static __always_inline enum xdp_action send_to_gtp_tunnel(struct packet_context *ctx,
                                                          int srcip, int dstip,
                                                          __u8 tos, __u8 qfi, int teid)
{
    if (-1 == add_gtp_over_ip4_headers(ctx, srcip, dstip, tos, qfi, teid))
        return XDP_ABORTED;
    upf_printk("upf n6: send gtp pdu %pI4 -> %pI4", &ctx->ip4->saddr, &ctx->ip4->daddr);
    increment_counter(ctx->n3_n6_counter, tx_n3);
    return route_ipv4(ctx->xdp_ctx, ctx->eth, ctx->ip4);
}

/* Process an IPv4 packet arriving on the N6 interface.
 * Note: If a UDP packet on the GTP-U port is encountered on N6,
 * we drop it.
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
        if (parse_udp(ctx) == GTP_UDP_PORT)
        {
            upf_printk("upf n6: GTP-U packet received on N6, dropping");
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
    /* Forward as a native downlink IPv4 packet */
    {
        const struct iphdr *ip4 = ctx->ip4;
        struct pdr_info *pdr = bpf_map_lookup_elem(&pdr_map_downlink_ip4, &ip4->daddr);
        if (!pdr)
        {
            upf_printk("upf n6: no downlink session for ip:%pI4", &ip4->daddr);
            return DEFAULT_XDP_ACTION;
        }
        __u32 far_id = pdr->far_id;
        __u32 qer_id = pdr->qer_id;
        if (pdr->sdf_mode)
        {
            struct sdf_filter *sdf = &pdr->sdf_rules.sdf_filter;
            if (match_sdf_filter_ipv4(ctx, sdf))
            {
                upf_printk("upf n6: packet matches SDF filter");
                far_id = pdr->sdf_rules.far_id;
                qer_id = pdr->sdf_rules.qer_id;
            }
            else if (pdr->sdf_mode & 1)
                return DEFAULT_XDP_ACTION;
        }
        struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
        if (!far)
        {
            upf_printk("upf n6: no downlink session far for ip:%pI4 far:%d",
                       &ip4->daddr, far_id);
            return XDP_DROP;
        }
        upf_printk("upf n6: downlink session for ip:%pI4 far:%d", &ip4->daddr, far_id);
        if (!(far->action & FAR_FORW))
            return XDP_DROP;
        if (!(far->outer_header_creation & OHC_GTP_U_UDP_IPv4))
            return XDP_DROP;
        struct qer_info *qer = bpf_map_lookup_elem(&qer_map, &qer_id);
        if (!qer)
        {
            upf_printk("upf n6: no downlink session qer for ip:%pI4 qer:%d",
                       &ip4->daddr, qer_id);
            return XDP_DROP;
        }
        upf_printk("upf n6: qer:%d gate_status:%d", qer_id, qer->dl_gate_status);
        if (qer->dl_gate_status != GATE_STATUS_OPEN)
            return XDP_DROP;
        const __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
        if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->dl_start,
                                                  qer->dl_maximum_bitrate))
            return XDP_DROP;
        __u8 tos = far->transport_level_marking >> 8;
        upf_printk("upf n6: use mapping %pI4 -> TEID:%d", &ip4->daddr, far->teid);
        return send_to_gtp_tunnel(ctx, far->localip, far->remoteip, tos, qer->qfi, far->teid);
    }
}

/* Process an IPv6 packet arriving on the N6 interface. */
static __always_inline enum xdp_action handle_ip6(struct packet_context *ctx)
{
    int l4_protocol = parse_ip6(ctx);
    switch (l4_protocol)
    {
    case IPPROTO_ICMPV6:
        upf_printk("upf n6: icmpv6 received, passing to kernel");
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
    {
        const struct ipv6hdr *ip6 = ctx->ip6;
        struct pdr_info *pdr = bpf_map_lookup_elem(&pdr_map_downlink_ip6, &ip6->daddr);
        if (!pdr)
        {
            upf_printk("upf n6: no downlink session for ip:%pI6c", &ip6->daddr);
            return DEFAULT_XDP_ACTION;
        }
        __u32 far_id = pdr->far_id;
        __u32 qer_id = pdr->qer_id;
        if (pdr->sdf_mode)
        {
            struct sdf_filter *sdf = &pdr->sdf_rules.sdf_filter;
            if (match_sdf_filter_ipv6(ctx, sdf))
            {
                upf_printk("upf n6: packet matches SDF filter");
                far_id = pdr->sdf_rules.far_id;
                qer_id = pdr->sdf_rules.qer_id;
            }
            else if (pdr->sdf_mode & 1)
                return DEFAULT_XDP_ACTION;
        }
        struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
        if (!far)
        {
            upf_printk("upf n6: no downlink session far for ip:%pI6c far:%d",
                       &ip6->daddr, far_id);
            return XDP_DROP;
        }
        upf_printk("upf n6: downlink session for ip:%pI6c far:%d", &ip6->daddr, far_id);
        if (!(far->action & FAR_FORW))
            return XDP_DROP;
        if (!(far->outer_header_creation & OHC_GTP_U_UDP_IPv4))
            return XDP_DROP;
        struct qer_info *qer = bpf_map_lookup_elem(&qer_map, &qer_id);
        if (!qer)
        {
            upf_printk("upf n6: no downlink session qer for ip:%pI6c qer:%d",
                       &ip6->daddr, qer_id);
            return XDP_DROP;
        }
        upf_printk("upf n6: qer:%d gate_status:%d", qer_id, qer->dl_gate_status);
        if (qer->dl_gate_status != GATE_STATUS_OPEN)
            return XDP_DROP;
        const __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
        if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->dl_start,
                                                  qer->dl_maximum_bitrate))
            return XDP_DROP;
        __u8 tos = far->transport_level_marking >> 8;
        upf_printk("upf n6: use mapping %pI6c -> TEID:%d", &ip6->daddr, far->teid);
        return send_to_gtp_tunnel(ctx, far->localip, far->remoteip, tos, qer->qfi, far->teid);
    }
}

/* Dispatch based on the L3 protocol. */
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
        increment_counter(ctx->counters, rx_arp);
        upf_printk("upf n6: arp received, passing to kernel");
        return XDP_PASS;
    default:
        return DEFAULT_XDP_ACTION;
    }
}

SEC("xdp/upf_n6_entrypoint")
int upf_n6_entrypoint_func(struct xdp_md *ctx)
{
    upf_printk("upf n6 entrypoint start");
    const __u32 key = 0;
    struct upf_statistic *statistic = bpf_map_lookup_elem(&upf_ext_stat, &key);
    if (!statistic)
    {
        const struct upf_statistic initval = {0};
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
