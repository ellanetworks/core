// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/zap"
)

func (e *ENB) connectAllPeersLocked() error {
	e.servedGUMMEI = make(map[int][]s1ap.GUMMEI)
	e.enbPeers = make(map[int64]int)
	e.mmePeers = make(map[int64]int)
	e.sourceGUMMEIs = make(map[int64]s1ap.GUMMEI)

	for idx := range e.peers {
		if err := e.dialAndActivateLocked(idx); err != nil {
			for _, peer := range e.peers {
				if peer.conn != nil {
					_ = peer.conn.Close()
					peer.conn = nil
				}
			}

			e.mmeShutdown = true

			return fmt.Errorf("connect to every S1-MME peer: %w", err)
		}
	}

	e.active = 0

	return nil
}

func (e *ENB) awaitRemainingS1Setups() error {
	if !e.connectAll {
		return nil
	}

	for range e.ConnectedPeerCount() - 1 {
		if _, err := e.WaitForMessage(NoUEID, Successful, s1ap.ProcS1Setup, 5*time.Second); err != nil {
			return fmt.Errorf("S1 Setup did not complete on every S1-MME peer: %w", err)
		}
	}

	return nil
}

func (e *ENB) dropPeerLocked(idx int, conn *sctp.SCTPConn) {
	peer := e.peers[idx]
	if peer.conn != conn {
		return
	}

	_ = peer.conn.Close()
	peer.conn = nil
	peer.state = mmeStateFailed

	logger.GnbLogger.Warn("s1enb: S1-MME peer lost", zap.String("address", peer.address))
}

func (e *ENB) ConnectedPeerCount() int {
	e.mmeMu.RLock()
	defer e.mmeMu.RUnlock()

	n := 0

	for _, peer := range e.peers {
		if peer.conn != nil {
			n++
		}
	}

	return n
}

func (e *ENB) recordServedGUMMEIs(peer int, served s1ap.ServedGUMMEIs) {
	var gummeis []s1ap.GUMMEI

	for _, item := range served {
		for _, plmn := range item.ServedPLMNs {
			for _, group := range item.ServedGroupIDs {
				for _, code := range item.ServedMMECs {
					gummeis = append(gummeis, s1ap.GUMMEI{PLMNIdentity: plmn, MMEGroupID: group, MMECode: code})
				}
			}
		}
	}

	e.mmeMu.Lock()
	e.servedGUMMEI[peer] = gummeis
	e.mmeMu.Unlock()
}

func (e *ENB) ServingGUMMEI(enbUEID int64) (s1ap.GUMMEI, bool) {
	e.mmeMu.RLock()
	defer e.mmeMu.RUnlock()

	idx, ok := e.enbPeers[enbUEID]
	if !ok || len(e.servedGUMMEI[idx]) == 0 {
		return s1ap.GUMMEI{}, false
	}

	return e.servedGUMMEI[idx][0], true
}

func sameGUMMEI(a, b s1ap.GUMMEI) bool {
	return a.PLMNIdentity == b.PLMNIdentity && a.MMEGroupID == b.MMEGroupID && a.MMECode == b.MMECode
}

func (e *ENB) InheritServingMME(enbUEID int64, source *ENB, sourceENBUEID int64) error {
	if !e.connectAll {
		return nil
	}

	gummei, ok := source.ServingGUMMEI(sourceENBUEID)
	if !ok {
		return fmt.Errorf("source eNB holds no GUMMEI for eNB UE S1AP ID %d", sourceENBUEID)
	}

	e.mmeMu.Lock()
	defer e.mmeMu.Unlock()

	for idx, served := range e.servedGUMMEI {
		if e.peers[idx].conn == nil {
			continue
		}

		for _, s := range served {
			if !sameGUMMEI(s, gummei) {
				continue
			}

			e.enbPeers[enbUEID] = idx
			e.sourceGUMMEIs[enbUEID] = gummei

			logger.GnbLogger.Info("s1enb: selected the MME by GUMMEI",
				zap.Int64("eNB UE S1AP ID", enbUEID),
				zap.String("address", e.peers[idx].address),
				zap.Uint8("MME Code", uint8(gummei.MMECode)),
			)

			return nil
		}
	}

	return fmt.Errorf("no connected MME serves the GUMMEI with MME Group %x, MME Code %d", gummei.MMEGroupID, gummei.MMECode)
}

func (e *ENB) sourceMMEGUMMEI(enbUEID int64) *s1ap.GUMMEI {
	e.mmeMu.RLock()
	defer e.mmeMu.RUnlock()

	gummei, ok := e.sourceGUMMEIs[enbUEID]
	if !ok {
		return nil
	}

	return &gummei
}

func (e *ENB) ServingMMEAddress(enbUEID int64) string {
	e.mmeMu.RLock()
	defer e.mmeMu.RUnlock()

	idx, ok := e.enbPeers[enbUEID]
	if !ok {
		return ""
	}

	return e.peers[idx].address
}

func ueS1APIDs(packet []byte) (enbID, mmeID int64, enbOK, mmeOK bool) {
	raw, err := s1ap.ParseRawPDU(packet)
	if err != nil {
		return 0, 0, false, false
	}

	if id, ok, err := raw.ENBUES1APID(); err == nil && ok {
		enbID, enbOK = int64(id), true
	}

	if id, ok, err := raw.MMEUES1APID(); err == nil && ok {
		mmeID, mmeOK = int64(id), true
	} else if id, ok, err := raw.SourceMMEUES1APID(); err == nil && ok {
		mmeID, mmeOK = int64(id), true
	}

	return enbID, mmeID, enbOK, mmeOK
}

func (e *ENB) bindIncoming(packet []byte, f Frame) {
	if f.Category == Successful && f.ProcedureCode == s1ap.ProcS1Setup {
		if resp, err := s1ap.ParseS1SetupResponse(f.Value); err == nil {
			e.recordServedGUMMEIs(f.Peer, resp.ServedGUMMEIs)
		}

		return
	}

	enbID, mmeID, enbOK, mmeOK := ueS1APIDs(packet)

	if !enbOK && f.ENBUES1APID != NoUEID {
		enbID, enbOK = f.ENBUES1APID, true
	}

	e.mmeMu.Lock()
	defer e.mmeMu.Unlock()

	if enbOK {
		e.enbPeers[enbID] = f.Peer
	}

	if mmeOK {
		e.mmePeers[mmeID] = f.Peer
	}
}

func isUEContextReleaseComplete(packet []byte) bool {
	pdu, err := s1ap.Unmarshal(packet)
	if err != nil {
		return false
	}

	out, ok := pdu.(*s1ap.SuccessfulOutcome)

	return ok && out.ProcedureCode == s1ap.ProcUEContextRelease
}

func (e *ENB) sendToServingMME(pdu []byte, ueAssociated bool) error {
	enbID, mmeID, enbOK, mmeOK := ueS1APIDs(pdu)

	e.mmeMu.Lock()

	idx := e.active

	if p, ok := e.enbPeers[enbID]; enbOK && ok {
		idx = p
	} else if p, ok := e.mmePeers[mmeID]; mmeOK && ok {
		idx = p
	}

	if enbOK {
		if isUEContextReleaseComplete(pdu) {
			delete(e.enbPeers, enbID)
		} else {
			e.enbPeers[enbID] = idx
		}
	}

	var conn *sctp.SCTPConn

	if idx >= 0 && idx < len(e.peers) {
		conn = e.peers[idx].conn
	}

	e.mmeMu.Unlock()

	if conn == nil {
		return ErrNoActiveMME
	}

	e.sendMu.Lock()
	defer e.sendMu.Unlock()

	return writeMessage(conn, pdu, ueAssociated)
}
