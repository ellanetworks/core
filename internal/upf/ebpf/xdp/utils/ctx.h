// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

/* Context shim: the datapath compiles from one source into two objects, native
 * XDP (default) and TCX ingress (-DCTX_TC). Program logic goes through the
 * accessors below; only the variant headers name a helper that exists for a
 * single program type. */

/* Bytes made linear and writable ahead of parsing and direct header writes.
 * Covers the deepest write: Ethernet (14) + VLAN (4) + IPv4 with options (60)
 * + TCP with options (60). Reads and writes past this bound need
 * ctx_load_bytes or another ctx_pull. */
#define CTX_PULL_LEN 192

#ifdef CTX_TC
#include "xdp/utils/ctx_skb.h"
#else
#include "xdp/utils/ctx_xdp.h"
#endif
