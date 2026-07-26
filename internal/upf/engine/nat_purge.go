// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"encoding/binary"
	"errors"
	"net/netip"

	bpf "github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

const (
	natPurgeBatchSize = 4096
	natPurgeAttempts  = 3
)

// purgeNATConntrack removes every nat_ct entry belonging to ueIP. The IP
// allocator hands out the lowest free address, so a released address must
// leave no conntrack state behind by the time it can be reallocated.
func (conn *SessionEngine) purgeNATConntrack(ueIP netip.Addr) {
	if conn.BpfObjects == nil || conn.BpfObjects.NatCt == nil || !ueIP.Is4() {
		return
	}

	addr4 := ueIP.As4()
	ueAddr := binary.NativeEndian.Uint32(addr4[:])

	var (
		keys   = make([]ebpf.N3N6EntrypointFiveTuple, natPurgeBatchSize)
		values = make([]ebpf.N3N6EntrypointNatEntry, natPurgeBatchSize)
	)

	// Each round rescans, so a partial scan or delete is retried rather than
	// leaving state the next holder of this address would inherit.
	for attempt := 1; ; attempt++ {
		var (
			cursor  bpf.MapBatchCursor
			matched []ebpf.N3N6EntrypointFiveTuple
			scanErr error
		)

		for {
			n, err := conn.BpfObjects.NatCt.BatchLookup(&cursor, keys, values, nil)

			for i := 0; i < n; i++ {
				if keys[i].Saddr == ueAddr || values[i].Src.Saddr == ueAddr {
					matched = append(matched, keys[i])
				}
			}

			if err != nil {
				if !errors.Is(err, bpf.ErrKeyNotExist) {
					scanErr = err
				}

				break
			}
		}

		if scanErr == nil && len(matched) == 0 {
			return
		}

		if attempt == natPurgeAttempts {
			logger.UpfLog.Error("NAT conntrack purge incomplete; a reallocation of this address may inherit its flows",
				zap.String("ueIP", ueIP.String()), zap.Int("remaining", len(matched)), zap.Error(scanErr))

			return
		}

		if len(matched) == 0 {
			continue
		}

		count, err := conn.BpfObjects.NatCt.BatchDelete(matched, &bpf.BatchOptions{})
		if err != nil {
			logger.UpfLog.Warn("NAT conntrack purge delete failed", zap.Error(err))
		}

		logger.UpfLog.Debug("Purged NAT conntrack entries for released UE address",
			zap.String("ueIP", ueIP.String()), zap.Int("count", count))
	}
}
