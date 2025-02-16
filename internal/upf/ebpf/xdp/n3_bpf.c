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

#include "xdp/utils/n3_statistics.h"
#include "xdp/utils/qer.h"
#include "xdp/utils/n3_pdr.h"
#include "xdp/utils/n3_sdf_filter.h"

#include "xdp/utils/common.h"
#include "xdp/utils/trace.h"
#include "xdp/utils/n3_packet_context.h"
#include "xdp/utils/n3_parsers.h"
#include "xdp/utils/csum.h"
#include "xdp/utils/n3_gtp_utils.h"
#include "xdp/utils/routing.h"
#include "xdp/utils/n3_icmp.h"

#define DEFAULT_XDP_ACTION XDP_PASS

static __always_inline enum xdp_action handle_gtp_packet(struct n3_packet_context *ctx)
{
    if (!ctx->gtp)
    {
        upf_printk("upf: unexpected packet context. no gtp header");
        return DEFAULT_XDP_ACTION;
    }

    __u32 teid = bpf_htonl(ctx->gtp->teid);
    /* Lookup uplink session using the TEID */
    struct n3_pdr_info *pdr = bpf_map_lookup_elem(&n3_pdr_map_uplink_ip4, &teid);
    if (!pdr)
    {
        upf_printk("upf: no session for teid:%d", teid);
        return DEFAULT_XDP_ACTION;
    }

    __u32 far_id = pdr->far_id;
    __u32 qer_id = pdr->qer_id;
    __u8 outer_header_removal = pdr->outer_header_removal;

    /* If an SDF is configured, match it against the inner packet */
    if (pdr->sdf_mode)
    {
        struct n3_packet_context inner_context = {
            .data = (char *)(long)ctx->data,
            .data_end = (const char *)(long)ctx->data_end,
        };

        if (inner_context.data + 1 > inner_context.data_end)
            return DEFAULT_XDP_ACTION;
        int eth_protocol = guess_eth_protocol(inner_context.data);
        switch (eth_protocol)
        {
        case ETH_P_IP_BE:
        {
            int ip_protocol = n3_parse_ip4(&inner_context);
            if (-1 == ip_protocol)
            {
                upf_printk("upf: unable to parse IPv4 header");
                return DEFAULT_XDP_ACTION;
            }
            if (-1 == n3_parse_l4(ip_protocol, &inner_context))
            {
                upf_printk("upf: unable to parse L4 header");
                return DEFAULT_XDP_ACTION;
            }
            const struct n3_sdf_filter *sdf = &pdr->n3_sdf_rules.n3_sdf_filter;
            if (match_n3_sdf_filter_ipv4(&inner_context, sdf))
            {
                upf_printk("upf: sdf filter matches teid:%d", teid);
                far_id = pdr->n3_sdf_rules.far_id;
                qer_id = pdr->n3_sdf_rules.qer_id;
                outer_header_removal = pdr->n3_sdf_rules.outer_header_removal;
            }
            else
            {
                upf_printk("upf: sdf filter doesn't match teid:%d", teid);
                if (pdr->sdf_mode & 1)
                    return DEFAULT_XDP_ACTION;
            }
            break;
        }
        case ETH_P_IPV6_BE:
        {
            int ip_protocol = parse_ip6(&inner_context);
            if (ip_protocol == -1)
            {
                upf_printk("upf: unable to parse IPv6 header");
                return DEFAULT_XDP_ACTION;
            }
            if (-1 == n3_parse_l4(ip_protocol, &inner_context))
            {
                upf_printk("upf: unable to parse L4 header");
                return DEFAULT_XDP_ACTION;
            }
            const struct n3_sdf_filter *sdf = &pdr->n3_sdf_rules.n3_sdf_filter;
            if (match_n3_sdf_filter_ipv6(&inner_context, sdf))
            {
                upf_printk("upf: sdf filter matches teid:%d", teid);
                far_id = pdr->n3_sdf_rules.far_id;
                qer_id = pdr->n3_sdf_rules.qer_id;
                outer_header_removal = pdr->n3_sdf_rules.outer_header_removal;
            }
            else
            {
                upf_printk("upf: sdf filter doesn't match teid:%d", teid);
                if (pdr->sdf_mode & 1)
                    return DEFAULT_XDP_ACTION;
            }
            break;
        }
        default:
            upf_printk("upf: unsupported inner ethernet protocol: %d", eth_protocol);
            if (pdr->sdf_mode & 1)
                return DEFAULT_XDP_ACTION;
            break;
        }
    }

    /* Lookup FAR and QER (both expected to be for uplink) */
    struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
    if (!far)
    {
        upf_printk("upf: no session far for teid:%d far:%d", teid, far_id);
        return XDP_DROP;
    }
    upf_printk("upf: far:%d action:%d outer_header_creation:%d", far_id, far->action, far->outer_header_creation);
    if (!(far->action & FAR_FORW))
        return XDP_DROP;
    struct qer_info *qer = bpf_map_lookup_elem(&qer_map, &qer_id);
    if (!qer)
    {
        upf_printk("upf: no session qer for teid:%d qer:%d", teid, qer_id);
        return XDP_DROP;
    }
    upf_printk("upf: qer:%d gate_status:%d mbr:%d", qer_id, qer->ul_gate_status, qer->ul_maximum_bitrate);
    if (qer->ul_gate_status != GATE_STATUS_OPEN)
        return XDP_DROP;

    const __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
    if (XDP_DROP == limit_rate_sliding_window(packet_size, &qer->ul_start, qer->ul_maximum_bitrate))
        return XDP_DROP;

    upf_printk("upf: session for teid:%d far:%d outer_header_removal:%d", teid, pdr->far_id, outer_header_removal);
    if (far->outer_header_creation & OHC_GTP_U_UDP_IPv4)
    {
        upf_printk("upf: session for teid:%d -> %d remote:%pI4", teid, far->teid, &far->remoteip);
        update_gtp_tunnel(ctx, far->localip, far->remoteip, 0, far->teid);
    }
    else if (outer_header_removal == OHR_GTP_U_UDP_IPv4)
    {
        long result = remove_gtp_header(ctx);
        if (result)
        {
            upf_printk("upf: handle_gtp_packet: can't remove gtp header: %d", result);
            return XDP_ABORTED;
        }
    }

    /* Account uplink traffic */
    {
        struct upf_n3_statistic *statistic = bpf_map_lookup_elem(&upf_ext_stat, &(__u32){0});
        if (statistic)
        {
            __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
            statistic->upf_n3_counters.ul_bytes += packet_size;
        }
    }

    if (ctx->ip4)
        return route_ipv4(ctx->xdp_ctx, ctx->eth, ctx->ip4);
    else if (ctx->ip6)
        return route_ipv6(ctx->xdp_ctx, ctx->eth, ctx->ip6);
    else
        return XDP_ABORTED;
}

static __always_inline enum xdp_action handle_gtpu(struct n3_packet_context *ctx)
{
    int pdu_type = parse_gtp(ctx);
    switch (pdu_type)
    {
    case GTPU_G_PDU:
        return handle_gtp_packet(ctx);
    case GTPU_ECHO_REQUEST:
        upf_printk("upf: gtp echo request [ %pI4 -> %pI4 ]",
                   &ctx->ip4->saddr, &ctx->ip4->daddr);
        return handle_echo_request(ctx);
    case GTPU_ECHO_RESPONSE:
        return XDP_PASS;
    case GTPU_ERROR_INDICATION:
    case GTPU_SUPPORTED_EXTENSION_HEADERS_NOTIFICATION:
    case GTPU_END_MARKER:
        return DEFAULT_XDP_ACTION;
    default:
        upf_printk("upf: unexpected gtp message: type=%d", pdu_type);
        return DEFAULT_XDP_ACTION;
    }
}

static __always_inline enum xdp_action handle_ip4(struct n3_packet_context *ctx)
{
    int l4_protocol = n3_parse_ip4(ctx);
    if (l4_protocol == IPPROTO_UDP && GTP_UDP_PORT == parse_udp(ctx))
    {
        upf_printk("upf: gtp-u received");
        return handle_gtpu(ctx);
    }
    return DEFAULT_XDP_ACTION;
}

static __always_inline enum xdp_action handle_ip6(struct n3_packet_context *ctx)
{
    return DEFAULT_XDP_ACTION;
}

static __always_inline enum xdp_action process_packet(struct n3_packet_context *ctx)
{
    __u16 l3_protocol = parse_ethernet(ctx);
    switch (l3_protocol)
    {
    case ETH_P_IPV6:
        return handle_ip6(ctx);
    case ETH_P_IP:
        return handle_ip4(ctx);
    case ETH_P_ARP:
        upf_printk("upf: arp received. passing to kernel");
        return XDP_PASS;
    }
    return DEFAULT_XDP_ACTION;
}

SEC("xdp/upf_n3_entrypoint")
int upf_n3_entrypoint_func(struct xdp_md *ctx)
{
    const __u32 key = 0;
    struct upf_n3_statistic *statistic = bpf_map_lookup_elem(&upf_ext_stat, &key);
    if (!statistic)
    {
        const struct upf_n3_statistic initval = {};
        bpf_map_update_elem(&upf_ext_stat, &key, &initval, BPF_ANY);
        statistic = bpf_map_lookup_elem(&upf_ext_stat, &key);
        if (!statistic)
            return XDP_ABORTED;
    }

    struct n3_packet_context context = {
        .data = (char *)(long)ctx->data,
        .data_end = (const char *)(long)ctx->data_end,
        .xdp_ctx = ctx,
        .counters = &statistic->upf_n3_counters,
    };

    enum xdp_action action = process_packet(&context);
    statistic->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;

    return action;
}

char _license[] SEC("license") = "GPL";
