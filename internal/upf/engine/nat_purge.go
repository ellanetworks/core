// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"sync"

	bpf "github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

const (
	natPurgeBatchSize = 4096
	natPurgeAttempts  = 3
)

type natPurger struct {
	mu      sync.Mutex
	running bool
	pending map[uint32][]chan struct{}
}

func (p *natPurger) purge(addr uint32, purgeSet func(map[uint32]struct{})) {
	done := make(chan struct{})

	p.mu.Lock()

	if p.pending == nil {
		p.pending = make(map[uint32][]chan struct{})
	}

	p.pending[addr] = append(p.pending[addr], done)

	if p.running {
		p.mu.Unlock()
		<-done

		return
	}

	p.running = true

	for len(p.pending) > 0 {
		batch := p.pending
		p.pending = make(map[uint32][]chan struct{})
		p.mu.Unlock()

		addrs := make(map[uint32]struct{}, len(batch))
		for a := range batch {
			addrs[a] = struct{}{}
		}

		purgeSet(addrs)

		for _, waiters := range batch {
			for _, w := range waiters {
				close(w)
			}
		}

		p.mu.Lock()
	}

	p.running = false
	p.mu.Unlock()
}

// purgeNATConntrack removes every nat_ct entry belonging to ueIP; the IP
// allocator reissues the lowest free address, so no state may outlive the
// session.
func (conn *SessionEngine) purgeNATConntrack(ueIP netip.Addr) {
	if conn.BpfObjects == nil || conn.BpfObjects.NatCt == nil || !ueIP.Is4() {
		return
	}

	addr4 := ueIP.As4()

	conn.natPurge.purge(binary.NativeEndian.Uint32(addr4[:]), conn.purgeNATAddresses)
}

func (conn *SessionEngine) purgeNATAddresses(addrs map[uint32]struct{}) {
	var (
		keys   = make([]ebpf.N3N6EntrypointFiveTuple, natPurgeBatchSize)
		values = make([]ebpf.N3N6EntrypointNatEntry, natPurgeBatchSize)
	)

	// Each round rescans so a partial scan or delete is retried.
	for attempt := 1; ; attempt++ {
		var (
			cursor  bpf.MapBatchCursor
			matched []ebpf.N3N6EntrypointFiveTuple
			scanErr error
		)

		for {
			n, err := conn.BpfObjects.NatCt.BatchLookup(&cursor, keys, values, nil)

			for i := 0; i < n; i++ {
				_, outbound := addrs[keys[i].Saddr]
				_, inbound := addrs[values[i].Peer.Saddr]

				if outbound || inbound {
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

		if len(matched) > 0 {
			count, err := conn.BpfObjects.NatCt.BatchDelete(matched, &bpf.BatchOptions{})
			if err != nil {
				logger.UpfLog.Warn("NAT conntrack purge delete failed", zap.Error(err))
			}

			logger.UpfLog.Debug("Purged NAT conntrack entries for released UE addresses",
				zap.Int("addresses", len(addrs)), zap.Int("count", count))

			// A complete scan whose every match was deleted needs
			// no confirming pass.
			if scanErr == nil && count == len(matched) {
				return
			}
		}

		if attempt == natPurgeAttempts {
			if scanErr != nil || len(matched) > 0 {
				logger.UpfLog.Error("NAT conntrack purge unconfirmed; a reallocation of these addresses may inherit their flows",
					zap.Int("addresses", len(addrs)), zap.Int("last_round", len(matched)), zap.Error(scanErr))
			}

			return
		}
	}
}
