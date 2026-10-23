// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func queueERABSetupRequest(t *testing.T, e *ENB, enbUEID int64, qos s1ap.ERABLevelQoSParameters, nasPDU []byte) {
	t.Helper()

	wire, err := (&s1ap.ERABSetupRequest{
		MMEUES1APID: 9,
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		ERABToBeSetup: []s1ap.ERABToBeSetupItemBearerSUReq{{
			ERABID:                6,
			QoS:                   qos,
			TransportLayerAddress: s1ap.TransportLayerAddress(netip.MustParseAddr("10.0.0.1").AsSlice()),
			GTPTEID:               0x1234,
			NASPDU:                s1ap.NASPDU(nasPDU),
		}},
	}).Marshal()
	if err != nil {
		t.Fatalf("marshal E-RAB Setup Request: %v", err)
	}

	pdu, err := s1ap.Unmarshal(wire)
	if err != nil {
		t.Fatalf("unmarshal E-RAB Setup Request: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok {
		t.Fatalf("E-RAB Setup Request decoded as %T", pdu)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.receivedFrames[Initiating] = map[s1ap.ProcedureCode][]Frame{
		s1ap.ProcERABSetup: {{Category: Initiating, ProcedureCode: im.ProcedureCode, Value: im.Value, ENBUES1APID: enbUEID}},
	}
}

func protectedDownlink(t *testing.T, sc *nas.SecurityContext, plain []byte) []byte {
	t.Helper()

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, 0), nas.DirectionDownlink, sc)
	if err != nil {
		t.Fatalf("protect downlink NAS: %v", err)
	}

	return wire
}

func TestAwaitDedicatedActivationAcceptsANonGBRBearer(t *testing.T) {
	const enbUEID = 3

	var (
		kInt nas.IntegrityKey
		kEnc nas.CipherKey
	)

	for i := range kInt {
		kInt[i], kEnc[i] = byte(i+1), byte(0xa0+i)
	}

	sc := mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, kInt, kEnc)

	e := &ENB{receivedFrames: make(map[Category]map[s1ap.ProcedureCode][]Frame)}
	e.cond = sync.NewCond(&e.mu)

	ue := &UE{sc: sc}

	info := &mme.DedicatedBearerInfo{DedicatedBearerContext: models.DedicatedBearerContext{
		EBI: 6,
		QCI: 69,
		ARP: models.Arp{PriorityLevel: 2, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable},
	}}

	filter := eps.TFTPacketFilter{
		Identifier: 1,
		Direction:  eps.TFTBidirectional,
		Precedence: 10,
		Components: []eps.TFTComponent{eps.ProtocolIdentifier(17), eps.SingleRemotePort(5000)},
	}

	plain, err := (&eps.ActivateDedicatedEPSBearerContextRequest{
		EPSBearerIdentity:       6,
		PTI:                     0,
		LinkedEPSBearerIdentity: 5,
		EPSQoS:                  eps.EPSQoS{QCI: 69},
		TFT:                     eps.TrafficFlowTemplate{Operation: eps.TFTCreate, Filters: []eps.TFTPacketFilter{filter}},
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("marshal Activate Dedicated EPS Bearer Context Request: %v", err)
	}

	queueERABSetupRequest(t, e, enbUEID, mme.DedicatedERABQoS(info), protectedDownlink(t, sc, plain))

	req, act, err := e.awaitDedicatedActivation(ue, enbUEID, time.Second)
	if err != nil {
		t.Fatalf("awaitDedicatedActivation: %v", err)
	}

	qos := req.ERABToBeSetup[0].QoS

	if qos.QCI != 69 || qos.ARP.PriorityLevel != 2 {
		t.Errorf("E-RAB QoS = QCI %d ARP %d, want QCI 69 ARP 2", qos.QCI, qos.ARP.PriorityLevel)
	}

	if qos.GBR != nil {
		t.Errorf("E-RAB GBR QoS Information = %+v, want none for a non-GBR QCI", qos.GBR)
	}

	if act.EPSQoS.QCI != 69 || len(act.EPSQoS.BitRates) != 0 {
		t.Errorf("EPS QoS = %+v, want the QCI octet alone", act.EPSQoS)
	}

	if _, ok := act.EPSQoS.GBRBitRates(); ok {
		t.Error("a QCI-only EPS QoS must report no bit rates")
	}

	if len(act.TFT.Filters) != 1 || act.TFT.Filters[0].Identifier != 1 {
		t.Errorf("TFT = %+v, want the one packet filter", act.TFT)
	}
}
