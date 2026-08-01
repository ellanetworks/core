// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "bpf/ctx/action.h"

/* The datapath compiles from one source into two objects, native XDP
 * (default) and TCX ingress (-DCTX_TC); only the variant headers name a
 * helper that exists for a single program type. */

#ifdef CTX_TC
#include "bpf/ctx/skb.h"
#else
#include "bpf/ctx/xdp.h"
#endif
