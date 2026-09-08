// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

enum ringbuf_id {
	RINGBUF_NOCP = 0,
	RINGBUF_RS_EVENT = 1,
	RINGBUF_NO_NEIGH = 2,
	RINGBUF_ID_MAX = 3,
};

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, __u64);
	__uint(max_entries, RINGBUF_ID_MAX);
} ringbuf_lost SEC(".maps");

static __always_inline void ringbuf_submit(void *ringbuf, void *data,
					   __u64 size, __u32 id)
{
	if (bpf_ringbuf_output(ringbuf, data, size, 0) == 0)
		return;

	__u64 *lost = bpf_map_lookup_elem(&ringbuf_lost, &id);
	if (lost)
		*lost += 1;
}
