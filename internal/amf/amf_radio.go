// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"go.uber.org/zap"
)

const (
	RanPresentGNbID   = 1
	RanPresentNgeNbID = 2
	RanPresentN3IwfID = 3
)

// Radio represents one SCTP association to a gNB.
// All mutations happen on the single goroutine serving this connection.
// Do not access Radio fields from other goroutines without synchronization.
type Radio struct {
	RanPresent int
	RanID      *models.GlobalRanNodeID
	Conn       NGAPWriter
	// name and supportedTAIs are written through UpdateRadioName /
	// UpdateRadioSupportedTAIs under amf.mu so a concurrent status read never sees a
	// half-written value. connectedAt is set once at construction. Guarded by amf.mu.
	name          string
	supportedTAIs []SupportedTAI
	connectedAt   time.Time
	lastSeen      atomic.Int64 // Unix nanoseconds; use LastSeenAt()/TouchLastSeen()
	amf           *AMF         // its registry lock (amf.mu) guards the conns index this radio's UEs live in
	Log           *zap.Logger
}

// UpdateRadioName sets a radio's RAN node name under the registry lock, so a
// concurrent status read never observes a half-written field.
func (a *AMF) UpdateRadioName(radio *Radio, name string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	radio.name = name
}

// UpdateRadioSupportedTAIs replaces a radio's broadcast TAI list under the registry lock.
func (a *AMF) UpdateRadioSupportedTAIs(radio *Radio, tais []SupportedTAI) {
	a.mu.Lock()
	defer a.mu.Unlock()

	radio.supportedTAIs = tais
}

// SupportedTAIList returns a snapshot of the radio's broadcast TAIs under the registry
// lock, so a caller on another goroutine never reads the live slice while an NGAP
// handler replaces it.
func (r *Radio) SupportedTAIList() []SupportedTAI {
	r.amf.mu.RLock()
	defer r.amf.mu.RUnlock()

	return append([]SupportedTAI(nil), r.supportedTAIs...)
}

// NodeName returns the radio's RAN node name under the registry lock. Must not be
// called while holding amf.mu.
func (r *Radio) NodeName() string {
	r.amf.mu.RLock()
	defer r.amf.mu.RUnlock()

	return r.name
}

// RadioNameForTest, RadioSupportedTAIsForTest and RadioConnectedAtForTest read the
// encapsulated Radio fields under the registry lock for tests in other packages that
// cannot name the unexported fields.
func (a *AMF) RadioNameForTest(r *Radio) string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return r.name
}

func (a *AMF) RadioSupportedTAIsForTest(r *Radio) []SupportedTAI {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return r.supportedTAIs
}

func (a *AMF) RadioConnectedAtForTest(r *Radio) time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return r.connectedAt
}

type SupportedTAI struct {
	Tai        models.Tai
	SNssaiList []models.Snssai
}

// radioFor returns the Radio serving a connection, or nil when none is tracked.
// For metadata/status/rare paths — a UE's send path uses ueConn.conn directly.
func (a *AMF) radioFor(conn NGAPWriter) *Radio {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.radios[conn]
}

// radioNameByConn returns the node name for a connection, or "" when untracked.
func (a *AMF) radioNameByConn(conn NGAPWriter) string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if r := a.radios[conn]; r != nil {
		return r.name
	}

	return ""
}

// RadioInfo is a read-only snapshot of a connected radio, exposed for status/API so
// the live *Radio never leaves the AMF. RanNodeType is 5G-only (the NG-RAN node may
// be a gNB, ng-eNB, or N3IWF).
type RadioInfo struct {
	Name          string
	ID            string
	Address       string
	RanNodeType   string
	ConnectedAt   time.Time
	LastSeenAt    time.Time
	SupportedTAIs []SupportedTAI
}

func (r *Radio) info() RadioInfo {
	addr := AddrString(r.RemoteAddr())

	return RadioInfo{
		Name:          r.name,
		ID:            r.NodeID(),
		Address:       addr,
		RanNodeType:   r.RanNodeTypeName(),
		ConnectedAt:   r.connectedAt,
		LastSeenAt:    r.LastSeenAt(),
		SupportedTAIs: r.supportedTAIs,
	}
}

// RemoveAllUeInRan removes every RAN UE bound to radio.
func (a *AMF) RemoveAllUeInRan(ctx context.Context, radio *Radio) {
	a.mu.RLock()

	ues := make([]*UeConn, 0)

	for _, ueConn := range a.conns {
		if ueConn.conn == radio.Conn {
			ues = append(ues, ueConn)
		}
	}

	a.mu.RUnlock()

	for _, ueConn := range ues {
		applyStatefulNasCleanup(ctx, a, ueConn)

		err := a.RemoveUeConn(ctx, ueConn)
		if err != nil {
			logger.AmfLog.Error("error removing ran ue", zap.Error(err))
		}
	}
}

// RemoveUe applies the post-NAS-connection-loss GMM cleanup and removes one RAN UE
// (a partial NG Reset). It applies the same per-UE handling as RemoveAllUeInRan, so a
// partial and a whole reset leave a registered UE in the same idle-supervised state.
func (a *AMF) RemoveUe(ctx context.Context, ueConn *UeConn) error {
	applyStatefulNasCleanup(ctx, a, ueConn)

	return a.RemoveUeConn(ctx, ueConn)
}

// applyStatefulNasCleanup runs the GMM state-machine cleanup that follows
// NAS connection loss: mid-registration UEs are aborted (TS 24.501); registered
// UEs deactivate the user plane so the UPF stops sending downlink toward the lost
// RAN and buffers it for paging (TS 23.501 §5.3.3.2.4), then start the mobile
// reachable timer.
func applyStatefulNasCleanup(ctx context.Context, amf *AMF, ueConn *UeConn) {
	ue := ueConn.UeContext()
	if ue == nil {
		return
	}

	switch ue.State() {
	case Registered:
		ue.deactivateSmContexts(ctx)
		amf.StartMobileReachable(ue)
	case RegistrationInitiated, DeregistrationInitiated:
		ue.Deregister(ctx)
	}
}

func (a *AMF) FindUEByRanUeNgapID(radio *Radio, ranUeNgapID int64) *UeConn {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, ueConn := range a.conns {
		if ueConn.conn == radio.Conn && ueConn.RanUeNgapID == ranUeNgapID {
			return ueConn
		}
	}

	return nil
}

// UpdateUERanNgapID records the RAN UE NGAP ID the target gNB assigned to a UE in
// HandoverRequestAcknowledge.
func (a *AMF) UpdateUERanNgapID(ueConn *UeConn, newRanUeNgapID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ueConn.RanUeNgapID = newRanUeNgapID
}

func (a *AMF) FindUEByAmfUeNgapID(radio *Radio, amfUeNgapID int64) *UeConn {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if ueConn := a.conns[amfUeNgapID]; ueConn != nil && ueConn.conn == radio.Conn {
		return ueConn
	}

	return nil
}

func (r *Radio) TouchLastSeen() {
	r.lastSeen.Store(time.Now().UnixNano())
}

// LastSeenAt returns the last-seen timestamp. Safe for concurrent use.
func (r *Radio) LastSeenAt() time.Time {
	ns := r.lastSeen.Load()
	if ns == 0 {
		return time.Time{}
	}

	return time.Unix(0, ns)
}

// SetLastSeenAt sets the last-seen timestamp. Safe for concurrent use.
func (r *Radio) SetLastSeenAt(t time.Time) {
	r.lastSeen.Store(t.UnixNano())
}

// RemoteAddr returns the gNB's remote address, or nil for a non-SCTP writer
// (a test double).
func (r *Radio) RemoteAddr() net.Addr {
	if conn, ok := r.Conn.(*sctp.SCTPConn); ok {
		return conn.RemoteAddr()
	}

	return nil
}

// Close closes the underlying SCTP association; a no-op for a non-SCTP writer.
func (r *Radio) Close() error {
	if conn, ok := r.Conn.(*sctp.SCTPConn); ok {
		return conn.Close()
	}

	return nil
}

// NodeID returns the RAN node identifier string regardless of radio type.
func (r *Radio) NodeID() string {
	if r.RanID == nil {
		return ""
	}

	switch r.RanPresent {
	case RanPresentGNbID:
		if r.RanID.GNbID != nil {
			return r.RanID.GNbID.GNBValue
		}
	case RanPresentNgeNbID:
		return r.RanID.NgeNbID
	case RanPresentN3IwfID:
		return r.RanID.N3IwfID
	}

	return ""
}

func (r *Radio) RanNodeTypeName() string {
	switch r.RanPresent {
	case RanPresentGNbID:
		return "gNB"
	case RanPresentNgeNbID:
		return "ng-eNB"
	case RanPresentN3IwfID:
		return "N3IWF"
	default:
		return "Unknown"
	}
}
