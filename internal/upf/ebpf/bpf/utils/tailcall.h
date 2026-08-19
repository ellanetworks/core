// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#include "bpf/utils/common.h"
#include "bpf/utils/packet_context.h"

/* Stage programs, populated at load. */
#define UPF_CALL_UPLINK 0
#define UPF_CALL_DOWNLINK 1
#define UPF_CALL_GTPU_CONTROL 2
#define UPF_CALL_LOCAL_SWITCH 3

struct {
	__uint(type, BPF_MAP_TYPE_PROG_ARRAY);
	__type(key, __u32);
	__type(value, __u32);
	__uint(max_entries, 5);
} upf_calls SEC(".maps");

/* Returns only if the tail call fails, i.e. the stage program is missing. */
static __always_inline enum ctx_action
gtpu_control_tail_call(struct packet_context *ctx)
{
	bpf_tail_call(ctx->ctx_buff, &upf_calls, UPF_CALL_GTPU_CONTROL);

	return DEFAULT_CTX_ACTION;
}

static __always_inline enum ctx_action
local_switch_tail_call(struct packet_context *ctx)
{
	bpf_tail_call(ctx->ctx_buff, &upf_calls, UPF_CALL_LOCAL_SWITCH);

	return DEFAULT_CTX_ACTION;
}
