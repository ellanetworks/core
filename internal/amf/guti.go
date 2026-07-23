// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

// releaseTmsisLocked unindexes and frees both the UE's current and in-flight old
// 5G-TMSI, returning them to the allocator. Caller holds amf.mu.
func (amf *AMF) releaseTmsisLocked(ue *UeContext) {
	if ue.tmsi != etsi.InvalidTMSI {
		delete(amf.uesByTmsi, ue.tmsi)
		amf.freeTmsiLocked(ue.tmsi)
	}

	if ue.oldTmsi != etsi.InvalidTMSI {
		delete(amf.uesByTmsi, ue.oldTmsi)
		amf.freeTmsiLocked(ue.oldTmsi)
	}
}

// freeTmsiLocked returns a 5G-TMSI to the allocator for reuse. Caller holds amf.mu.
func (amf *AMF) freeTmsiLocked(t etsi.TMSI) {
	amf.tmsi.Free(t)
}

func (amf *AMF) LookupUeByGuti(guti etsi.GUTI5G) (*UeContext, bool) {
	if guti == etsi.InvalidGUTI5G {
		return nil, false
	}

	amf.mu.RLock()
	defer amf.mu.RUnlock()

	// uesByTmsi indexes both the current and the in-flight old 5G-TMSI of every UE,
	// so an inbound GUTI/5G-S-TMSI resolves in O(1) without scanning every UE. The
	// 5G-TMSI is the unpredictable, per-UE part of the GUTI; the GUAMI is invariant,
	// so the TMSI alone disambiguates.
	ue, ok := amf.uesByTmsi[guti.Tmsi]

	return ue, ok
}

// ReallocateGUTI allocates a new 5G-GUTI for the UE and preserves the old one
// (resolvable until the UE acknowledges the reallocation, when CommitGUTIRealloc runs).
// A reallocation already in flight reuses its staged 5G-TMSI, so a retransmitted
// trigger does not burn a fresh one (TS 24.501 §5.4.4). The GUTI index is kept in
// step under a.mu.
func (a *AMF) ReallocateGUTI(ctx context.Context, ue *UeContext) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if ue.oldTmsi != etsi.InvalidTMSI {
		return nil
	}

	tmsi, err := a.allocateTMSI(ctx)
	if err != nil {
		return fmt.Errorf("failed to allocate TMSI: %v", err)
	}

	ue.oldTmsi = ue.tmsi
	ue.tmsi = tmsi

	a.uesByTmsi[tmsi] = ue

	return nil
}

// gutiFor rebuilds a 5G-GUTI from the invariant serving GUAMI and a per-UE 5G-TMSI,
// returning InvalidGUTI for an unset TMSI. The AMF stores only the TMSI (uesByTmsi)
// and reconstructs the GUTI here on demand, so the GUAMI is not duplicated per-UE
// (TS 23.003).
func gutiFor(guami *models.Guami, tmsi etsi.TMSI) (etsi.GUTI5G, error) {
	if guami == nil || tmsi == etsi.InvalidTMSI {
		return etsi.InvalidGUTI5G, nil
	}

	return etsi.NewGUTI5G(guami.PlmnID.Mcc, guami.PlmnID.Mnc, guami.AmfID, tmsi)
}

// Guti rebuilds the UE's current 5G-GUTI from the serving GUAMI and the stored
// 5G-TMSI, read under the registry lock.
func (a *AMF) Guti(guami *models.Guami, ue *UeContext) (etsi.GUTI5G, error) {
	if ue == nil {
		return etsi.InvalidGUTI5G, nil
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	return gutiFor(guami, ue.tmsi)
}

// OldGuti rebuilds the UE's in-flight previous 5G-GUTI, valid during a reallocation
// window (until CommitGUTIRealloc).
func (a *AMF) OldGuti(guami *models.Guami, ue *UeContext) (etsi.GUTI5G, error) {
	if ue == nil {
		return etsi.InvalidGUTI5G, nil
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	return gutiFor(guami, ue.oldTmsi)
}

// PagingGuti returns the 5G-GUTI to page the UE with: the previous GUTI while an
// unacknowledged reallocation is pending (the UE still listens on the old 5G-S-TMSI until
// it acknowledges), else the current GUTI (TS 24.501 §5.4.4). Read under one lock so the
// choice is consistent.
func (a *AMF) PagingGuti(guami *models.Guami, ue *UeContext) (etsi.GUTI5G, error) {
	if ue == nil {
		return etsi.InvalidGUTI5G, nil
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if ue.oldTmsi != etsi.InvalidTMSI {
		return gutiFor(guami, ue.oldTmsi)
	}

	return gutiFor(guami, ue.tmsi)
}

// CommitGUTIRealloc releases the previous 5G-TMSI for the UE and unindexes it.
func (a *AMF) CommitGUTIRealloc(ue *UeContext) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if ue.oldTmsi != etsi.InvalidTMSI {
		delete(a.uesByTmsi, ue.oldTmsi)
		a.freeTmsiLocked(ue.oldTmsi)
	}

	ue.oldTmsi = etsi.InvalidTMSI
}

func (amf *AMF) StmsiToGuti(ctx context.Context, buf []byte) (etsi.GUTI5G, error) {
	// The 5G-S-TMSI mobile identity is a fixed 7-octet value (TS 24.501 §9.11.3.4).
	if len(buf) != 7 {
		return etsi.InvalidGUTI5G, fmt.Errorf("invalid 5G-S-TMSI length: %d", len(buf))
	}

	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		return etsi.InvalidGUTI5G, fmt.Errorf("could not get operator info: %v", err)
	}

	tmpReginID := operatorInfo.Guami.AmfID[:2]
	amfID := hex.EncodeToString(buf[1:3])

	tmsi5G, err := etsi.NewTMSI(binary.BigEndian.Uint32(buf[3:]))
	if err != nil {
		return etsi.InvalidGUTI5G, err
	}

	guti, err := etsi.NewGUTI5G(operatorInfo.Guami.PlmnID.Mcc, operatorInfo.Guami.PlmnID.Mnc, tmpReginID+amfID, tmsi5G)
	if err != nil {
		return etsi.InvalidGUTI5G, err
	}

	return guti, nil
}
