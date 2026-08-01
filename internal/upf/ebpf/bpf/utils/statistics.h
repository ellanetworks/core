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
};

#define UPF_MAX_ACTION 8
#define UPF_ACTION_MASK 0x07

struct upf_statistic {
	struct byte_counter byte_counter;
	struct packet_counters packet_counters;
	/* Frames the datapath forwarded, indexed by enum ctx_action. Only the
	 * forwarding actions are counted here — a dropped frame goes to
	 * drop_reasons instead, so every frame lands in exactly one cell of
	 * exactly one of the two arrays and the totals reconcile by
	 * construction.
	 *
	 * Indexed by the datapath's action, not the hook's verdict: TC has no
	 * verdict distinguishing a hairpin transmit from a redirect, so a
	 * verdict-keyed counter would report no transmits at all under TCX. */
	__u64 forwarded_actions[UPF_MAX_ACTION];
	/* Frames the datapath did not forward, indexed by enum upf_drop_reason.
	 * Aborts land here too, under their UPF_DROP_INTERNAL_* reason: an
	 * abort is a drop whose cause is the datapath itself. */
	__u64 drop_reasons[UPF_DROP_REASON_MAX];
};
