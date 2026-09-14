/**
 * SPDX-FileCopyrightText: Ella Networks Inc.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 * 
 *     http://www.apache.org/licenses/LICENSE-2.0
 * 
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#pragma once

#include <stdbool.h>
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#include "bpf/utils/common.h"
#include "bpf/utils/gtpu.h"
#include "bpf/utils/packet_context.h"
#include "bpf/utils/ringbuf_lost.h"

struct error_indication {
	__u32 teid;
	__u8 peer_addr[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(key, 0);
	__uint(value, 0);
	__uint(max_entries, 16384);
} error_ind_map SEC(".maps");

#define GTPU_ERROR_IND_MAX_IES 6
#define GTPU_ERROR_IND_MAX_IE_LEN 64

__noinline __weak int gtpu_parse_error_indication(struct __ctx_buff *ctx_buff,
						  __u32 ie_off, __u32 ie_len,
						  struct error_indication *out)
{
	if (!out)
		return -1;

	out->teid = 0;
	__builtin_memset(out->peer_addr, 0, sizeof(out->peer_addr));

	__u32 off = 0;
	bool have_teid = false;
	bool have_peer = false;

#pragma clang loop unroll(disable)
	for (int i = 0; i < GTPU_ERROR_IND_MAX_IES; i++) {
		if (have_teid && have_peer)
			break;

		if (off + 1 > ie_len)
			break;

		__u8 type;

		if (ctx_load_bytes(ctx_buff, ie_off + off, &type,
				   sizeof(type)) < 0)
			break;

		if (type == GTPU_IE_TEID_DATA_I) {
			__be32 teid;

			if (off + 1 + sizeof(teid) > ie_len)
				break;

			if (ctx_load_bytes(ctx_buff, ie_off + off + 1, &teid,
					   sizeof(teid)) < 0)
				break;

			out->teid = bpf_htonl(teid);
			have_teid = true;
			off += 1 + sizeof(teid);

			continue;
		}

		if (type == GTPU_IE_RECOVERY) {
			if (off + 2 > ie_len)
				break;

			off += 2;

			continue;
		}

		if (!(type & GTPU_IE_TLV_FLAG))
			break;

		__u8 len_be[2];

		if (off + 3 > ie_len)
			break;

		if (ctx_load_bytes(ctx_buff, ie_off + off + 1, len_be,
				   sizeof(len_be)) < 0)
			break;

		__u32 len = ((__u32)len_be[0] << 8) | len_be[1];

		if (len > GTPU_ERROR_IND_MAX_IE_LEN || off + 3 + len > ie_len)
			break;

		if (type == GTPU_IE_PEER_ADDRESS && len == 4) {
			__u8 v4[4];

			if (ctx_load_bytes(ctx_buff, ie_off + off + 3, v4,
					   sizeof(v4)) < 0)
				break;

			out->peer_addr[10] = 0xff;
			out->peer_addr[11] = 0xff;
			__builtin_memcpy(out->peer_addr + 12, v4, sizeof(v4));
			have_peer = true;
		} else if (type == GTPU_IE_PEER_ADDRESS && len == 16) {
			__u8 v6[16];

			if (ctx_load_bytes(ctx_buff, ie_off + off + 3, v6,
					   sizeof(v6)) < 0)
				break;

			__builtin_memcpy(out->peer_addr, v6, sizeof(v6));
			have_peer = true;
		}

		off += 3 + len;
	}

	if (!have_teid || !have_peer)
		return -1;

	return 0;
}

static __always_inline enum ctx_action
handle_error_indication(struct packet_context *ctx)
{
	if (!ctx->gtp || ctx->gtp_hdr_len < sizeof(struct gtpuhdr))
		return DEFAULT_CTX_ACTION;

	__u32 payload_len = bounded_u16(bpf_ntohs(ctx->gtp->message_length));
	__u32 consumed = ctx->gtp_hdr_len - sizeof(struct gtpuhdr);

	if (payload_len < consumed)
		return DEFAULT_CTX_ACTION;

	struct error_indication ev = {};

	__u32 ie_off =
		ctx_frame_offset(ctx->ctx_buff, ctx->gtp) + ctx->gtp_hdr_len;

	if (gtpu_parse_error_indication(ctx->ctx_buff, ie_off,
					payload_len - consumed, &ev) < 0)
		return DEFAULT_CTX_ACTION;

	ringbuf_submit(&error_ind_map, &ev, sizeof(ev), RINGBUF_ERROR_IND);

	return DEFAULT_CTX_ACTION;
}
