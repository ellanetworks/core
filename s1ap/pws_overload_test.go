// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/per"
)

func pwsPLMN() PLMNIdentity { return PLMNIdentity{0x00, 0xf1, 0x10} }

func pwsECGI(cell uint32) EUTRANCGI { return EUTRANCGI{PLMNIdentity: pwsPLMN(), CellID: cell} }

func pwsTAI(tac TAC) TAI { return TAI{PLMNIdentity: pwsPLMN(), TAC: tac} }

func pwsENB() GlobalENBID {
	return GlobalENBID{PLMNIdentity: pwsPLMN(), ENBID: ENBID{Kind: ENBIDMacro, Value: 0x12345}}
}

func pwsGUMMEI() GUMMEI {
	return GUMMEI{PLMNIdentity: pwsPLMN(), MMEGroupID: MMEGroupID{0x80, 0x01}, MMECode: 0x01}
}

func pwsCBSPage() WarningMessageContents {
	page := make(WarningMessageContents, 84)
	page[0] = 1
	copy(page[1:], "hello")
	page[83] = 5

	return page
}

func pwsInitialUEMessage() S1Message {
	b, err := hex.DecodeString("000c402b000005000800020001001a000302074a004300060000f1100001006440080000f110123450100086400130")
	if err != nil {
		panic(err)
	}

	return b
}

type newMessageCase struct {
	name   string
	msg    interface{ Marshal() ([]byte, error) }
	parse  func([]byte) (any, error)
	choice func(PDU) bool
	golden string
}

func isInitiating(p PDU) bool { _, ok := p.(*InitiatingMessage); return ok }

func isSuccessful(p PDU) bool { _, ok := p.(*SuccessfulOutcome); return ok }

func newMessageCases() []newMessageCase {
	return append([]newMessageCase{
		{
			name: "OverloadStart",
			msg: &OverloadStart{
				OverloadResponse:               OverloadResponse{OverloadAction: OverloadActionRejectDelayTolerantAccess},
				GUMMEIList:                     []GUMMEI{pwsGUMMEI()},
				TrafficLoadReductionIndication: Ptr(uint8(50)),
			},
			parse:  func(b []byte) (any, error) { return ParseOverloadStart(b) },
			choice: isInitiating,
			golden: "0022401a000003006500024080009a4008000000f11080010100a1400162",
		},
		{
			name:   "OverloadStop",
			msg:    &OverloadStop{GUMMEIList: []GUMMEI{pwsGUMMEI(), pwsGUMMEI()}},
			parse:  func(b []byte) (any, error) { return ParseOverloadStop(b) },
			choice: isInitiating,
			golden: "00230016000001009a400f010000f1108001010000f110800101",
		},
		{
			name: "WriteReplaceWarningRequest",
			msg: &WriteReplaceWarningRequest{
				MessageIdentifier:                 0x1112,
				SerialNumber:                      0x3000,
				WarningAreaList:                   &WarningAreaList{CellIDList: []EUTRANCGI{pwsECGI(0x1234501), pwsECGI(0x1234502)}},
				RepetitionPeriod:                  60,
				ExtendedRepetitionPeriod:          Ptr(uint32(5000)),
				NumberofBroadcastRequest:          0,
				WarningType:                       &WarningType{0x05, 0x80},
				WarningSecurityInfo:               &WarningSecurityInfo{},
				DataCodingScheme:                  Ptr(DataCodingScheme(0x0f)),
				WarningMessageContents:            Ptr(pwsCBSPage()),
				ConcurrentWarningMessageIndicator: Ptr(ConcurrentWarningMessageIndicatorTrue),
				WarningAreaCoordinates:            &WarningAreaCoordinates{0xaa, 0xbb},
			},
			parse:  func(b []byte) (any, error) { return ParseWriteReplaceWarningRequest(b) },
			choice: isInitiating,
			golden: "00240080e000000c006f00021112007000023000007140120000010000f1101234501000f1101234502000720002003c00900003400388007300020000007440020580007540320000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007640010f0077405600530168656c6c6f000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005008e000100011e40040001aabb",
		},
		{
			name: "WriteReplaceWarningRequestTAIs",
			msg: &WriteReplaceWarningRequest{
				MessageIdentifier:        0x1100,
				SerialNumber:             0x0001,
				WarningAreaList:          &WarningAreaList{TAIList: []TAI{pwsTAI(1)}},
				RepetitionPeriod:         0,
				NumberofBroadcastRequest: 1,
			},
			parse:  func(b []byte) (any, error) { return ParseWriteReplaceWarningRequest(b) },
			choice: isInitiating,
			golden: "00240028000005006f00021100007000020001007140092000000000f1100001007200020000007300020001",
		},
		{
			name: "WriteReplaceWarningResponse",
			msg: &WriteReplaceWarningResponse{
				MessageIdentifier: 0x1112,
				SerialNumber:      0x3000,
				BroadcastCompletedAreaList: &BroadcastCompletedAreaList{TAIBroadcast: []TAIBroadcastItem{{
					TAI:                pwsTAI(7),
					CompletedCellInTAI: []CompletedCellInTAIItem{{ECGI: pwsECGI(0x1234501)}},
				}}},
				CriticalityDiagnostics: &CriticalityDiagnostics{ProcedureCode: Ptr(ProcWriteReplaceWarning)},
			},
			parse:  func(b []byte) (any, error) { return ParseWriteReplaceWarningResponse(b) },
			choice: isSuccessful,
			golden: "2024002c000004006f00021112007000023000007840132000000000f110000700000000f11012345010003a40024024",
		},
		{
			name: "WriteReplaceWarningResponseEmergencyAreas",
			msg: &WriteReplaceWarningResponse{
				MessageIdentifier: 0x1112,
				SerialNumber:      0x3000,
				BroadcastCompletedAreaList: &BroadcastCompletedAreaList{EmergencyAreaIDBroadcast: []EmergencyAreaIDBroadcastItem{{
					EmergencyAreaID:    EmergencyAreaID{0x01, 0x02, 0x03},
					CompletedCellInEAI: []CompletedCellInEAIItem{{ECGI: pwsECGI(0x1234501)}},
				}}},
			},
			parse:  func(b []byte) (any, error) { return ParseWriteReplaceWarningResponse(b) },
			choice: isSuccessful,
			golden: "20240024000003006f00021112007000023000007840114000000001020300000000f11012345010",
		},
		{
			name: "KillRequest",
			msg: &KillRequest{
				MessageIdentifier:      0x1112,
				SerialNumber:           0x3000,
				WarningAreaList:        &WarningAreaList{EmergencyAreaIDList: []EmergencyAreaID{{0x01, 0x02, 0x03}}},
				KillAllWarningMessages: Ptr(KillAllWarningMessagesTrue),
			},
			parse:  func(b []byte) (any, error) { return ParseKillRequest(b) },
			choice: isInitiating,
			golden: "002b001e000004006f000211120070000230000071400640000001020300bf000100",
		},
		{
			name: "KillResponse",
			msg: &KillResponse{
				MessageIdentifier: 0x1112,
				SerialNumber:      0x3000,
				BroadcastCancelledAreaList: &BroadcastCancelledAreaList{CellIDCancelled: []CellIDCancelledItem{
					{ECGI: pwsECGI(0x1234501), NumberOfBroadcasts: 3},
				}},
			},
			parse:  func(b []byte) (any, error) { return ParseKillResponse(b) },
			choice: isSuccessful,
			golden: "202b0020000003006f00021112007000023000008d400d0000000000f110123450100003",
		},
		{
			name: "KillResponseTAIs",
			msg: &KillResponse{
				MessageIdentifier: 0x1112,
				SerialNumber:      0x3000,
				BroadcastCancelledAreaList: &BroadcastCancelledAreaList{TAICancelled: []TAICancelledItem{{
					TAI:                pwsTAI(7),
					CancelledCellInTAI: []CancelledCellInTAIItem{{ECGI: pwsECGI(0x1234501), NumberOfBroadcasts: 300}},
				}}},
			},
			parse:  func(b []byte) (any, error) { return ParseKillResponse(b) },
			choice: isSuccessful,
			golden: "202b0028000003006f00021112007000023000008d40152000000000f110000700000000f11012345010012c",
		},
		{
			name: "KillResponseEmergencyAreas",
			msg: &KillResponse{
				MessageIdentifier: 0x1112,
				SerialNumber:      0x3000,
				BroadcastCancelledAreaList: &BroadcastCancelledAreaList{EmergencyAreaIDCancelled: []EmergencyAreaIDCancelledItem{{
					EmergencyAreaID:    EmergencyAreaID{0x01, 0x02, 0x03},
					CancelledCellInEAI: []CancelledCellInEAIItem{{ECGI: pwsECGI(0x1234501), NumberOfBroadcasts: 1}},
				}}},
			},
			parse:  func(b []byte) (any, error) { return ParseKillResponse(b) },
			choice: isSuccessful,
			golden: "202b0026000003006f00021112007000023000008d40134000000001020300000000f110123450100001",
		},
		{
			name: "PWSRestartIndication",
			msg: &PWSRestartIndication{
				ECGIListForRestart:            []EUTRANCGI{pwsECGI(0x1234501)},
				GlobalENBID:                   pwsENB(),
				TAIListForRestart:             []TAI{pwsTAI(1), pwsTAI(2)},
				EmergencyAreaIDListForRestart: []EmergencyAreaID{{0x01, 0x02, 0x03}},
			},
			parse:  func(b []byte) (any, error) { return ParsePWSRestartIndication(b) },
			choice: isInitiating,
			golden: "0031403600000400b60009000000f11012345010003b00080000f1100012345000bc000e00010000f11000010000f110000200be000400010203",
		},
		{
			name: "PWSFailureIndication",
			msg: &PWSFailureIndication{
				PWSFailedECGIList: []EUTRANCGI{pwsECGI(0x1234501), pwsECGI(0x1234502)},
				GlobalENBID:       pwsENB(),
			},
			parse:  func(b []byte) (any, error) { return ParsePWSFailureIndication(b) },
			choice: isInitiating,
			golden: "0033402300000200de0010010000f1101234501000f11012345020003b00080000f11000123450",
		},
		{
			name: "RerouteNASRequest",
			msg: &RerouteNASRequest{
				ENBUES1APID:    1,
				MMEUES1APID:    Ptr(MMEUES1APID(2)),
				S1Message:      pwsInitialUEMessage(),
				MMEGroupID:     MMEGroupID{0x80, 0x02},
				AdditionalGUTI: &AdditionalGUTI{GUMMEI: pwsGUMMEI(), MTMSI: 0xdeadbeef},
				UEUsageType:    Ptr(uint8(128)),
			},
			parse:  func(b []byte) (any, error) { return ParseRerouteNASRequest(b) },
			choice: isInitiating,
			golden: "0034005d00000600080002000100004002000200e100302f000c402b000005000800020001001a000302074a004300060000f1100001006440080000f11012345010008640013000df0002800200e0400b0000f110800101deadbeef00e6400180",
		},
		{
			name: "RerouteNASRequestEmptyS1Message",
			msg: &RerouteNASRequest{
				ENBUES1APID: 1,
				S1Message:   S1Message{},
				MMEGroupID:  MMEGroupID{0x80, 0x02},
			},
			parse:  func(b []byte) (any, error) { return ParseRerouteNASRequest(b) },
			choice: isInitiating,
		},
	}, uEContextReleaseRequestGoldenCases()...)
}

func TestNewMessagesRoundTrip(t *testing.T) {
	dump := os.Getenv("S1AP_DUMP_NEW_MESSAGES") != ""

	for _, tc := range newMessageCases() {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.msg.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if dump {
				t.Logf("%s %s", tc.name, hex.EncodeToString(b))
			}

			if tc.golden != "" && hex.EncodeToString(b) != tc.golden {
				t.Errorf("bytes = %x, want %s", b, tc.golden)
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			if !tc.choice(pdu) {
				t.Fatalf("PDU alternative %T", pdu)
			}

			got, err := tc.parse(pdu.value())
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tc.msg) {
				t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, tc.msg)
			}

			again, err := got.(interface{ Marshal() ([]byte, error) }).Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if hex.EncodeToString(again) != hex.EncodeToString(b) {
				t.Errorf("re-encode = %x, want %x", again, b)
			}
		})
	}
}

func TestNewMessagesWireCriticality(t *testing.T) {
	type wireIE struct {
		id   ProtocolIEID
		crit Criticality
	}

	cases := map[string][]wireIE{
		"OverloadStart":               {{IDOverloadResponse, CriticalityReject}, {IDGUMMEIList, CriticalityIgnore}, {IDTrafficLoadReductionIndication, CriticalityIgnore}},
		"OverloadStop":                {{IDGUMMEIList, CriticalityIgnore}},
		"WriteReplaceWarningRequest":  {{IDMessageIdentifier, CriticalityReject}, {IDSerialNumber, CriticalityReject}, {IDWarningAreaList, CriticalityIgnore}, {IDRepetitionPeriod, CriticalityReject}, {IDExtendedRepetitionPeriod, CriticalityReject}, {IDNumberofBroadcastRequest, CriticalityReject}, {IDWarningType, CriticalityIgnore}, {IDWarningSecurityInfo, CriticalityIgnore}, {IDDataCodingScheme, CriticalityIgnore}, {IDWarningMessageContents, CriticalityIgnore}, {IDConcurrentWarningMessageIndicator, CriticalityReject}, {IDWarningAreaCoordinates, CriticalityIgnore}},
		"WriteReplaceWarningResponse": {{IDMessageIdentifier, CriticalityReject}, {IDSerialNumber, CriticalityReject}, {IDBroadcastCompletedAreaList, CriticalityIgnore}, {IDCriticalityDiagnostics, CriticalityIgnore}},
		"KillRequest":                 {{IDMessageIdentifier, CriticalityReject}, {IDSerialNumber, CriticalityReject}, {IDWarningAreaList, CriticalityIgnore}, {IDKillAllWarningMessages, CriticalityReject}},
		"KillResponse":                {{IDMessageIdentifier, CriticalityReject}, {IDSerialNumber, CriticalityReject}, {IDBroadcastCancelledAreaList, CriticalityIgnore}},
		"PWSRestartIndication":        {{IDECGIListForRestart, CriticalityReject}, {IDGlobalENBID, CriticalityReject}, {IDTAIListForRestart, CriticalityReject}, {IDEmergencyAreaIDListForRestart, CriticalityReject}},
		"PWSFailureIndication":        {{IDPWSfailedECGIList, CriticalityReject}, {IDGlobalENBID, CriticalityReject}},
		"UEContextReleaseRequestGWContextReleaseIndication": {{IDMMEUES1APID, CriticalityReject}, {IDENBUES1APID, CriticalityReject}, {IDCause, CriticalityIgnore}, {IDGWContextReleaseIndication, CriticalityReject}},
		"RerouteNASRequest": {{IDENBUES1APID, CriticalityReject}, {IDMMEUES1APID, CriticalityIgnore}, {IDS1Message, CriticalityReject}, {IDMMEGroupID, CriticalityReject}, {IDAdditionalGUTI, CriticalityIgnore}, {IDUEUsageType, CriticalityIgnore}},
	}

	for _, tc := range newMessageCases() {
		want, ok := cases[tc.name]
		if !ok {
			continue
		}

		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.msg.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			r := per.NewReader(pdu.value())
			if _, err := r.ReadBit(); err != nil {
				t.Fatal(err)
			}

			got, err := decodeIEContainer(r, per.Aligned)
			if err != nil {
				t.Fatal(err)
			}

			if len(got) != len(want) {
				t.Fatalf("%d IEs, want %d", len(got), len(want))
			}

			for i := range want {
				if got[i].id != want[i].id || got[i].crit != want[i].crit {
					t.Errorf("IE %d = {%s, %s}, want {%s, %s}", i, got[i].id, got[i].crit, want[i].id, want[i].crit)
				}
			}
		})
	}
}

func TestWarningAreaListNeedsExactlyOneAlternative(t *testing.T) {
	for name, l := range map[string]WarningAreaList{
		"none": {},
		"two":  {CellIDList: []EUTRANCGI{pwsECGI(1)}, TAIList: []TAI{pwsTAI(1)}},
	} {
		if _, err := per.Marshal(l); err == nil {
			t.Errorf("%s: Marshal() = nil error", name)
		}
	}

	if _, err := per.Marshal(BroadcastCompletedAreaList{}); err == nil {
		t.Error("empty BroadcastCompletedAreaList encoded")
	}

	if _, err := per.Marshal(BroadcastCancelledAreaList{}); err == nil {
		t.Error("empty BroadcastCancelledAreaList encoded")
	}
}

func TestWarningAreaListExtensionAlternativeNotComprehended(t *testing.T) {
	var l WarningAreaList
	if err := per.Unmarshal([]byte{0x80, 0x01, 0x00}, &l); !errors.Is(err, errNotComprehended) {
		t.Fatalf("err = %v, want errNotComprehended", err)
	}
}

func TestOverloadActionExtensionValues(t *testing.T) {
	for v := OverloadActionRejectNonEmergencyMODT; v <= OverloadActionNotAcceptMODataOrDelayTolerantAccessFromCPCIoT; v++ {
		b, err := per.Marshal(OverloadResponse{OverloadAction: v})
		if err != nil {
			t.Fatalf("%d: %v", v, err)
		}

		var got OverloadResponse
		if err := per.Unmarshal(b, &got); err != nil || got.OverloadAction != v {
			t.Fatalf("%d: got %d, %v", v, got.OverloadAction, err)
		}
	}

	if _, err := per.Marshal(OverloadResponse{OverloadAction: 7}); err == nil {
		t.Error("undefined OverloadAction encoded")
	}

	var got OverloadResponse
	if err := per.Unmarshal([]byte{0x42, 0x00}, &got); !errors.Is(err, errNotComprehended) {
		t.Errorf("unknown extension value: err = %v, want errNotComprehended", err)
	}
}

func TestOverloadStartWithoutResponseRejected(t *testing.T) {
	_, err := ParseOverloadStart(container(t))

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("err = %v, want an AbstractSyntaxError", err)
	}
}
