// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/per"
)

type rel19Case struct {
	name  string
	msg   interface{ Marshal() ([]byte, error) }
	id    ProtocolIEID
	crit  Criticality
	value []byte
	parse func(b []byte) (interface{ Marshal() ([]byte, error) }, error)
}

func rel19MessageCases(t *testing.T) []rel19Case {
	t.Helper()

	tunnel := TunnelInformation{TransportLayerAddress: tlaOf(192, 0, 2, 9), UDPPortNumber: &PortNumber{0x08, 0x68}}

	return []rel19Case{
		{
			name: "HandoverRequest/TimeRefDistribution",
			msg: &HandoverRequest{
				Cause: Ptr(CauseRadioNetworkUnspecified.Cause()), ERABToBeSetup: ERABToBeSetupListHOReq{{ERABID: 5, TransportLayerAddress: goldTLA(), GTPTEID: 1, QoS: goldQoS()}},
				SourceToTarget: TransparentContainer{0x00}, TimeRefDistribution: Ptr(TimeRefDistributionTrue),
			},
			id: IDTimeRefDistribution, crit: CriticalityIgnore, value: []byte{0x00},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) { return ParseHandoverRequest(b) },
		},
		{
			name: "PathSwitchRequestAcknowledge/TimeRefDistribution",
			msg:  &PathSwitchRequestAcknowledge{MMEUES1APID: Ptr(MMEUES1APID(1)), ENBUES1APID: Ptr(ENBUES1APID(2)), TimeRefDistribution: Ptr(TimeRefDistributionTrue)},
			id:   IDTimeRefDistribution, crit: CriticalityIgnore, value: []byte{0x00},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) {
				return ParsePathSwitchRequestAcknowledge(b)
			},
		},
		{
			name: "InitialContextSetupRequest/CoarseUELocation",
			msg:  &InitialContextSetupRequest{MMEUES1APID: 1, ENBUES1APID: 2, ERABToBeSetup: []ERABToBeSetupItemCtxtSUReq{{ERABID: 5, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1}}, CoarseUELocation: &CoarseUELocation{0x11, 0x22}},
			id:   IDCoarseUELocation, crit: CriticalityIgnore, value: []byte{0x02, 0x11, 0x22},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) {
				return ParseInitialContextSetupRequest(b)
			},
		},
		{
			name: "InitialContextSetupRequest/TimeRefDistribution",
			msg:  &InitialContextSetupRequest{MMEUES1APID: 1, ENBUES1APID: 2, ERABToBeSetup: []ERABToBeSetupItemCtxtSUReq{{ERABID: 5, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1}}, TimeRefDistribution: Ptr(TimeRefDistributionTrue)},
			id:   IDTimeRefDistribution, crit: CriticalityIgnore, value: []byte{0x00},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) {
				return ParseInitialContextSetupRequest(b)
			},
		},
		{
			name: "UEContextModificationRequest/TimeRefDistribution",
			msg:  &UEContextModificationRequest{MMEUES1APID: 1, ENBUES1APID: 2, TimeRefDistribution: Ptr(TimeRefDistributionTrue)},
			id:   IDTimeRefDistribution, crit: CriticalityIgnore, value: []byte{0x00},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) {
				return ParseUEContextModificationRequest(b)
			},
		},
		{
			name: "DownlinkNASTransport/CoarseUELocation",
			msg:  &DownlinkNASTransport{MMEUES1APID: 1, ENBUES1APID: 2, NASPDU: NASPDU{0x07}, CoarseUELocation: &CoarseUELocation{0x33}},
			id:   IDCoarseUELocation, crit: CriticalityIgnore, value: []byte{0x01, 0x33},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) { return ParseDownlinkNASTransport(b) },
		},
		{
			name: "InitialUEMessage/CoarseUELocationRequested",
			msg:  &InitialUEMessage{ENBUES1APID: 2, NASPDU: NASPDU{0x07}, TAI: TAI{PLMNIdentity: goldPLMN(), TAC: 1}, EUTRANCGI: &EUTRANCGI{PLMNIdentity: goldPLMN(), CellID: 1}, RRCEstablishmentCause: Ptr(RRCCauseMOSignalling), CoarseUELocationRequested: Ptr(CoarseUELocationRequestedTrue)},
			id:   IDCoarseUELocationRequested, crit: CriticalityIgnore, value: []byte{0x00},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) { return ParseInitialUEMessage(b) },
		},
		{
			name: "InitialUEMessage/GW-TransportLayerAddress",
			msg:  &InitialUEMessage{ENBUES1APID: 2, NASPDU: NASPDU{0x07}, TAI: TAI{PLMNIdentity: goldPLMN(), TAC: 1}, EUTRANCGI: &EUTRANCGI{PLMNIdentity: goldPLMN(), CellID: 1}, RRCEstablishmentCause: Ptr(RRCCauseMOSignalling), GWTransportLayerAddress: Ptr(tlaOf(192, 0, 2, 1))},
			id:   IDGWTransportLayerAddress, crit: CriticalityIgnore, value: []byte{0x0f, 0x80, 0xc0, 0x00, 0x02, 0x01},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) { return ParseInitialUEMessage(b) },
		},
		{
			name: "UplinkNASTransport/SIPTO-L-GW-TransportLayerAddress",
			msg:  &UplinkNASTransport{MMEUES1APID: 1, ENBUES1APID: 2, NASPDU: NASPDU{0x07}, EUTRANCGI: &EUTRANCGI{PLMNIdentity: goldPLMN(), CellID: 1}, TAI: &TAI{PLMNIdentity: goldPLMN(), TAC: 1}, SIPTOLGWTransportLayerAddress: Ptr(tlaOf(192, 0, 2, 1))},
			id:   IDSIPTOLGWTransportLayerAddress, crit: CriticalityIgnore, value: []byte{0x0f, 0x80, 0xc0, 0x00, 0x02, 0x01},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) { return ParseUplinkNASTransport(b) },
		},
		{
			name: "HandoverNotify/Tunnel-Information-for-BBF",
			msg:  &HandoverNotify{MMEUES1APID: 1, ENBUES1APID: 2, EUTRANCGI: &EUTRANCGI{PLMNIdentity: goldPLMN(), CellID: 1}, TAI: &TAI{PLMNIdentity: goldPLMN(), TAC: 1}, TunnelInformationForBBF: &tunnel},
			id:   IDTunnelInformationForBBF, crit: CriticalityIgnore, value: []byte{0x41, 0xf0, 0xc0, 0x00, 0x02, 0x09, 0x08, 0x68},
			parse: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) { return ParseHandoverNotify(b) },
		},
	}
}

func TestRel19AndCarrierMessageIEs(t *testing.T) {
	for _, tc := range rel19MessageCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.msg.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			body := pdu.value()

			field := findField(t, body, tc.id)
			if field.Criticality != tc.crit || !bytes.Equal(field.Value, tc.value) {
				t.Fatalf("IE %s = %s % x, want %s % x", tc.id, field.Criticality, field.Value, tc.crit, tc.value)
			}

			out, err := tc.parse(body)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			again, err := out.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(again, b) {
				t.Fatalf("re-encoded\n got % x\nwant % x", again, b)
			}
		})
	}
}

func findField(t *testing.T, body []byte, id ProtocolIEID) rawIE {
	t.Helper()

	r := per.NewReader(body)
	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range fields {
		if f.ID == id {
			return f
		}
	}

	t.Fatalf("IE %s not in the message", id)

	return rawIE{}
}

func TestS1RemovalProcedure(t *testing.T) {
	req := &S1RemovalRequest{GlobalENBID: GlobalENBID{PLMNIdentity: goldPLMN(), ENBID: ENBID{MacroENBID: Ptr(MacroENBID(0x12345))}}}

	b, err := req.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got, want := hex.EncodeToString(b), "0043000f000001003b00080000f11000123450"; got != want {
		t.Fatalf("S1 REMOVAL REQUEST = %s, want %s", got, want)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseS1RemovalRequest(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	if !got.GlobalENBID.Equal(req.GlobalENBID) {
		t.Fatalf("Global-ENB-ID = %+v", got.GlobalENBID)
	}

	resp := &S1RemovalResponse{MMEName: Ptr(Name("mme1"))}

	b, err = resp.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if pdu, err = Unmarshal(b); err != nil {
		t.Fatal(err)
	}

	if so, ok := pdu.(*SuccessfulOutcome); !ok || so.ProcedureCode != ProcS1Removal || so.Criticality != CriticalityReject {
		t.Fatalf("response PDU = %+v", pdu)
	}

	r, err := ParseS1RemovalResponse(pdu.value())
	if err != nil || r.MMEName == nil || *r.MMEName != "mme1" {
		t.Fatalf("response = %+v, %v", r, err)
	}

	fail := &S1RemovalFailure{Cause: Ptr(CauseMiscUnspecified.Cause())}

	b, err = fail.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if pdu, err = Unmarshal(b); err != nil {
		t.Fatal(err)
	}

	if uo, ok := pdu.(*UnsuccessfulOutcome); !ok || uo.ProcedureCode != ProcS1Removal {
		t.Fatalf("failure PDU = %+v", pdu)
	}

	f, err := ParseS1RemovalFailure(pdu.value())
	if err != nil || f.Cause == nil || !f.Cause.Equal(CauseMiscUnspecified.Cause()) {
		t.Fatalf("failure = %+v, %v", f, err)
	}

	missing, err := DecodeS1RemovalFailure(container(t))
	if err != nil {
		t.Fatal(err)
	}

	if d := missing.Diagnostics(); len(d.IEs) != 1 || d.IEs[0].ID != IDCause {
		t.Fatalf("diagnostics = %+v, want the missing Cause", d)
	}

	if ProcS1Removal != 67 {
		t.Fatalf("ProcS1Removal = %d", ProcS1Removal)
	}
}
