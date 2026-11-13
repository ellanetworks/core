// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/per"
)

func pwsPLMN() PLMNIdentity { return PLMNIdentity{0x00, 0xf1, 0x10} }

func pwsTAI() TAI { return TAI{PLMNIdentity: pwsPLMN(), TAC: 1} }

func pwsNRCGI() NRCGI { return NRCGI{PLMNIdentity: pwsPLMN(), NRCellIdentity: 0x123456789} }

func pwsEUTRACGI() EUTRACGI { return EUTRACGI{PLMNIdentity: pwsPLMN(), EUTRACellIdentity: 0x1234567} }

func pwsCBSData() WarningMessageContents {
	page := make([]byte, 82)
	copy(page, []byte{0xd4, 0xf2, 0x9c, 0x0e})

	return append(append(WarningMessageContents{0x01}, page...), 0x04)
}

func pwsGNB() GlobalRANNodeID {
	return GlobalRANNodeID{Kind: RANNodeIDGNB, PLMNIdentity: pwsPLMN(), Value: 1, Bits: 22}
}

func goldOverloadStart() *OverloadStart {
	return &OverloadStart{
		AMFOverloadResponse:               &OverloadResponse{OverloadAction: OverloadActionRejectRRCCRSignalling},
		AMFTrafficLoadReductionIndication: Ptr(TrafficLoadReductionIndication(50)),
		OverloadStartNSSAIList: OverloadStartNSSAIList{{
			SliceOverloadList:                   []SliceOverloadItem{{SNSSAI: SNSSAI{SST: 1, SD: &SD{0x00, 0x00, 0x01}}}},
			SliceOverloadResponse:               &OverloadResponse{OverloadAction: OverloadActionPermitEmergencySessionsAndMTServicesOnly},
			SliceTrafficLoadReductionIndication: Ptr(TrafficLoadReductionIndication(20)),
		}},
	}
}

func goldGUAMIList() GUAMIList {
	return GUAMIList{
		{GUAMI: GUAMI{PLMNIdentity: pwsPLMN(), AMFRegionID: 0xca, AMFSetID: 0x3f8, AMFPointer: 0x01}},
		{GUAMI: GUAMI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}, AMFRegionID: 0x01, AMFSetID: 0x001, AMFPointer: 0x3f}},
	}
}

func goldOverloadStartGUAMIList() *OverloadStart {
	m := goldOverloadStart()
	m.GUAMIList = goldGUAMIList()

	return m
}

func goldOverloadStopGUAMIList() *OverloadStop {
	return &OverloadStop{GUAMIList: goldGUAMIList()}
}

func goldWriteReplaceWarningRequest() *WriteReplaceWarningRequest {
	return &WriteReplaceWarningRequest{
		MessageIdentifier:           0x1112,
		SerialNumber:                0x3456,
		WarningAreaList:             &WarningAreaList{NRCGIs: []NRCGI{pwsNRCGI()}},
		RepetitionPeriod:            10,
		NumberOfBroadcastsRequested: 1,
		WarningType:                 &WarningType{0x05, 0x80},
		DataCodingScheme:            Ptr(DataCodingScheme(0x0f)),
		WarningMessageContents:      pwsCBSData(),
		ConcurrentWarningMessageInd: Ptr(ConcurrentWarningMessageIndTrue),
		WarningAreaCoordinates:      WarningAreaCoordinates{0xaa, 0xbb},
	}
}

func goldWriteReplaceWarningResponse() *WriteReplaceWarningResponse {
	return &WriteReplaceWarningResponse{
		MessageIdentifier: 0x1112,
		SerialNumber:      0x3456,
		BroadcastCompletedAreaList: &BroadcastCompletedAreaList{TAIBroadcastNR: []TAIBroadcastNRItem{{
			TAI:                 pwsTAI(),
			CompletedCellsInTAI: []CompletedCellsInTAINRItem{{NRCGI: pwsNRCGI()}},
		}}},
	}
}

func goldPWSCancelRequest() *PWSCancelRequest {
	return &PWSCancelRequest{
		MessageIdentifier:        0x1112,
		SerialNumber:             0x3456,
		WarningAreaList:          &WarningAreaList{TAIs: []TAI{pwsTAI()}},
		CancelAllWarningMessages: Ptr(CancelAllWarningMessagesTrue),
	}
}

func goldPWSCancelResponse() *PWSCancelResponse {
	return &PWSCancelResponse{
		MessageIdentifier: 0x1112,
		SerialNumber:      0x3456,
		BroadcastCancelledAreaList: &BroadcastCancelledAreaList{CellIDCancelledNR: []CellIDCancelledNRItem{{
			NRCGI:              pwsNRCGI(),
			NumberOfBroadcasts: 3,
		}}},
	}
}

func goldPWSRestartIndication() *PWSRestartIndication {
	return &PWSRestartIndication{
		CellIDListForRestart:          CellIDListForRestart{NRCGIs: []NRCGI{pwsNRCGI()}},
		GlobalRANNodeID:               pwsGNB(),
		TAIListForRestart:             TAIListForRestart{pwsTAI()},
		EmergencyAreaIDListForRestart: EmergencyAreaIDListForRestart{{0x01, 0x02, 0x03}},
	}
}

func goldPWSFailureIndication() *PWSFailureIndication {
	return &PWSFailureIndication{
		PWSFailedCellIDList: PWSFailedCellIDList{EUTRACGIs: []EUTRACGI{pwsEUTRACGI()}},
		GlobalRANNodeID:     pwsGNB(),
	}
}

func goldRerouteNASRequest() *RerouteNASRequest {
	return &RerouteNASRequest{
		RANUENGAPID:  5,
		AMFUENGAPID:  Ptr(AMFUENGAPID(7)),
		NGAPMessage:  NGAPMessage{0x00, 0x0f, 0x40, 0x00},
		AMFSetID:     1,
		AllowedNSSAI: AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
		SourceToTargetAMFInformationReroute: &SourceToTargetAMFInformationReroute{
			RejectedNSSAIinPLMN: &RejectedNSSAIinPLMN{0x01},
		},
		PartiallyAllowedNSSAI: PartiallyAllowedNSSAI{{SNSSAI: SNSSAI{SST: 2}}},
	}
}

type pwsCase struct {
	name    string
	marshal func() ([]byte, error)
	parse   func([]byte) (any, error)
	want    any
	golden  string
}

func pwsCases() []pwsCase {
	return []pwsCase{
		{"OverloadStart", goldOverloadStart().Marshal, func(v []byte) (any, error) { return ParseOverloadStart(v) }, goldOverloadStart(), "0016401d000003000200011000094001620031400c000060000010080000012260"},
		{"OverloadStop", (&OverloadStop{}).Marshal, func(v []byte) (any, error) { return ParseOverloadStop(v) }, &OverloadStop{}, "00170003000000"},
		{"OverloadStartGUAMIList", goldOverloadStartGUAMIList().Marshal, func(v []byte) (any, error) { return ParseOverloadStart(v) }, goldOverloadStartGUAMIList(), "00164031000004000200011000094001620031400c00006000001008000001226001c3401000010000f110cafe010002f83901007f"},
		{"OverloadStopGUAMIList", goldOverloadStopGUAMIList().Marshal, func(v []byte) (any, error) { return ParseOverloadStop(v) }, goldOverloadStopGUAMIList(), "0017001700000101c3401000010000f110cafe010002f83901007f"},
		{"WriteReplaceWarningRequest", goldWriteReplaceWarningRequest().Marshal, func(v []byte) (any, error) { return ParseWriteReplaceWarningRequest(v) }, goldWriteReplaceWarningRequest(), "003300809d00000a002300021112005f00023456007a400c2000000000f110123456789000570002000a002f00020001007d40020580001440010f007b4056005301d4f29c0e000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040011000100008d40040001aabb"},
		{"WriteReplaceWarningResponse", goldWriteReplaceWarningResponse().Marshal, func(v []byte) (any, error) { return ParseWriteReplaceWarningResponse(v) }, goldWriteReplaceWarningResponse(), "20330028000003002300021112005f00023456000d40158000000000f11000000100000000f1101234567890"},
		{"PWSCancelRequest", goldPWSCancelRequest().Marshal, func(v []byte) (any, error) { return ParsePWSCancelRequest(v) }, goldPWSCancelRequest(), "00200022000004002300021112005f00023456007a400a4000000000f110000001000e000100"},
		{"PWSCancelResponse", goldPWSCancelResponse().Marshal, func(v []byte) (any, error) { return ParsePWSCancelResponse(v) }, goldPWSCancelResponse(), "20200021000003002300021112005f00023456000c400e6000000000f11012345678900003"},
		{"PWSRestartIndication", goldPWSRestartIndication().Marshal, func(v []byte) (any, error) { return ParsePWSRestartIndication(v) }, goldPWSRestartIndication(), "002240340000040010000c4000000000f1101234567890001b00080000f110000000040068000900000000f1100000010017000400010203"},
		{"PWSFailureIndication", goldPWSFailureIndication().Marshal, func(v []byte) (any, error) { return ParsePWSFailureIndication(v) }, goldPWSFailureIndication(), "0021401d0000020051000a00000000f11012345670001b00080000f11000000004"},
		{"RerouteNASRequest", goldRerouteNASRequest().Marshal, func(v []byte) (any, error) { return ParseRerouteNASRequest(v) }, goldRerouteNASRequest(), "0024004f000007005500020005000a40020007002a000504000f400000030002004000000002000100ab4021200100000000000000000000000000000000000000000000000000000000000000019e40020002"},
	}
}

func TestPWSOverloadRoundTripGolden(t *testing.T) {
	for _, c := range pwsCases() {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.marshal()
			if err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(b); got != c.golden {
				t.Errorf("PDU = %s, want %s", got, c.golden)
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			got, err := c.parse(pdu.value())
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("round trip = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestPWSOverloadWireCriticality(t *testing.T) {
	type wireIE struct {
		id   ProtocolIEID
		crit Criticality
	}

	tests := []struct {
		name string
		body func(*per.Writer, per.Encoding) error
		want []wireIE
	}{
		{"OverloadStart §9.2.6.14", goldOverloadStart().encodeBody, []wireIE{
			{IDAMFOverloadResponse, CriticalityReject},
			{IDAMFTrafficLoadReductionIndication, CriticalityIgnore},
			{IDOverloadStartNSSAIList, CriticalityIgnore},
		}},
		{"OverloadStart §9.2.6.14 GUAMI List", goldOverloadStartGUAMIList().encodeBody, []wireIE{
			{IDAMFOverloadResponse, CriticalityReject},
			{IDAMFTrafficLoadReductionIndication, CriticalityIgnore},
			{IDOverloadStartNSSAIList, CriticalityIgnore},
			{IDGUAMIList, CriticalityIgnore},
		}},
		{"OverloadStop §9.2.6.15", (&OverloadStop{}).encodeBody, nil},
		{"OverloadStop §9.2.6.15 GUAMI List", goldOverloadStopGUAMIList().encodeBody, []wireIE{
			{IDGUAMIList, CriticalityIgnore},
		}},
		{"WriteReplaceWarningRequest §9.2.8.1", func() func(*per.Writer, per.Encoding) error {
			m := goldWriteReplaceWarningRequest()
			m.WarningSecurityInfo = &WarningSecurityInfo{}

			return m.encodeBody
		}(), []wireIE{
			{IDMessageIdentifier, CriticalityReject},
			{IDSerialNumber, CriticalityReject},
			{IDWarningAreaList, CriticalityIgnore},
			{IDRepetitionPeriod, CriticalityReject},
			{IDNumberOfBroadcastsRequested, CriticalityReject},
			{IDWarningType, CriticalityIgnore},
			{IDWarningSecurityInfo, CriticalityIgnore},
			{IDDataCodingScheme, CriticalityIgnore},
			{IDWarningMessageContents, CriticalityIgnore},
			{IDConcurrentWarningMessageInd, CriticalityReject},
			{IDWarningAreaCoordinates, CriticalityIgnore},
		}},
		{"WriteReplaceWarningResponse §9.2.8.2", func() func(*per.Writer, per.Encoding) error {
			m := goldWriteReplaceWarningResponse()
			m.CriticalityDiagnostics = &CriticalityDiagnostics{}

			return m.encodeBody
		}(), []wireIE{
			{IDMessageIdentifier, CriticalityReject},
			{IDSerialNumber, CriticalityReject},
			{IDBroadcastCompletedAreaList, CriticalityIgnore},
			{IDCriticalityDiagnostics, CriticalityIgnore},
		}},
		{"PWSCancelRequest §9.2.8.3", goldPWSCancelRequest().encodeBody, []wireIE{
			{IDMessageIdentifier, CriticalityReject},
			{IDSerialNumber, CriticalityReject},
			{IDWarningAreaList, CriticalityIgnore},
			{IDCancelAllWarningMessages, CriticalityReject},
		}},
		{"PWSCancelResponse §9.2.8.4", func() func(*per.Writer, per.Encoding) error {
			m := goldPWSCancelResponse()
			m.CriticalityDiagnostics = &CriticalityDiagnostics{}

			return m.encodeBody
		}(), []wireIE{
			{IDMessageIdentifier, CriticalityReject},
			{IDSerialNumber, CriticalityReject},
			{IDBroadcastCancelledAreaList, CriticalityIgnore},
			{IDCriticalityDiagnostics, CriticalityIgnore},
		}},
		{"PWSRestartIndication §9.2.8.5", goldPWSRestartIndication().encodeBody, []wireIE{
			{IDCellIDListForRestart, CriticalityReject},
			{IDGlobalRANNodeID, CriticalityReject},
			{IDTAIListForRestart, CriticalityReject},
			{IDEmergencyAreaIDListForRestart, CriticalityReject},
		}},
		{"PWSFailureIndication §9.2.8.6", goldPWSFailureIndication().encodeBody, []wireIE{
			{IDPWSFailedCellIDList, CriticalityReject},
			{IDGlobalRANNodeID, CriticalityReject},
		}},
		{"RerouteNASRequest §9.2.5.5", func() func(*per.Writer, per.Encoding) error {
			m := goldRerouteNASRequest()
			m.SourceToTargetAMFInformationReroute.ConfiguredNSSAI = &ConfiguredNSSAI{}
			m.SourceToTargetAMFInformationReroute.RejectedNSSAIinTA = &RejectedNSSAIinTA{}

			return m.encodeBody
		}(), []wireIE{
			{IDRANUENGAPID, CriticalityReject},
			{IDAMFUENGAPID, CriticalityIgnore},
			{IDNGAPMessage, CriticalityReject},
			{IDAMFSetID, CriticalityReject},
			{IDAllowedNSSAI, CriticalityReject},
			{IDSourceToTargetAMFInformationReroute, CriticalityIgnore},
			{IDPartiallyAllowedNSSAI, CriticalityIgnore},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wireIEs(t, tt.body)
			if len(got) != len(tt.want) {
				t.Fatalf("encoded %d IEs, want %d", len(got), len(tt.want))
			}

			for i, w := range tt.want {
				if got[i].id != w.id || got[i].crit != w.crit {
					t.Errorf("IE %d: got %v/%v, want %v/%v", i, got[i].id, got[i].crit, w.id, w.crit)
				}
			}
		})
	}
}

func pwsRoundTrip[T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](t *testing.T, in PT) {
	t.Helper()

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	out := PT(new(T))
	if err := out.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(out, in) {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}

func TestPWSAreaListAlternativesRoundTrip(t *testing.T) {
	eai := EmergencyAreaID{0x0a, 0x0b, 0x0c}

	for _, l := range []WarningAreaList{
		{EUTRACGIs: []EUTRACGI{pwsEUTRACGI()}},
		{NRCGIs: []NRCGI{pwsNRCGI()}},
		{TAIs: []TAI{pwsTAI()}},
		{EmergencyAreaIDs: []EmergencyAreaID{eai}},
	} {
		pwsRoundTrip(t, &l)
	}

	for _, l := range []BroadcastCompletedAreaList{
		{CellIDBroadcastEUTRA: []CellIDBroadcastEUTRAItem{{EUTRACGI: pwsEUTRACGI()}}},
		{TAIBroadcastEUTRA: []TAIBroadcastEUTRAItem{{TAI: pwsTAI(), CompletedCellsInTAI: []CompletedCellsInTAIEUTRAItem{{EUTRACGI: pwsEUTRACGI()}}}}},
		{EmergencyAreaIDBroadcastEUTRA: []EmergencyAreaIDBroadcastEUTRAItem{{EmergencyAreaID: eai, CompletedCellsInEAI: []CompletedCellsInEAIEUTRAItem{{EUTRACGI: pwsEUTRACGI()}}}}},
		{CellIDBroadcastNR: []CellIDBroadcastNRItem{{NRCGI: pwsNRCGI()}}},
		{TAIBroadcastNR: []TAIBroadcastNRItem{{TAI: pwsTAI(), CompletedCellsInTAI: []CompletedCellsInTAINRItem{{NRCGI: pwsNRCGI()}}}}},
		{EmergencyAreaIDBroadcastNR: []EmergencyAreaIDBroadcastNRItem{{EmergencyAreaID: eai, CompletedCellsInEAI: []CompletedCellsInEAINRItem{{NRCGI: pwsNRCGI()}}}}},
	} {
		pwsRoundTrip(t, &l)
	}

	for _, l := range []BroadcastCancelledAreaList{
		{CellIDCancelledEUTRA: []CellIDCancelledEUTRAItem{{EUTRACGI: pwsEUTRACGI(), NumberOfBroadcasts: 1}}},
		{TAICancelledEUTRA: []TAICancelledEUTRAItem{{TAI: pwsTAI(), CancelledCellsInTAI: []CancelledCellsInTAIEUTRAItem{{EUTRACGI: pwsEUTRACGI(), NumberOfBroadcasts: 2}}}}},
		{EmergencyAreaIDCancelledEUTRA: []EmergencyAreaIDCancelledEUTRAItem{{EmergencyAreaID: eai, CancelledCellsInEAI: []CancelledCellsInEAIEUTRAItem{{EUTRACGI: pwsEUTRACGI(), NumberOfBroadcasts: 3}}}}},
		{CellIDCancelledNR: []CellIDCancelledNRItem{{NRCGI: pwsNRCGI(), NumberOfBroadcasts: 65535}}},
		{TAICancelledNR: []TAICancelledNRItem{{TAI: pwsTAI(), CancelledCellsInTAI: []CancelledCellsInTAINRItem{{NRCGI: pwsNRCGI()}}}}},
		{EmergencyAreaIDCancelledNR: []EmergencyAreaIDCancelledNRItem{{EmergencyAreaID: eai, CancelledCellsInEAI: []CancelledCellsInEAINRItem{{NRCGI: pwsNRCGI()}}}}},
	} {
		pwsRoundTrip(t, &l)
	}

	pwsRoundTrip(t, &CellIDListForRestart{EUTRACGIs: []EUTRACGI{pwsEUTRACGI()}})
	pwsRoundTrip(t, &PWSFailedCellIDList{NRCGIs: []NRCGI{pwsNRCGI()}})
}

func TestPWSChoiceNeedsExactlyOneAlternative(t *testing.T) {
	for _, m := range []per.Marshaler{
		WarningAreaList{},
		WarningAreaList{TAIs: []TAI{pwsTAI()}, NRCGIs: []NRCGI{pwsNRCGI()}},
		BroadcastCompletedAreaList{},
		BroadcastCancelledAreaList{},
		CellIDListForRestart{},
		PWSFailedCellIDList{EUTRACGIs: []EUTRACGI{pwsEUTRACGI()}, NRCGIs: []NRCGI{pwsNRCGI()}},
	} {
		if err := m.MarshalPER(per.NewWriter(), per.Aligned); err == nil {
			t.Errorf("%T %+v: Marshal accepted it", m, m)
		}
	}
}

func TestPWSChoiceExtensionsNotComprehended(t *testing.T) {
	cases := []struct {
		name string
		u    per.Unmarshaler
		idx  int64
		ub   int64
	}{
		{"WarningAreaList", &WarningAreaList{}, warningAreaListChoiceExtensions, warningAreaListAlternatives - 1},
		{"BroadcastCompletedAreaList", &BroadcastCompletedAreaList{}, broadcastAreaChoiceExtensions, broadcastAreaAlternatives - 1},
		{"BroadcastCancelledAreaList", &BroadcastCancelledAreaList{}, broadcastAreaChoiceExtensions, broadcastAreaAlternatives - 1},
		{"CellIDListForRestart", &CellIDListForRestart{}, cellListChoiceExtensions, cellListAlternatives - 1},
		{"PWSFailedCellIDList", &PWSFailedCellIDList{}, cellListChoiceExtensions, cellListAlternatives - 1},
		{"OverloadResponse", &OverloadResponse{}, overloadResponseChoiceExtensions, overloadResponseAlternatives - 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, c.ub, c.idx); err != nil {
				t.Fatal(err)
			}

			if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, 999); err != nil {
				t.Fatal(err)
			}

			if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(CriticalityIgnore)); err != nil {
				t.Fatal(err)
			}

			if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0x00}); err != nil {
				t.Fatal(err)
			}

			err := c.u.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned)
			if !errors.Is(err, errNotComprehended) {
				t.Fatalf("err = %v, want errNotComprehended", err)
			}
		})
	}
}

func TestPWSMandatoryRejectIEAbsentRejects(t *testing.T) {
	if _, err := ParseWriteReplaceWarningRequest(container(t)); err == nil {
		t.Error("ParseWriteReplaceWarningRequest accepted a request without its mandatory IEs")
	}

	if _, err := ParseRerouteNASRequest(container(t)); err == nil {
		t.Error("ParseRerouteNASRequest accepted a request without its mandatory IEs")
	}
}

func TestTrafficLoadReductionIndicationRange(t *testing.T) {
	for _, v := range []TrafficLoadReductionIndication{0, 100} {
		if err := v.MarshalPER(per.NewWriter(), per.Aligned); err == nil {
			t.Errorf("TrafficLoadReductionIndication %d encoded", v)
		}
	}
}

func TestOverloadStopGUAMIListBounds(t *testing.T) {
	if _, err := (&OverloadStop{GUAMIList: make(GUAMIList, maxnoofGUAMIs+1)}).Marshal(); err == nil {
		t.Fatal("encoded a GUAMI List longer than maxnoofGUAMIs")
	}

	if _, err := (&OverloadStop{GUAMIList: make(GUAMIList, maxnoofGUAMIs)}).Marshal(); err != nil {
		t.Fatalf("maxnoofGUAMIs items: %v", err)
	}
}
