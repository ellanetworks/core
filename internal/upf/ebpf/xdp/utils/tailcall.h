// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "xdp/utils/common.h"
#include "xdp/utils/packet_context.h"

/* Stage programs, populated at load. Each tail call starts a fresh program
 * with its own instruction and stack budgets, which is what keeps the
 * forwarding stages small enough to verify. */
#define UPF_CALL_UPLINK 0
#define UPF_CALL_DOWNLINK 1
#define UPF_CALL_GTPU_CONTROL 2

struct {
	__uint(type, BPF_MAP_TYPE_PROG_ARRAY);
	__type(key, __u32);
	__type(value, __u32);
	__uint(max_entries, 4);
} upf_calls SEC(".maps");

/* Hands a GTP-U message the UPF answers itself to the control stage. Returns
 * only if the tail call fails, which means the stage program is missing. */
static __always_inline enum xdp_action
gtpu_control_tail_call(struct packet_context *ctx)
{
	bpf_tail_call(ctx->xdp_ctx, &upf_calls, UPF_CALL_GTPU_CONTROL);

	return DEFAULT_XDP_ACTION;
}
