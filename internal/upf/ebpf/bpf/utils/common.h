// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "bpf/ctx/ctx.h"

#define increment_counter(OBJECT, COUNTER) OBJECT->COUNTER++;

#define increment_counter_sync(OBJECT, COUNTER) \
	__sync_fetch_and_add(&OBJECT->COUNTER, 1);

#define DEFAULT_CTX_ACTION CTX_ACT_OK

/* Opaque to the optimizer, so a mask applied afterwards survives codegen and
 * the verifier can derive bounds the compiler considers already known. */
#define barrier_var(var) asm volatile("" : "+r"(var))

static __always_inline __u32 ctx_frame_offset(struct __ctx_buff *ctx,
					      const void *hdr)
{
	return (__u32)((const __u8 *)hdr - (const __u8 *)ctx_data(ctx));
}

/* A byte-swapped header field reaches the verifier as an unbounded scalar, and
 * packet-pointer arithmetic on one is rejected. The mask restores the bound
 * the field's width already guarantees. */
static __always_inline __u32 bounded_u16(__u32 value)
{
	barrier_var(value);

	return value & 0xFFFF;
}
