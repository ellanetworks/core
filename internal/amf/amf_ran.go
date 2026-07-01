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
	RanPresent    int
	RanID         *models.GlobalRanNodeID
	Name          string
	Conn          NGAPWriter
	ConnectedAt   time.Time
	lastSeen      atomic.Int64 // Unix nanoseconds; use LastSeenAt()/TouchLastSeen()
	SupportedTAIs []SupportedTAI
	amf           *AMF // the owning AMF; its registry lock (amf.mu) guards the ranUEs index this radio's UEs live in
	Log           *zap.Logger
}

type SupportedTAI struct {
	Tai        models.Tai
	SNssaiList []models.Snssai
}

// RemoveAllUeInRan removes every RAN UE bound to this radio.
func (r *Radio) RemoveAllUeInRan(ctx context.Context) {
	r.amf.mu.RLock()

	ues := make([]*RanUe, 0)

	for _, ranUe := range r.amf.ranUEs {
		if ranUe.radio == r {
			ues = append(ues, ranUe)
		}
	}

	r.amf.mu.RUnlock()

	for _, ranUe := range ues {
		applyStatefulNasCleanup(ctx, ranUe)

		err := ranUe.Remove(ctx)
		if err != nil {
			logger.AmfLog.Error("error removing ran ue", zap.Error(err))
		}
	}
}

// applyStatefulNasCleanup runs the GMM state-machine cleanup that follows
// NAS connection loss: mid-registration UEs are aborted (TS 24.501);
// registered UEs start the mobile reachable timer.
func applyStatefulNasCleanup(ctx context.Context, ranUe *RanUe) {
	ue := ranUe.UeContext()
	if ue == nil {
		return
	}

	switch ue.State() {
	case Registered:
		ue.ResetMobileReachableTimer()
	case Authentication, SecurityMode, ContextSetup:
		ue.Deregister(ctx)
	}
}

func (r *Radio) FindUEByRanUeNgapID(ranUeNgapID int64) *RanUe {
	r.amf.mu.RLock()
	defer r.amf.mu.RUnlock()

	for _, ranUe := range r.amf.ranUEs {
		if ranUe.radio == r && ranUe.RanUeNgapID == ranUeNgapID {
			return ranUe
		}
	}

	return nil
}

// UpdateUERanNgapID records the RAN UE NGAP ID the target gNB assigned to a UE
// in HandoverRequestAcknowledge. The UE is keyed by its AMF UE NGAP ID, so only
// the field is updated.
func (r *Radio) UpdateUERanNgapID(ranUe *RanUe, newRanUeNgapID int64) {
	r.amf.mu.Lock()
	defer r.amf.mu.Unlock()

	ranUe.RanUeNgapID = newRanUeNgapID
}

func (r *Radio) FindUEByAmfUeNgapID(amfUeNgapID int64) *RanUe {
	r.amf.mu.RLock()
	defer r.amf.mu.RUnlock()

	if ranUe := r.amf.ranUEs[amfUeNgapID]; ranUe != nil && ranUe.radio == r {
		return ranUe
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

func (r *Radio) ConnectedSubscribers() []string {
	r.amf.mu.RLock()

	ues := make([]*UeContext, 0)

	for _, ranUe := range r.amf.ranUEs {
		if ranUe.radio == r && ranUe.amfUe != nil {
			ues = append(ues, ranUe.amfUe)
		}
	}

	r.amf.mu.RUnlock()

	// Read each supi through its accessor (UeContext.mu), not the raw field under
	// r.mu, so a concurrent SetSupi cannot race this scan.
	supis := make([]string, 0, len(ues))
	for _, ue := range ues {
		supi := ue.Supi()
		if supi.IsValid() && supi.IsIMSI() {
			supis = append(supis, supi.IMSI())
		}
	}

	return supis
}
