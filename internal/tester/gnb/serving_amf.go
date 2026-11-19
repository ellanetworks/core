// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"net"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/zap"
)

func (g *GnodeB) connectAllPeersLocked(n3Conn *net.UDPConn) (*GnodeB, error) {
	g.servedGUAMI = make(map[int][]ngap.GUAMI)
	g.ranPeers = make(map[int64]int)
	g.amfPeers = make(map[int64]int)

	for idx := range g.n2Peers {
		if err := g.n2DialAndActivateLocked(idx); err != nil {
			for _, peer := range g.n2Peers {
				if peer.conn != nil {
					_ = peer.conn.Close()
					peer.conn = nil
				}
			}

			g.n2Shutdown = true

			if n3Conn != nil {
				_ = n3Conn.Close()
			}

			return nil, fmt.Errorf("connect to every N2 peer: %w", err)
		}
	}

	g.n2Active = 0

	return g, nil
}

func (g *GnodeB) dropPeerLocked(idx int, conn *sctp.SCTPConn) {
	peer := g.n2Peers[idx]
	if peer.conn != conn {
		return
	}

	_ = peer.conn.Close()
	peer.conn = nil
	peer.state = n2StateFailed

	logger.GnbLogger.Warn("gnb: N2 peer lost", zap.String("address", peer.address))
}

func (g *GnodeB) ConnectedPeerCount() int {
	g.n2Mu.RLock()
	defer g.n2Mu.RUnlock()

	n := 0

	for _, peer := range g.n2Peers {
		if peer.conn != nil {
			n++
		}
	}

	return n
}

func (g *GnodeB) AwaitNGSetup(timeout time.Duration) error {
	for range g.ConnectedPeerCount() {
		if _, err := g.WaitForMessage(Successful, ngap.ProcNGSetup, timeout); err != nil {
			return err
		}
	}

	return nil
}

func (g *GnodeB) recordServedGUAMIs(peer int, list ngap.ServedGUAMIList) {
	if !g.connectAll {
		return
	}

	guamis := make([]ngap.GUAMI, 0, len(list))
	for _, item := range list {
		guamis = append(guamis, item.GUAMI)
	}

	g.n2Mu.Lock()
	g.servedGUAMI[peer] = guamis
	g.n2Mu.Unlock()
}

func (g *GnodeB) recordUEGUAMI(ranUENGAPID int64, guami ngap.GUAMI) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.ueGUAMIs == nil {
		g.ueGUAMIs = make(map[int64]ngap.GUAMI)
	}

	g.ueGUAMIs[ranUENGAPID] = guami
}

func (g *GnodeB) ServingGUAMI(ranUENGAPID int64) (ngap.GUAMI, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	guami, ok := g.ueGUAMIs[ranUENGAPID]

	return guami, ok
}

func sameGUAMI(a, b ngap.GUAMI) bool {
	return a.PLMNIdentity == b.PLMNIdentity && a.AMFRegionID == b.AMFRegionID && a.AMFSetID == b.AMFSetID && a.AMFPointer == b.AMFPointer
}

func (g *GnodeB) InheritServingAMF(ranUENGAPID int64, source *GnodeB, sourceRANUENGAPID int64) error {
	if !g.connectAll {
		return nil
	}

	guami, ok := source.ServingGUAMI(sourceRANUENGAPID)
	if !ok {
		return fmt.Errorf("source gNB holds no GUAMI for RAN UE NGAP ID %d", sourceRANUENGAPID)
	}

	g.n2Mu.Lock()
	defer g.n2Mu.Unlock()

	for idx, served := range g.servedGUAMI {
		if g.n2Peers[idx].conn == nil {
			continue
		}

		for _, s := range served {
			if !sameGUAMI(s, guami) {
				continue
			}

			g.ranPeers[ranUENGAPID] = idx

			logger.GnbLogger.Info("gnb: selected the AMF by GUAMI",
				zap.Int64("RAN UE NGAP ID", ranUENGAPID),
				zap.String("address", g.n2Peers[idx].address),
				zap.Uint8("AMF Pointer", uint8(guami.AMFPointer)),
			)

			return nil
		}
	}

	return fmt.Errorf("no connected AMF serves the GUAMI with AMF Region %d, AMF Set %d, AMF Pointer %d", guami.AMFRegionID, guami.AMFSetID, guami.AMFPointer)
}

func (g *GnodeB) ServingAMFAddress(ranUENGAPID int64) string {
	g.n2Mu.RLock()
	defer g.n2Mu.RUnlock()

	idx, ok := g.ranPeers[ranUENGAPID]
	if !ok {
		return ""
	}

	return g.n2Peers[idx].address
}

func (g *GnodeB) bindIncoming(frame SCTPFrame) {
	pdu, err := ngap.Unmarshal(frame.Data)
	if err != nil {
		return
	}

	ranID, ranOK := ngap.PeekRANUENGAPID(pdu)
	amfID, amfOK := ngap.PeekAMFUENGAPID(pdu)

	if !ranOK {
		if id, ok := frameRANUEID(frame); ok {
			ranID, ranOK = ngap.RANUENGAPID(id), true
		}
	}

	g.n2Mu.Lock()
	defer g.n2Mu.Unlock()

	if ranOK {
		g.ranPeers[int64(ranID)] = frame.Peer
	}

	if amfOK {
		g.amfPeers[int64(amfID)] = frame.Peer
	}
}

func (g *GnodeB) sendToServingAMF(packet []byte, msgType NGAPProcedure) error {
	var (
		ranID, amfID int64
		ranOK, amfOK bool
	)

	if pdu, err := ngap.Unmarshal(packet); err == nil {
		if id, ok := ngap.PeekRANUENGAPID(pdu); ok {
			ranID, ranOK = int64(id), true
		}

		if id, ok := ngap.PeekAMFUENGAPID(pdu); ok {
			amfID, amfOK = int64(id), true
		}
	}

	g.n2Mu.Lock()

	idx := g.n2Active

	if p, ok := g.ranPeers[ranID]; ranOK && ok {
		idx = p
	} else if p, ok := g.amfPeers[amfID]; amfOK && ok {
		idx = p
	}

	if ranOK {
		if msgType == NGAPProcedureUEContextReleaseComplete {
			delete(g.ranPeers, ranID)
		} else {
			g.ranPeers[ranID] = idx
		}
	}

	var conn *sctp.SCTPConn

	if idx >= 0 && idx < len(g.n2Peers) {
		conn = g.n2Peers[idx].conn
	}

	g.n2Mu.Unlock()

	if conn == nil {
		return ErrNoActivePeer
	}

	return writeToConn(conn, packet, msgType)
}
