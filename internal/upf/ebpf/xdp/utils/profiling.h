// Copyright 2025 Ella Networks

#pragma once

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

/*
 * Per-packet profiling infrastructure for the XDP UPF pipeline.
 *
 * Enabled by compiling with -DENABLE_PROFILING (passed via BPF_CFLAGS).
 * When disabled, all macros expand to nothing — zero overhead.
 *
 * Overhead note: each bpf_ktime_get_ns() call costs ~600–900 ns on typical
 * hardware. With all sections enabled (~15 START/END pairs per packet) the
 * total instrumentation overhead is roughly 9–14 µs per packet. This is
 * intentional and documented; do not attempt to optimise it away by reusing
 * timestamps across sections.
 *
 * Usage (BPF C side):
 *   PROFILE_START(idx);
 *   ... code to measure ...
 *   PROFILE_END(idx);
 *
 * Usage (Go userspace side):
 *   stats, err := ReadProfilingStats(bpfObjects)
 *
 * Enabling profiling:
 *   BPF_CFLAGS="-DENABLE_PROFILING" go generate ./internal/upf/ebpf/
 */

/* One entry per profiling index, stored in a per-CPU array map. */
struct profile_entry {
	__u64 total_ns; /* accumulated nanoseconds */
	__u64 count;    /* number of samples */
};

/*
 * Profiling indices — one per pipeline sub-stage, split by direction.
 * Keep this enum in sync with PROF_NUM_ENTRIES below.
 */
enum profile_index {
	PROF_N3_TOTAL        = 0,  /* N3 (uplink) full pipeline */
	PROF_N6_TOTAL        = 1,  /* N6 (downlink) full pipeline */
	PROF_N3_PDR_LOOKUP   = 2,  /* uplink TEID → PDR map lookup */
	PROF_N6_PDR_LOOKUP   = 3,  /* downlink DIP → PDR map lookup */
	PROF_N3_MTU_CHECK    = 4,  /* uplink bpf_check_mtu */
	PROF_N6_MTU_CHECK    = 5,  /* downlink bpf_check_mtu */
	PROF_N3_QER_RATELIMIT = 6, /* uplink gate + sliding-window rate limit */
	PROF_N6_QER_RATELIMIT = 7, /* downlink gate + sliding-window rate limit */
	PROF_N3_GTP_MANIP    = 8,  /* uplink GTP header update/removal */
	PROF_N6_GTP_MANIP    = 9,  /* downlink GTP header encapsulation */
	PROF_N3_SDF_FILTER   = 10, /* uplink SDF filter match */
	PROF_N6_SDF_FILTER   = 11, /* downlink SDF filter match */
	PROF_N3_NAT          = 12, /* uplink source NAT (masquerade) */
	PROF_N6_NAT          = 13, /* downlink destination NAT (masquerade) */
	PROF_N3_FIB_ROUTING  = 14, /* uplink FIB lookup + redirect */
	PROF_N6_FIB_ROUTING  = 15, /* downlink FIB lookup + redirect */
	PROF_NUM_ENTRIES     = 16,
};

#ifdef ENABLE_PROFILING

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__type(key, __u32);
	__type(value, struct profile_entry);
	__uint(max_entries, PROF_NUM_ENTRIES);
} profiling_map SEC(".maps");

/*
 * PROFILE_START(idx) — record the start timestamp into a local variable.
 * The variable name encodes the index to allow nesting without collisions.
 */
#define PROFILE_START(idx) \
	__u64 _prof_start_##idx = bpf_ktime_get_ns()

/*
 * PROFILE_END(idx) — compute elapsed ns and accumulate into the per-CPU map.
 * Silently skips the update if the map lookup fails (should never happen for
 * a PERCPU_ARRAY with a valid key, but the BPF verifier requires the check).
 */
#define PROFILE_END(idx) \
	do { \
		__u64 _prof_end_##idx = bpf_ktime_get_ns(); \
		__u32 _prof_key_##idx = (idx); \
		struct profile_entry *_prof_e_##idx = \
			bpf_map_lookup_elem(&profiling_map, &_prof_key_##idx); \
		if (_prof_e_##idx) { \
			_prof_e_##idx->total_ns += _prof_end_##idx - _prof_start_##idx; \
			_prof_e_##idx->count += 1; \
		} \
	} while (0)

#else /* !ENABLE_PROFILING */

#define PROFILE_START(idx) do {} while (0)
#define PROFILE_END(idx)   do {} while (0)

#endif /* ENABLE_PROFILING */
