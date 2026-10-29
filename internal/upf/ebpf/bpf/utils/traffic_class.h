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
#define RADIO_DROP_MAP_SIZE 4096
#define MAX_RADIOS 1024
#define RADIO_MASK (MAX_RADIOS - 1)

_Static_assert((MAX_TRAFFIC_CLASSES & TRAFFIC_CLASS_MASK) == 0,
	       "MAX_TRAFFIC_CLASSES must be a power of two");
_Static_assert((MAX_RADIOS & RADIO_MASK) == 0,
	       "MAX_RADIOS must be a power of two");

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

struct radio_drop_key {
	__u16 radio_id;
	__u8 direction;
	__u8 reason;
};

struct traffic_class_mark {
	__u32 forwarded_key;
	__u32 radio_key;
	__u16 class_id;
	__u16 radio_id;
	__u8 direction;
	__u8 pad[3];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct traffic_class_counters);
	__uint(max_entries, 2 * MAX_TRAFFIC_CLASSES);
} traffic_class_forwarded SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct traffic_class_counters);
	__uint(max_entries, 2 * MAX_RADIOS);
} radio_forwarded SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct traffic_class_drop_key);
	__type(value, __u64);
	__uint(max_entries, TRAFFIC_CLASS_DROP_MAP_SIZE);
} traffic_class_drops SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_HASH);
	__type(key, struct radio_drop_key);
	__type(value, __u64);
	__uint(max_entries, RADIO_DROP_MAP_SIZE);
} radio_drops SEC(".maps");

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

static __always_inline void radio_forward(__u16 radio_id, __u8 direction,
					  __u64 bytes)
{
	if (!radio_id)
		return;

	__u32 key = ((__u32)(radio_id & RADIO_MASK) << 1) | (direction & 1);
	struct traffic_class_counters *c =
		bpf_map_lookup_elem(&radio_forwarded, &key);
	if (!c)
		return;

	c->packets++;
	c->bytes += bytes;
}

static __always_inline void drop_counter_add(void *map, const void *key)
{
	__u64 *packets = bpf_map_lookup_elem(map, key);
	if (packets) {
		*packets += 1;
		return;
	}

	__u64 first = 1;

	if (!bpf_map_update_elem(map, key, &first, BPF_NOEXIST))
		return;

	packets = bpf_map_lookup_elem(map, key);
	if (packets)
		*packets += 1;
}

static __always_inline struct traffic_class_mark *traffic_class_current(void)
{
	const __u32 zero = 0;

	return bpf_map_lookup_elem(&traffic_class_marks, &zero);
}

static __always_inline void traffic_class_mark(__u16 class_id, __u16 radio_id,
					       __u8 direction)
{
	struct traffic_class_mark *m = traffic_class_current();
	if (!m)
		return;

	m->class_id = class_id;
	m->radio_id = radio_id;
	m->direction = direction;
	m->forwarded_key = ((__u32)(class_id & TRAFFIC_CLASS_MASK) << 1) |
			   (direction & 1);
	m->radio_key = ((__u32)(radio_id & RADIO_MASK) << 1) | (direction & 1);
}

static __always_inline void traffic_class_reclass(__u16 class_id)
{
	struct traffic_class_mark *m = traffic_class_current();
	if (!m)
		return;

	m->class_id = class_id;
	m->forwarded_key = ((__u32)(class_id & TRAFFIC_CLASS_MASK) << 1) |
			   (m->direction & 1);
}

static __always_inline __u16 traffic_class_marked(void)
{
	struct traffic_class_mark *m = traffic_class_current();

	return m ? m->class_id : 0;
}

static __always_inline void traffic_class_bill(__u64 bytes)
{
	struct traffic_class_mark *m = traffic_class_current();
	if (!m)
		return;

	struct traffic_class_counters *c;

	if (m->class_id) {
		c = bpf_map_lookup_elem(&traffic_class_forwarded,
					&m->forwarded_key);
		if (c) {
			c->packets++;
			c->bytes += bytes;
		}
	}

	if (m->radio_id) {
		c = bpf_map_lookup_elem(&radio_forwarded, &m->radio_key);
		if (c) {
			c->packets++;
			c->bytes += bytes;
		}
	}

	m->class_id = 0;
	m->radio_id = 0;
}

static __always_inline void traffic_class_release(void)
{
	struct traffic_class_mark *m = traffic_class_current();
	if (m) {
		m->class_id = 0;
		m->radio_id = 0;
	}
}

__noinline __weak int traffic_class_settle(enum upf_drop_reason reason)
{
	struct traffic_class_mark *m = traffic_class_current();
	if (!m || !(m->class_id | m->radio_id))
		return 0;

	if (reason != UPF_DROP_NOCP_BUFFER && m->class_id) {
		struct traffic_class_drop_key key = {
			.class_id = m->class_id & TRAFFIC_CLASS_MASK,
			.direction = m->direction & 1,
			.reason = reason & UPF_DROP_REASON_MASK,
		};

		drop_counter_add(&traffic_class_drops, &key);
	}

	if (reason != UPF_DROP_NOCP_BUFFER && m->radio_id) {
		struct radio_drop_key key = {
			.radio_id = m->radio_id & RADIO_MASK,
			.direction = m->direction & 1,
			.reason = reason & UPF_DROP_REASON_MASK,
		};

		drop_counter_add(&radio_drops, &key);
	}

	m->class_id = 0;
	m->radio_id = 0;

	return 0;
}
