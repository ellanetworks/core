// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0
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

#include "bpf/ctx/ctx.h"
#include "bpf/utils/drop_reason.h"

struct byte_counter {
	__u64 bytes;
};

struct packet_counters {
	__u64 rx;
	__u64 tx;
	/* Fragments reaching either program with no recorded ports, counted
	 * before a session claims them and whether or not the miss was
	 * fatal: reordering is what it separates from policy. */
	__u64 frag_unresolved;
};

#define UPF_MAX_ACTION 8
#define UPF_ACTION_MASK 0x07

struct upf_statistic {
	struct byte_counter byte_counter;
	struct packet_counters packet_counters;
	/* Indexed by enum ctx_action, forwarding actions only: the two arrays
	 * are disjoint and sum to the frames handled. */
	__u64 forwarded_actions[UPF_MAX_ACTION];
	/* Indexed by enum upf_drop_reason; aborts land here too. */
	__u64 drop_reasons[UPF_DROP_REASON_MAX];
};
