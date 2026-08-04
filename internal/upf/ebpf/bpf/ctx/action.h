// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>

/* The datapath's own vocabulary, not a hook verdict: TC has no verdict for an
 * abort, nor for a transmit back out the ingress interface. Programs convert
 * once, at their return boundary (ctx_verdict), which the XDP encoding makes
 * the identity on XDP. */
enum ctx_action {
	CTX_ACT_ABORTED = XDP_ABORTED,
	CTX_ACT_DROP = XDP_DROP,
	CTX_ACT_OK = XDP_PASS,
	CTX_ACT_TX = XDP_TX,
	CTX_ACT_REDIRECT = XDP_REDIRECT,
};
