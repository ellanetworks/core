// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include "bpf/ctx/action.h"

/* One source, two objects: native XDP (default) and TCX ingress (-DCTX_TC).
 * Only the variant headers name helpers tied to a single program type. */

#ifdef CTX_TC
#include "bpf/ctx/skb.h"
#else
#include "bpf/ctx/xdp.h"
#endif
