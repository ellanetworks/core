// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>

/* Context shim: the datapath compiles from one source into two objects, native
 * XDP (default) and TCX ingress (-DCTX_TC). Program logic goes through the
 * accessors below; only the variant headers name a helper that exists for a
 * single program type. */

/* What the datapath decided to do with a frame. This is the datapath's own
 * vocabulary, not a hook verdict: the two hooks encode verdicts differently
 * and TC cannot express half of these — it has no abort and no transmit-back,
 * so a program that spoke TC's vocabulary would report aborts as drops and
 * hairpins as redirects. Every program returns one of these and converts to
 * the hook's verdict exactly once, at its return boundary (ctx_verdict), and
 * the statistics are indexed by the action itself.
 *
 * The values are the XDP verdict encoding, so ctx_verdict is the identity in
 * the XDP object and the statistics index needs no translation in either. */
enum ctx_action {
	CTX_ACT_ABORTED = XDP_ABORTED,
	CTX_ACT_DROP = XDP_DROP,
	CTX_ACT_OK = XDP_PASS,
	CTX_ACT_TX = XDP_TX,
	CTX_ACT_REDIRECT = XDP_REDIRECT,
};

#ifdef CTX_TC
#include "bpf/ctx/skb.h"
#else
#include "bpf/ctx/xdp.h"
#endif
