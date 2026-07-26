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

const natPurgeBatchSize = 4096

// purgeNATConntrack removes every nat_ct entry belonging to ueIP, in both
// directions. It runs synchronously in session teardown: the IP allocator
// hands out the lowest free address, so a released address must leave no
// conntrack state behind by the time a new session can receive it.
func (conn *SessionEngine) purgeNATConntrack(ueIP netip.Addr) {
	if conn.BpfObjects == nil || conn.BpfObjects.NatCt == nil || !ueIP.Is4() {
		return
	}

	addr4 := ueIP.As4()
	ueAddr := binary.NativeEndian.Uint32(addr4[:])

	var (
		cursor  bpf.MapBatchCursor
		keys    = make([]ebpf.N3N6EntrypointFiveTuple, natPurgeBatchSize)
		values  = make([]ebpf.N3N6EntrypointNatEntry, natPurgeBatchSize)
		matched []ebpf.N3N6EntrypointFiveTuple
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
				logger.UpfLog.Warn("NAT conntrack purge scan failed", zap.Error(err))
			}

			break
		}
	}

	if len(matched) == 0 {
		return
	}

	count, err := conn.BpfObjects.NatCt.BatchDelete(matched, &bpf.BatchOptions{})
	if err != nil {
		logger.UpfLog.Warn("NAT conntrack purge delete failed", zap.Error(err))
	}

	logger.UpfLog.Debug("Purged NAT conntrack entries for released UE address",
		zap.String("ueIP", ueIP.String()), zap.Int("count", count))
}
