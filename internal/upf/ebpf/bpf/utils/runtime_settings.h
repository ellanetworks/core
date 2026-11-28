// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <linux/bpf.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>

struct runtime_settings {
	__u32 buffer_veth_ifindex;
	__u32 reasm_veth_ifindex;
	__u32 frag_tx_enabled;
};

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, __u32);
	__type(value, struct runtime_settings);
	__uint(max_entries, 1);
} runtime_settings_map SEC(".maps");

static const __u32 runtime_settings_key = 0;

static __always_inline const struct runtime_settings *runtime_settings(void)
{
	return bpf_map_lookup_elem(&runtime_settings_map, &runtime_settings_key);
}

__noinline __weak __u32 buffer_veth_ifindex(void)
{
	const struct runtime_settings *s = runtime_settings();

	return s ? s->buffer_veth_ifindex : 0;
}

__noinline __weak __u32 reasm_veth_ifindex(void)
{
	const struct runtime_settings *s = runtime_settings();

	return s ? s->reasm_veth_ifindex : 0;
}

__noinline __weak __u32 frag_tx_enabled(void)
{
	const struct runtime_settings *s = runtime_settings();

	return s ? s->frag_tx_enabled : 0;
}
