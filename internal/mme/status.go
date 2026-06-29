// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"sort"
	"time"

	"github.com/ellanetworks/core/internal/sctp"
)

// ConnectedSubscriber is a runtime view of an EMM-registered UE for the API
// layer.
type ConnectedSubscriber struct {
	RadioName          string
	NumSessions        int
	Imei               string    // 15-digit IMEI from the UE's IMEISV, empty if unknown
	LastSeenAt         time.Time // most recent uplink NAS activity, zero if none
	CipheringAlgorithm string    // EPS NAS ciphering, e.g. "EEA2" (TS 33.401)
	IntegrityAlgorithm string    // EPS NAS integrity, e.g. "EIA2"
	// Sessions are the UE's PDN connections, one per active APN, ordered by EPS
	// bearer identity (TS 23.401).
	Sessions []SubscriberSession
}

// SubscriberSession is one PDN connection of an attached UE — a default EPS
// bearer to an APN (TS 23.401).
type SubscriberSession struct {
	BearerID     uint8
	APN          string
	PDNType      uint8 // negotiated PDN type: 1 IPv4 / 2 IPv6 / 3 IPv4v6
	IPv4Address  string
	IPv6Prefix   string
	AMBRUplink   string // session AMBR (profile UE-AMBR), raw "<n> <unit>" form
	AMBRDownlink string
}

// connectedSubscriber builds the runtime view of a UE. The caller must hold m.mu.
func (m *MME) connectedSubscriber(ue *UeContext) ConnectedSubscriber {
	radioName := ""

	if ue.S1 != nil {
		if conn, ok := ue.S1.conn.(*sctp.SCTPConn); ok {
			if s := m.enbs[conn]; s != nil {
				radioName = s.name
			}
		}
	}

	imei, eea, eia := ue.securitySnapshot()

	cs := ConnectedSubscriber{
		RadioName:          radioName,
		Imei:               imei,
		LastSeenAt:         ue.lastSeenTime(),
		CipheringAlgorithm: epsCipheringAlgName(eea),
		IntegrityAlgorithm: epsIntegrityAlgName(eia),
	}

	for _, p := range m.SnapshotPDNs(ue) {
		s := SubscriberSession{
			BearerID:     p.Ebi,
			APN:          p.Apn,
			PDNType:      p.PdnType,
			AMBRUplink:   ue.AmbrUplink,
			AMBRDownlink: ue.AmbrDownlink,
		}

		if p.UeIP.IsValid() {
			s.IPv4Address = p.UeIP.String()
		}

		if p.UeIPv6Prefix.IsValid() {
			s.IPv6Prefix = p.UeIPv6Prefix.String()
		}

		cs.Sessions = append(cs.Sessions, s)
	}

	sort.Slice(cs.Sessions, func(i, j int) bool { return cs.Sessions[i].BearerID < cs.Sessions[j].BearerID })
	cs.NumSessions = len(cs.Sessions)

	return cs
}

// ConnectedSubscribers returns the status of every EMM-registered UE keyed by
// IMSI. A registered UE has one default EPS bearer (TS 23.401), reported
// as a single session.
func (m *MME) ConnectedSubscribers() map[string]ConnectedSubscriber {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]ConnectedSubscriber)

	for imsi, ue := range m.ues {
		if ue.emmState.load() != EMMRegistered || ue.imsi == "" {
			continue
		}

		out[imsi] = m.connectedSubscriber(ue)
	}

	return out
}

// LookupSubscriber returns the runtime status of an EMM-registered UE by IMSI.
func (m *MME) LookupSubscriber(imsi string) (ConnectedSubscriber, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ue, ok := m.ues[imsi]
	if !ok || ue.emmState.load() != EMMRegistered {
		return ConnectedSubscriber{}, false
	}

	return m.connectedSubscriber(ue), true
}

// CountRegisteredSubscribers returns the number of EMM-registered UEs.
func (m *MME) CountRegisteredSubscribers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0

	for _, ue := range m.ues {
		if ue.emmState.load() == EMMRegistered && ue.imsi != "" {
			count++
		}
	}

	return count
}

func epsCipheringAlgName(eea byte) string {
	switch eea {
	case 0:
		return "EEA0"
	case 1:
		return "EEA1"
	case 2:
		return "EEA2"
	case 3:
		return "EEA3"
	default:
		return ""
	}
}

func epsIntegrityAlgName(eia byte) string {
	switch eia {
	case 0:
		return "EIA0"
	case 1:
		return "EIA1"
	case 2:
		return "EIA2"
	case 3:
		return "EIA3"
	default:
		return ""
	}
}
