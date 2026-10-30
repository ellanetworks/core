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
#include "bpf/utils/pdr.h"
#include "bpf/utils/sdf_classifier.h"

#define MAX_SESSION_QERS (1 + MAX_CLASSIFIER_TARGETS)
#define QER_WINDOW_MAP_SIZE (MAX_PDU_SESSIONS * MAX_SESSION_QERS)

struct qer_key {
	__u64 seid;
	__u32 qer_id;
	__u32 _pad;
};

struct qer_window {
	struct bpf_spin_lock lock;
	__u32 _pad;
	__u64 ul_start;
	__u64 dl_start;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, struct qer_key);
	__type(value, struct qer_window);
	__uint(max_entries, QER_WINDOW_MAP_SIZE);
} qer_windows SEC(".maps");

enum qer_direction {
	QER_UPLINK,
	QER_DOWNLINK,
};

enum qer_result {
	QER_ALLOWED,
	QER_RATE_LIMITED,
	QER_METER_MISSING,
};

static __always_inline __u64 qer_rate(const struct qer_info *qer,
				      enum qer_direction direction)
{
	return direction == QER_UPLINK ? qer->ul_maximum_bitrate :
					 qer->dl_maximum_bitrate;
}

static __noinline enum qer_result
limit_qer_rate_slow(__u64 seid, __u32 qer_id, const struct qer_info *qer,
		    __u64 packet_size, enum qer_direction direction)
{
	static const __u64 NSEC_PER_SEC = 1000000000ULL;
	static const __u64 NSEC_PER_MSEC = 1000000ULL;
	static const __u64 MAX_BURST_NSEC = 500 * NSEC_PER_MSEC;
	const __u64 rate = qer_rate(qer, direction);

	if (rate == 0)
		return QER_ALLOWED;

	struct qer_key key = { .seid = seid, .qer_id = qer_id, ._pad = 0 };
	struct qer_window *meter = bpf_map_lookup_elem(&qer_windows, &key);
	if (!meter)
		return QER_METER_MISSING;

	const __u64 window = qer->averaging_window_ms * NSEC_PER_MSEC;
	const __u64 window_size =
		window && window < MAX_BURST_NSEC ? window : MAX_BURST_NSEC;

	const __u64 bit_nsec = packet_size * 8 * NSEC_PER_SEC;
	const __u64 tx_time = bit_nsec / rate + (bit_nsec % rate != 0);
	const __u64 now = bpf_ktime_get_ns();
	__u64 *windows_start = direction == QER_UPLINK ?
					&meter->ul_start : &meter->dl_start;
	enum qer_result result = QER_RATE_LIMITED;

	bpf_spin_lock(&meter->lock);

	__u64 start = *windows_start;

	__u64 window_floor = now > window_size ? now - window_size : 0;
	if (start < window_floor)
		start = window_floor;

	if (start <= now) {
		__u64 next = start + tx_time;
		*windows_start = next < start ? ~0ULL : next;
		result = QER_ALLOWED;
	}

	bpf_spin_unlock(&meter->lock);
	return result;
}

static __always_inline enum qer_result
limit_qer_rate(__u64 seid, __u32 qer_id, const struct qer_info *qer,
	       __u64 packet_size, enum qer_direction direction)
{
	if (qer_rate(qer, direction) == 0)
		return QER_ALLOWED;

	return limit_qer_rate_slow(seid, qer_id, qer, packet_size, direction);
}
