// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <stdbool.h>

#include "bpf/utils/drop_reason.h"

#define MAX_TRAFFIC_CLASSES 1024
#define TRAFFIC_CLASS_MASK (MAX_TRAFFIC_CLASSES - 1)
#define TRAFFIC_CLASS_DROP_MAP_SIZE 4096

_Static_assert((MAX_TRAFFIC_CLASSES & TRAFFIC_CLASS_MASK) == 0,
	       "MAX_TRAFFIC_CLASSES must be a power of two");

enum traffic_class_direction {
	TRAFFIC_CLASS_UPLINK = 0,
	TRAFFIC_CLASS_DOWNLINK = 1,
};

struct traffic_class_counters {
	__u64 packets;
	__u64 bytes;
};

struct traffic_class_drop_key {
	__u16 class_id;
	__u8 direction;
	__u8 reason;
};

struct traffic_class_mark {
	__u64 bytes;
	__u16 class_id;
	__u8 direction;
	__u8 bounce_reason;
	__u8 pad[4];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct traffic_class_counters);
	__uint(max_entries, 2 * MAX_TRAFFIC_CLASSES);
} traffic_class_forwarded SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct traffic_class_drop_key);
	__type(value, __u64);
	__uint(max_entries, TRAFFIC_CLASS_DROP_MAP_SIZE);
} traffic_class_drops SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct traffic_class_mark);
	__uint(max_entries, 1);
} traffic_class_marks SEC(".maps");

static __always_inline void traffic_class_forward(__u16 class_id,
						  __u8 direction, __u64 bytes)
{
	if (!class_id)
		return;

	__u32 key = ((__u32)(class_id & TRAFFIC_CLASS_MASK) << 1) |
		    (direction & 1);
	struct traffic_class_counters *c =
		bpf_map_lookup_elem(&traffic_class_forwarded, &key);
	if (!c)
		return;

	c->packets++;
	c->bytes += bytes;
}

static __always_inline void traffic_class_drop(__u16 class_id, __u8 direction,
					       enum upf_drop_reason reason)
{
	struct traffic_class_drop_key key = {
		.class_id = class_id & TRAFFIC_CLASS_MASK,
		.direction = direction & 1,
		.reason = reason & UPF_DROP_REASON_MASK,
	};
	__u64 *packets = bpf_map_lookup_elem(&traffic_class_drops, &key);
	if (packets) {
		*packets += 1;
		return;
	}

	__u64 first = 1;

	if (!bpf_map_update_elem(&traffic_class_drops, &key, &first,
				 BPF_NOEXIST))
		return;

	packets = bpf_map_lookup_elem(&traffic_class_drops, &key);
	if (packets)
		*packets += 1;
}

static __always_inline void traffic_class_mark(__u16 class_id, __u8 direction)
{
	const __u32 zero = 0;
	struct traffic_class_mark *m =
		bpf_map_lookup_elem(&traffic_class_marks, &zero);
	if (!m)
		return;

	m->class_id = class_id;
	m->direction = direction;
	m->bounce_reason = UPF_DROP_UNSPEC;
	m->bytes = 0;
}

static __always_inline __u16 traffic_class_marked(void)
{
	const __u32 zero = 0;
	struct traffic_class_mark *m =
		bpf_map_lookup_elem(&traffic_class_marks, &zero);

	return m ? m->class_id : 0;
}

static __always_inline void traffic_class_bounce(enum upf_drop_reason reason)
{
	const __u32 zero = 0;
	struct traffic_class_mark *m =
		bpf_map_lookup_elem(&traffic_class_marks, &zero);
	if (m)
		m->bounce_reason = reason;
}

static __always_inline void traffic_class_bill(__u64 bytes)
{
	const __u32 zero = 0;
	struct traffic_class_mark *m =
		bpf_map_lookup_elem(&traffic_class_marks, &zero);
	if (m)
		m->bytes = bytes;
}

__noinline __weak int traffic_class_settle(bool forwarded,
					   enum upf_drop_reason reason)
{
	const __u32 zero = 0;
	struct traffic_class_mark *m =
		bpf_map_lookup_elem(&traffic_class_marks, &zero);
	if (!m || !m->class_id)
		return 0;

	if (!forwarded && reason != UPF_DROP_NOCP_BUFFER)
		traffic_class_drop(m->class_id, m->direction, reason);
	else if (forwarded && m->bytes)
		traffic_class_forward(m->class_id, m->direction, m->bytes);
	else if (forwarded && m->bounce_reason != UPF_DROP_UNSPEC)
		traffic_class_drop(m->class_id, m->direction, m->bounce_reason);

	m->class_id = 0;
	m->bounce_reason = UPF_DROP_UNSPEC;
	m->bytes = 0;

	return 0;
}
