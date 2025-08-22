// Copyright 2024 Ella Networks
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

struct upf_counters {
	__u64 bytes;
};

struct counters {
	__u64 rx;
	__u64 tx;
};

#define EUPF_MAX_XDP_ACTION 8
#define EUPF_MAX_XDP_ACTION_MASK 0x07

struct upf_statistic {
	struct upf_counters upf_counters;
	struct counters upf_counter;
	__u64 xdp_actions[EUPF_MAX_XDP_ACTION];
};
