// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "bpf/utils/urr.h"

struct urr_first_packet_args {
	__u64 seid;
	__u32 urr_id;
	__u32 nr_cpus;
	__u64 first_packet_ns;
};

struct urr_first_packet_scan {
	struct urr_key key;
	__u64 first_packet_ns;
};

static long urr_take_first_packet_on_cpu(__u64 cpu, void *data)
{
	struct urr_first_packet_scan *scan = data;
	struct urr_counters *counters =
		bpf_map_lookup_percpu_elem(&urr_map, &scan->key, (__u32)cpu);
	if (!counters)
		return 0;

	const __u64 first =
		__sync_lock_test_and_set(&counters->first_packet_ns, 0);
	if (first && (!scan->first_packet_ns || first < scan->first_packet_ns))
		scan->first_packet_ns = first;

	return 0;
}

static __always_inline int
urr_take_first_packet(struct urr_first_packet_args *args)
{
	struct urr_first_packet_scan scan = {
		.key = { .seid = args->seid, .urr_id = args->urr_id },
	};

	if (bpf_loop(args->nr_cpus, urr_take_first_packet_on_cpu, &scan, 0) < 0)
		return -1;

	args->first_packet_ns = scan.first_packet_ns;

	return 0;
}

SEC("syscall")
int urr_first_packet_func(struct urr_first_packet_args *args)
{
	return urr_take_first_packet(args);
}

char _license[] SEC("license") = "GPL";
