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

/* Include uplink-specific PDR definitions */
#include "xdp/pdr_n3.h"

#include "xdp/statistics.h"
#include "xdp/qer.h"
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

/* Forward declarations for functions used in this file */
static __always_inline enum xdp_action handle_gtpu(struct packet_context *ctx);
static __always_inline enum xdp_action handle_gtp_packet(struct packet_context *ctx);
static __always_inline enum xdp_action handle_n6_packet_ipv4(struct packet_context *ctx);

/* Process an N3 packet: verify that the packet is IPv4, UDP and that the UDP port equals GTP_UDP_PORT */
static __always_inline enum xdp_action process_n3_packet(struct packet_context *ctx)
{
    __u16 eth_proto = parse_ethernet(ctx);
    if (eth_proto != ETH_P_IP)
        return DEFAULT_XDP_ACTION;

    int ip_proto = parse_ip4(ctx);
    if (ip_proto != IPPROTO_UDP)
        return DEFAULT_XDP_ACTION;

    int udp_port = parse_udp(ctx);
    if (udp_port != GTP_UDP_PORT)
        return DEFAULT_XDP_ACTION;

    /* Count as IPv4 and UDP */
    increment_counter(ctx->counters, rx_ip4);
    increment_counter(ctx->counters, rx_udp);
    increment_counter(ctx->n3_n6_counter, rx_n3);

    /* Process GTP-U */
    return handle_gtpu(ctx);
}

static __always_inline enum xdp_action handle_gtpu(struct packet_context *ctx)
{
    int pdu_type = parse_gtp(ctx);
    switch (pdu_type)
    {
    case GTPU_G_PDU:
        increment_counter(ctx->counters, rx_gtp_pdu);
        return handle_gtp_packet(ctx);
    case GTPU_ECHO_REQUEST:
        increment_counter(ctx->counters, rx_gtp_echo);
        upf_printk("upf: gtp echo request [ %pI4 -> %pI4 ]",
                   &ctx->ip4->saddr, &ctx->ip4->daddr);
        return handle_echo_request(ctx);
    case GTPU_ECHO_RESPONSE:
        return XDP_PASS; // Pass echo response to userspace program
    case GTPU_ERROR_INDICATION:
    case GTPU_SUPPORTED_EXTENSION_HEADERS_NOTIFICATION:
    case GTPU_END_MARKER:
        increment_counter(ctx->counters, rx_gtp_other);
        return DEFAULT_XDP_ACTION;
    default:
        increment_counter(ctx->counters, rx_gtp_unexp);
        upf_printk("upf: unexpected gtp message: type=%d", pdu_type);
        return DEFAULT_XDP_ACTION;
    }
}

static __always_inline enum xdp_action handle_gtp_packet(struct packet_context *ctx)
{
    if (!ctx->gtp)
    {
        upf_printk("upf: unexpected packet context. no gtp header");
        return DEFAULT_XDP_ACTION;
    }

    /* Step 1: search for PDR and apply PDR instructions */
    __u32 teid = bpf_htonl(ctx->gtp->teid);
    struct pdr_info *pdr = bpf_map_lookup_elem(&pdr_map_uplink_ip4, &teid);
    if (!pdr)
    {
        upf_printk("upf: no session for teid:%d", teid);
        return DEFAULT_XDP_ACTION;
    }

    __u32 far_id = pdr->far_id;
    __u32 qer_id = pdr->qer_id;
    __u8 outer_header_removal = pdr->outer_header_removal;

    if (pdr->sdf_mode)
    {
        struct packet_context inner_context = {
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
            int ip_protocol = parse_ip4(&inner_context);
            if (-1 == ip_protocol)
            {
                upf_printk("upf: unable to parse IPv4 header");
                return DEFAULT_XDP_ACTION;
            }
            if (-1 == parse_l4(ip_protocol, &inner_context))
            {
                upf_printk("upf: unable to parse L4 header");
                return DEFAULT_XDP_ACTION;
            }
            const struct sdf_filter *sdf = &pdr->sdf_rules.sdf_filter;
            if (match_sdf_filter_ipv4(&inner_context, sdf))
            {
                upf_printk("upf: sdf filter matches teid:%d", teid);
                far_id = pdr->sdf_rules.far_id;
                qer_id = pdr->sdf_rules.qer_id;
                outer_header_removal = pdr->sdf_rules.outer_header_removal;
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
            if (-1 == parse_l4(ip_protocol, &inner_context))
            {
                upf_printk("upf: unable to parse L4 header");
                return DEFAULT_XDP_ACTION;
            }
            const struct sdf_filter *sdf = &pdr->sdf_rules.sdf_filter;
            if (match_sdf_filter_ipv6(&inner_context, sdf))
            {
                upf_printk("upf: sdf filter matches teid:%d", teid);
                far_id = pdr->sdf_rules.far_id;
                qer_id = pdr->sdf_rules.qer_id;
                outer_header_removal = pdr->sdf_rules.outer_header_removal;
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

    /* Step 2: search for FAR and apply FAR instructions */
    struct far_info *far = bpf_map_lookup_elem(&far_map, &far_id);
    if (!far)
    {
        upf_printk("upf: no session far for teid:%d far:%d", teid, far_id);
        return XDP_DROP;
    }
    upf_printk("upf: far:%d action:%d outer_header_creation:%d", far_id, far->action, far->outer_header_creation);
    if (!(far->action & FAR_FORW))
        return XDP_DROP;

    /* Step 3: search for QER and apply QER instructions */
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

    if (ctx->ip4 && ctx->ip4->daddr == far->localip && ctx->ip4->protocol == IPPROTO_ICMP)
    {
        upf_printk("upf: prepare icmp ping reply to request %pI4 -> %pI4", &ctx->ip4->saddr, &ctx->ip4->daddr);
        if (-1 == prepare_icmp_echo_reply(ctx, far->localip, ctx->ip4->saddr))
            return XDP_ABORTED;
        upf_printk("upf: send icmp ping reply %pI4 -> %pI4", &ctx->ip4->saddr, &ctx->ip4->daddr);
        return handle_n6_packet_ipv4(ctx);
    }

    struct upf_statistic *statistic = bpf_map_lookup_elem(&upf_ext_stat, &(__u32){0});
    if (statistic)
    {
        __u64 packet_size = ctx->xdp_ctx->data_end - ctx->xdp_ctx->data;
        statistic->upf_counters.ul_bytes += packet_size;
    }
    if (ctx->ip4)
    {
        increment_counter(ctx->n3_n6_counter, tx_n6);
        return route_ipv4(ctx->xdp_ctx, ctx->eth, ctx->ip4);
    }
    else if (ctx->ip6)
    {
        increment_counter(ctx->n3_n6_counter, tx_n6);
        return route_ipv6(ctx->xdp_ctx, ctx->eth, ctx->ip6);
    }
    else
    {
        return XDP_ABORTED;
    }
}

/* For the N3 entrypoint, we won’t use the N6-specific PDR processing.
   Provide a dummy definition for handle_n6_packet_ipv4 so the file compiles.
*/
static __always_inline enum xdp_action handle_n6_packet_ipv4(struct packet_context *ctx)
{
    return DEFAULT_XDP_ACTION;
}

SEC("xdp/upf_n3_entrypoint")
int upf_n3_entrypoint_func(struct xdp_md *ctx)
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

    enum xdp_action action = process_n3_packet(&context);
    statistic->xdp_actions[action & EUPF_MAX_XDP_ACTION_MASK] += 1;
    return action;
}

char _license[] SEC("license") = "GPL";
