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

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include "xdp/utils/pdr.h"

static __always_inline enum xdp_action
limit_rate_sliding_window(const __u64 packet_size,
			  volatile __u64 *windows_start, const __u64 rate)
{
	static const __u64 NSEC_PER_SEC = 1000000000ULL;
	static const __u64 window_size = 5000000ULL;

	/* Currently 0 rate means that traffic rate is not limited */
	if (rate == 0)
		return XDP_PASS;

	__u64 tx_time = packet_size * 8 * NSEC_PER_SEC / rate;
	__u64 now = bpf_ktime_get_ns();

	__u64 start = *(volatile __u64 *)windows_start;
	if (start + tx_time > now)
		return XDP_DROP;

	if (start + window_size < now) {
		*(volatile __u64 *)windows_start = now - window_size + tx_time;
		return XDP_PASS;
	}

	*(volatile __u64 *)windows_start = start + tx_time;
	//__sync_fetch_and_add(&window->start, tx_time);
	return XDP_PASS;
}
