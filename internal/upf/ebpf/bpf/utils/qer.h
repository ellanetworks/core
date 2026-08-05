/**
 * Copyright 2023 Edgecom LLC
 * SPDX-FileCopyrightText: Ella Networks Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Modified by Ella Networks.
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

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include "bpf/ctx/ctx.h"
#include "bpf/utils/pdr.h"

/* Per QER: a dual-stack session has two downlink PDRs sharing one AMBR.
 *
 * LRU because the window is derived state: no ordering against session setup,
 * no leak, and an eviction costs at most one window of burst. */
#define QER_WINDOW_MAP_SIZE (2 * MAX_PDU_SESSIONS)

struct qer_key {
	__u64 seid;
	__u32 qer_id;
	__u32 _pad;
};

struct qer_window {
	volatile __u64 ul_start;
	volatile __u64 dl_start;
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, struct qer_key);
	__type(value, struct qer_window);
	__uint(max_entries, QER_WINDOW_MAP_SIZE);
} qer_windows SEC(".maps");

/* NULL when the map is full, which leaves the packet unlimited. */
static __always_inline struct qer_window *qer_window_for(__u64 seid,
							 __u32 qer_id)
{
	struct qer_key key = { .seid = seid, .qer_id = qer_id, ._pad = 0 };

	struct qer_window *window = bpf_map_lookup_elem(&qer_windows, &key);
	if (window)
		return window;

	struct qer_window fresh = {};

	bpf_map_update_elem(&qer_windows, &key, &fresh, BPF_NOEXIST);

	return bpf_map_lookup_elem(&qer_windows, &key);
}

static __always_inline enum ctx_action
limit_rate_sliding_window(const __u64 packet_size,
			  volatile __u64 *windows_start, const __u64 rate)
{
	static const __u64 NSEC_PER_SEC = 1000000000ULL;
	static const __u64 window_size = 5000000ULL;

	/* Currently 0 rate means that traffic rate is not limited */
	if (rate == 0)
		return CTX_ACT_OK;

	__u64 tx_time = packet_size * 8 * NSEC_PER_SEC / rate;
	__u64 now = bpf_ktime_get_ns();

	/* When the link finishes transmitting what it has been charged for. */
	__u64 start = *(volatile __u64 *)windows_start;

	/* Cap accrued credit at one window, bounding a burst after silence. Also
	 * recovers a start a PFCP re-put zeroed. */
	__u64 window_floor = now > window_size ? now - window_size : 0;
	if (start < window_floor)
		start = window_floor;

	/* Charged once, below: charging here too under-delivers the rate once
	 * tx_time exceeds the window. */
	if (start > now)
		return CTX_ACT_DROP;

	/* A backlogged sender is admitted every tx_time. */
	*(volatile __u64 *)windows_start = start + tx_time;

	return CTX_ACT_OK;
}
