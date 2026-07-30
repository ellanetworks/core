// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

/* Context shim: the datapath compiles from one source into two objects, native
 * XDP (default) and TCX ingress (-DCTX_TC). Program logic goes through the
 * accessors below; only the variant headers name a helper that exists for a
 * single program type. */

#ifdef CTX_TC
#include "xdp/utils/ctx_skb.h"
#else
#include "xdp/utils/ctx_xdp.h"
#endif
