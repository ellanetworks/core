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
