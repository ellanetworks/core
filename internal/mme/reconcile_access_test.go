// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// The QoS-only reconcile path signals the UE without otherwise touching the
// anchor, so it asks the anchor which access serves the session. A MODIFY EPS
// BEARER CONTEXT REQUEST for a session that moved to 5GS would carry EPS QoS for
// a PDU session the MME no longer owns.
func TestQoSReconcileSignalsOnlyWhileTheAnchorServesEPS(t *testing.T) {
	for _, tc := range []struct {
		name        string
		movedOffEPS bool
		wantSent    bool
	}{
		{"anchor serves EPS", false, true},
		{"session moved to 5GS", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			sm := &fakeSessionManager{movedOffEPS: tc.movedOffEPS}
			m.Session = sm

			ue, cc := securedUE(t, m)
			ctx := context.Background()

			qos, err := ResolveQoSByAPN(ctx, m, ue.IMSI(), "internet")
			if err != nil {
				t.Fatalf("ResolveQoSByAPN: %v", err)
			}

			p := testPDN(ue)
			p.Apn = "internet"
			p.PdnType = eps.PDNTypeIPv4
			p.SessionRef = "anchor-session"
			p.DnConfig = qos.DnFingerprint()
			p.SessAmbrDLBps = qos.SessAmbrDL.Bps()
			p.SessAmbrULBps = qos.SessAmbrUL.Bps()
			p.Arp = qos.ARP

			// The one difference the reconciler can adopt in place.
			p.Qci = qos.QCI + 1

			cc.mu.Lock()
			before := len(cc.sent)
			cc.mu.Unlock()

			m.ReconcileUE(ctx, ue)

			cc.mu.Lock()
			after := len(cc.sent)
			cc.mu.Unlock()

			if sent := after > before; sent != tc.wantSent {
				t.Errorf("bearer modification signalled = %v, want %v", sent, tc.wantSent)
			}
		})
	}
}

// TS 24.301 §6.6.1.1 carries the configuration options of a transferred PDN
// connection in the extended element where the UE and network support it end to
// end; §8.3.18.9 scopes the classic element to the case where they do not. A
// bearer modification uses the same element as the activation.
func TestBearerModificationUsesTheElementThePDNConnectionWasSetUpWith(t *testing.T) {
	for _, tc := range []struct {
		name     string
		extended bool
	}{
		{"extended supported end to end", true},
		{"extended unsupported", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &eps.ModifyEPSBearerContextRequest{}
			p := &PdnConnection{Ebi: DefaultERABID, PdnType: eps.PDNTypeIPv4}
			qos := &EpsQoS{DNS: "8.8.8.8", MTU: 1400}

			if _, ok := setModifyConfigurationOptions(req, p, qos, tc.extended); !ok {
				t.Fatal("DNS reported as unset, want it parsed")
			}

			gotExtended := req.ExtendedProtocolConfigurationOptions != nil
			gotClassic := req.ProtocolConfigurationOptions != nil

			if gotExtended != tc.extended || gotClassic == tc.extended {
				t.Errorf("extended element = %v, classic element = %v, want %v and %v",
					gotExtended, gotClassic, tc.extended, !tc.extended)
			}
		})
	}
}

// The element a bearer modification uses is decided by the request type the PDN
// connection was opened with and the UE's own ePCO support (TS 24.301 §6.6.1.1,
// §5.5.1.2.4), the same predicate the activation goes through.
func TestBearerModificationElementFollowsTheRequestTypeAndUECapability(t *testing.T) {
	// ePCO is octet 8 bit 8 of the UE network capability (TS 24.301 §9.9.3.34);
	// Rest starts at octet 7.
	const (
		advertised    = 0x80
		notAdvertised = 0x00
	)

	for _, tc := range []struct {
		name        string
		requestType eps.RequestType
		octet8      byte
		wantExt     bool
	}{
		{"transferred connection, UE advertised ePCO", eps.RequestTypeHandover, advertised, true},
		{"transferred connection, UE did not advertise ePCO", eps.RequestTypeHandover, notAdvertised, false},
		{"initial request, UE advertised ePCO", eps.RequestTypeInitialRequest, advertised, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, cc := connectedBearerUE(t, m)
			ue.SetUESecurityCapability(eps.UENetworkCapability{
				EEA:  0xF0,
				EIA:  0x70,
				Rest: []byte{0x00, tc.octet8},
			}, nil, MintAuthProofForAttachRequest())

			p := testPDN(ue)
			p.PdnType = eps.PDNTypeIPv4
			m.SetPDNRequestType(ue, p, tc.requestType)

			defer m.StopESMGuard(p)

			qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
			if err != nil {
				t.Fatal(err)
			}

			// A fingerprint differing from the current one in the DNS field alone, so
			// the bearer is modified in place and the modification carries the PCO.
			parts := strings.Split(qos.DnFingerprint(), "|")
			parts[2] = "9.9.9.9"
			p.DnConfig = strings.Join(parts, "|")

			m.ReconcileDataNetwork(context.Background())

			if len(cc.sent) != 1 {
				t.Fatalf("messages sent = %d, want 1 Modify EPS Bearer Context Request", len(cc.sent))
			}

			wire := decodeDownlinkNAS(t, cc.sent[0])

			plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink,
				mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
			if err != nil {
				t.Fatalf("unprotect downlink: %v", err)
			}

			req, err := eps.ParseModifyEPSBearerContextRequest(plain)
			if err != nil {
				t.Fatalf("parse Modify request: %v", err)
			}

			gotExt := req.ExtendedProtocolConfigurationOptions != nil
			gotClassic := req.ProtocolConfigurationOptions != nil

			if gotExt != tc.wantExt || gotClassic == tc.wantExt {
				t.Errorf("extended element = %v, classic element = %v, want %v and %v",
					gotExt, gotClassic, tc.wantExt, !tc.wantExt)
			}
		})
	}
}
