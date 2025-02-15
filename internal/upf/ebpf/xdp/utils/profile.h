#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

/* Extended enum for all instrumentation steps */
enum profile_step
{
    STEP_UPF_IP_ENTRYPOINT,
    STEP_PROCESS_PACKET,
    STEP_PARSE_ETHERNET,
    STEP_HANDLE_IP4,
    STEP_HANDLE_IP6,
    STEP_HANDLE_GTPU,
    STEP_HANDLE_GTP_PACKET,
    STEP_HANDLE_N6_PACKET_IP4,
    STEP_HANDLE_N6_PACKET_IP6,
    STEP_SEND_TO_GTP_TUNNEL,
    /* New routing-related steps */
    STEP_ROUTE_IPV4_LOOKUP,
    STEP_ROUTE_IPV4_PROCESS,
    STEP_ROUTE_IPV4,
    STEP_ROUTE_IPV6_LOOKUP,
    STEP_ROUTE_IPV6_PROCESS,
    STEP_ROUTE_IPV6,
    NUM_PROFILE_STEPS,
};

/* Structure to hold instrumentation data */
struct profile_info
{
    __u64 count;
    __u64 total_ns;
};

/* A per-CPU array map for profile data */
struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, NUM_PROFILE_STEPS);
    __type(key, __u32);
    __type(value, struct profile_info);
} profile_map SEC(".maps");

/* Helper to update the profile data */
static __always_inline void update_profile(__u32 step, __u64 delta)
{
    struct profile_info *info = bpf_map_lookup_elem(&profile_map, &step);
    if (info)
    {
        info->count += 1;
        info->total_ns += delta;
    }
}
