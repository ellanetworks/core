// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"os/exec"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

const goldenSourceNGRANContainer = "7001800040050501004000050140200145030002f8391234567890ff10320002f8390abcdef1800fff9ff604020b0002f83900001011000064"

const goldenS1APLastVisitedEUTRANCellInformation = "0002f83900001011000064"

func containerPLMN() PLMNIdentity { return PLMNIdentity{0x02, 0xf8, 0x39} }

func goldSourceNGRANContainer() *SourceNGRANNodeToTargetNGRANNodeTransparentContainer {
	return &SourceNGRANNodeToTargetNGRANNodeTransparentContainer{
		RRCContainer: RRCContainer{0x80},
		PDUSessionResourceInformationList: PDUSessionResourceInformationList{
			{
				PDUSessionID: 5,
				QosFlowInformationList: QosFlowInformationList{
					{QosFlowIdentifier: 1, DLForwarding: Ptr(DLForwardingProposed)},
					{QosFlowIdentifier: 2},
				},
				DRBsToQosFlowsMappingList: DRBsToQosFlowsMappingList{
					{DRBID: 1, AssociatedQosFlowList: AssociatedQosFlowList{{QosFlowIdentifier: 1, QosFlowMappingIndication: Ptr(QosFlowMappingDL)}, {QosFlowIdentifier: 2}}},
				},
			},
		},
		ERABInformationList: ERABInformationList{{ERABID: 5, DLForwarding: Ptr(DLForwardingProposed)}, {ERABID: 6}},
		TargetCellID:        NGRANCGI{NRCGI: &NRCGI{PLMNIdentity: containerPLMN(), NRCellIdentity: 0x123456789}},
		IndexToRFSP:         Ptr(IndexToRFSP(256)),
		UEHistoryInformation: UEHistoryInformation{
			{LastVisitedCellInformation: LastVisitedCellInformation{NGRANCell: &LastVisitedNGRANCellInformation{
				GlobalCellID:                          NGRANCGI{EUTRACGI: &EUTRACGI{PLMNIdentity: containerPLMN(), EUTRACellIdentity: 0x0abcdef}},
				CellType:                              CellType{CellSize: CellSizeLarge},
				TimeUEStayedInCell:                    4095,
				TimeUEStayedInCellEnhancedGranularity: Ptr(TimeUEStayedInCellEnhancedGranularity(40950)),
				HOCauseValue:                          Ptr(CauseRadioNetworkHandoverDesirableForRadio.Cause()),
			}}},
			{LastVisitedCellInformation: LastVisitedCellInformation{EUTRANCell: Ptr(LastVisitedEUTRANCellInformation(mustHexBytes(goldenS1APLastVisitedEUTRANCellInformation)))}},
		},
	}
}

func mustHexBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return b
}

func TestSourceNGRANNodeToTargetNGRANNodeTransparentContainerGolden(t *testing.T) {
	in := goldSourceNGRANContainer()

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenSourceNGRANContainer {
		t.Fatalf("encoded\n got %s\nwant %s", got, goldenSourceNGRANContainer)
	}

	out, err := ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b)
	if err != nil {
		t.Fatal(err)
	}

	if !out.Equal(*in) {
		t.Fatalf("decoded\n got %+v\nwant %+v", out, in)
	}

	if out.TargetCellID.NRCGI == nil || out.TargetCellID.NRCGI.NRCellIdentity != 0x123456789 || out.TargetCellID.NRCGI.PLMNIdentity != containerPLMN() {
		t.Fatalf("targetCell-ID = %+v", out.TargetCellID)
	}

	again, err := out.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(again, b) {
		t.Fatalf("re-encoded %x, want %x", again, b)
	}
}

func TestSourceNGRANNodeToTargetNGRANNodeTransparentContainerMinimal(t *testing.T) {
	in := &SourceNGRANNodeToTargetNGRANNodeTransparentContainer{
		RRCContainer:         RRCContainer{0x80},
		TargetCellID:         NGRANCGI{EUTRACGI: &EUTRACGI{PLMNIdentity: containerPLMN(), EUTRACellIdentity: 0x0000101}},
		UEHistoryInformation: UEHistoryInformation{{LastVisitedCellInformation: LastVisitedCellInformation{GERANCell: Ptr(LastVisitedGERANCellInformation{0x01})}}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b)
	if err != nil {
		t.Fatal(err)
	}

	if !out.Equal(*in) {
		t.Fatalf("decoded\n got %+v\nwant %+v", out, in)
	}
}

func TestSourceNGRANNodeToTargetNGRANNodeTransparentContainerRejectsInvalid(t *testing.T) {
	for name, c := range map[string]*SourceNGRANNodeToTargetNGRANNodeTransparentContainer{
		"no target cell": {
			RRCContainer:         RRCContainer{0x80},
			UEHistoryInformation: UEHistoryInformation{{LastVisitedCellInformation: LastVisitedCellInformation{UTRANCell: Ptr(LastVisitedUTRANCellInformation{0x01})}}},
		},
		"two target cells": {
			RRCContainer:         RRCContainer{0x80},
			TargetCellID:         NGRANCGI{NRCGI: &NRCGI{}, EUTRACGI: &EUTRACGI{}},
			UEHistoryInformation: UEHistoryInformation{{LastVisitedCellInformation: LastVisitedCellInformation{UTRANCell: Ptr(LastVisitedUTRANCellInformation{0x01})}}},
		},
		"empty history": {
			RRCContainer: RRCContainer{0x80},
			TargetCellID: NGRANCGI{NRCGI: &NRCGI{}},
		},
		"history cell without alternative": {
			RRCContainer:         RRCContainer{0x80},
			TargetCellID:         NGRANCGI{NRCGI: &NRCGI{}},
			UEHistoryInformation: UEHistoryInformation{{}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Marshal(); err == nil {
				t.Fatal("Marshal succeeded")
			}
		})
	}
}

func handTransparentExtensions(t *testing.T, w *per.Writer, crit Criticality) {
	t.Helper()

	w.AlignToByte()
	mustNil(t, per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxProtocolExtensions, 2))

	for i, id := range []int64{1000, 999} {
		mustNil(t, per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, id))
		mustNil(t, per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(crit)))
		mustNil(t, per.EncodeOpenTypeBytes(w, per.Aligned, bytes.Repeat([]byte{0x5a}, 1+i*130)))
	}
}

func handAdditions(t *testing.T, w *per.Writer) {
	t.Helper()

	mustNil(t, per.EncodeNormallySmallLength(w, per.Aligned, 2, func(int64) error {
		w.WriteBit(false)
		w.WriteBit(true)

		return nil
	}))
	mustNil(t, per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0x77, 0x88}))
}

func handChoiceExtension(t *testing.T, w *per.Writer) {
	t.Helper()

	mustNil(t, encodeContainerField(w, ieField{ID: 998, Criticality: CriticalityIgnore, Value: apimpl.RawValue([]byte{0x01, 0x02, 0x03})}))
}

func handSourceNGRANContainer(t *testing.T, extCrit Criticality) []byte {
	t.Helper()

	w := per.NewWriter()
	enc := per.Aligned

	w.WriteBit(true)
	w.WriteBit(false)
	w.WriteBit(false)
	w.WriteBit(false)
	w.WriteBit(true)

	mustNil(t, RRCContainer{0x80}.MarshalPER(w, enc))
	mustNil(t, per.EncodeConstrainedWholeNumber(w, enc, 0, ngranCGIAlternatives-1, ngranCGIChoiceExtensions))
	handChoiceExtension(t, w)

	mustNil(t, per.EncodeLength(w, enc, 1, maxnoofCellsinUEHistoryInfo, true, 2, func(int64) error {
		w.WriteBit(true)
		w.WriteBit(true)
		mustNil(t, per.EncodeConstrainedWholeNumber(w, enc, 0, lastVisitedCellInformationAlternatives-1, lastVisitedCellInformationChoiceExtensions))
		handChoiceExtension(t, w)
		handTransparentExtensions(t, w, CriticalityReject)
		handAdditions(t, w)

		w.WriteBit(false)
		w.WriteBit(false)
		mustNil(t, per.EncodeConstrainedWholeNumber(w, enc, 0, lastVisitedCellInformationAlternatives-1, lastVisitedCellInformationNGRAN))
		w.WriteBit(true)
		w.WriteBit(false)
		w.WriteBit(false)
		w.WriteBit(true)
		mustNil(t, per.EncodeConstrainedWholeNumber(w, enc, 0, ngranCGIAlternatives-1, ngranCGINR))
		mustNil(t, (&NRCGI{PLMNIdentity: containerPLMN(), NRCellIdentity: 1}).MarshalPER(w, enc))
		w.WriteBit(true)
		w.WriteBit(true)
		mustNil(t, CellSizeSmall.MarshalPER(w, enc))
		handTransparentExtensions(t, w, CriticalityIgnore)
		handAdditions(t, w)
		mustNil(t, TimeUEStayedInCell(7).MarshalPER(w, enc))
		handTransparentExtensions(t, w, CriticalityReject)
		handAdditions(t, w)

		return nil
	}))

	handTransparentExtensions(t, w, extCrit)
	handAdditions(t, w)
	w.AlignToByte()

	return w.Bytes()
}

func TestSourceNGRANNodeToTargetNGRANNodeTransparentContainerLossless(t *testing.T) {
	for _, crit := range []Criticality{CriticalityIgnore, CriticalityReject} {
		t.Run(crit.String(), func(t *testing.T) {
			b := handSourceNGRANContainer(t, crit)

			c, err := DecodeSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b)
			if err != nil {
				t.Fatal(err)
			}

			if d := c.Diagnostics(); len(d.IEs) == 0 {
				t.Fatal("unknown extensions and alternatives in the container are not reported")
			}

			if c.TargetCellID.NRCGI != nil || c.TargetCellID.EUTRACGI != nil || c.TargetCellID.ChoiceExtension == nil || c.TargetCellID.ChoiceExtension.ID != 998 {
				t.Fatalf("targetCell-ID = %+v", c.TargetCellID)
			}

			if len(c.UEHistoryInformation) != 2 || c.UEHistoryInformation[1].LastVisitedCellInformation.NGRANCell == nil {
				t.Fatalf("UE history = %+v", c.UEHistoryInformation)
			}

			again, err := c.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(again, b) {
				t.Fatalf("re-encoded\n got  %x\n want %x", again, b)
			}
		})
	}
}

func TestSourceNGRANNodeToTargetNGRANNodeTransparentContainerTruncated(t *testing.T) {
	b, err := goldSourceNGRANContainer().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	for n := range len(b) - 1 {
		if _, err := ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b[:n]); err == nil {
			t.Errorf("parse of %d/%d octets succeeded", n, len(b))
		}
	}
}

func TestSourceNGRANNodeToTargetNGRANNodeTransparentContainerExtensionValue(t *testing.T) {
	w := per.NewWriter()
	w.WriteBit(false)
	w.WriteBit(false)
	w.WriteBit(false)
	w.WriteBit(true)
	w.WriteBit(false)
	mustNil(t, RRCContainer{0x80}.MarshalPER(w, per.Aligned))
	mustNil(t, (&NGRANCGI{NRCGI: &NRCGI{}}).MarshalPER(w, per.Aligned))
	mustNil(t, per.EncodeInteger(w, per.Aligned, per.Bounds{LB: 1, HasLB: true, UB: 256, HasUB: true, Extensible: true}, 300))
	mustNil(t, (UEHistoryInformation{{LastVisitedCellInformation: LastVisitedCellInformation{UTRANCell: Ptr(LastVisitedUTRANCellInformation{0x01})}}}).MarshalPER(w, per.Aligned))
	w.AlignToByte()

	c, err := DecodeSourceNGRANNodeToTargetNGRANNodeTransparentContainer(w.Bytes())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if deref(c.IndexToRFSP) != 300 {
		t.Errorf("IndexToRFSP = %v, want the extension value 300 kept", c.IndexToRFSP)
	}

	re, err := c.Marshal()
	mustNil(t, err)

	if !bytes.Equal(re, w.Bytes()) {
		t.Errorf("re-encode = % x, want % x", re, w.Bytes())
	}

	if _, err := ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(w.Bytes()); err == nil {
		t.Fatal("Parse accepted an IndexToRFSP outside its root, want it reported not comprehended")
	}
}

func goldContainerHandoverRequest() *HandoverRequest {
	c, err := goldSourceNGRANContainer().Marshal()
	if err != nil {
		panic(err)
	}

	transfer, err := (&PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: &GTPTunnel{
			TransportLayerAddress: tlaOf(192, 168, 1, 1), GTPTEID: 1,
		}},
		PDUSessionType: PDUSessionTypeIPv4,
		QosFlowSetupRequest: QosFlowSetupRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: QosFlowLevelQosParameters{
				QosCharacteristics:             QosCharacteristics{NonDynamic5QI: &NonDynamic5QIDescriptor{FiveQI: 9}},
				AllocationAndRetentionPriority: AllocationAndRetentionPriority{PriorityLevelARP: 1},
			},
		}},
	}).Marshal()
	if err != nil {
		panic(err)
	}

	return &HandoverRequest{
		AMFUENGAPID:               1,
		HandoverType:              HandoverTypeIntra5GS,
		Cause:                     Ptr(CauseRadioNetworkHandoverDesirableForRadio.Cause()),
		UEAggregateMaximumBitRate: UEAggregateMaximumBitRate{DL: 1000000000, UL: 1000000000},
		UESecurityCapabilities:    UESecurityCapabilities{NREncryptionAlgorithms: SecurityAlgorithmsFrom(0x8000), NRIntegrityProtectionAlgorithms: SecurityAlgorithmsFrom(0x8000)},
		SecurityContext:           SecurityContext{NextHopChainingCount: 1},
		PDUSessionResourceSetupListHOReq: PDUSessionResourceSetupListHOReq{{
			PDUSessionID: 5, SNSSAI: SNSSAI{SST: 1}, Transfer: transfer,
		}},
		AllowedNSSAI:                       AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
		SourceToTargetTransparentContainer: c,
		GUAMI:                              GUAMI{PLMNIdentity: containerPLMN(), AMFRegionID: 1, AMFSetID: 1},
		AMFUENGAPID2:                       Ptr(AMFUENGAPID(3)),
		TargetRANNodeID:                    Ptr(NewGlobalGNBID(containerPLMN(), 0x12345, 22)),
	}
}

func goldPathSwitchRequestSourceAMFGUAMI() *PathSwitchRequest {
	return &PathSwitchRequest{
		RANUENGAPID:             2,
		SourceAMFUENGAPID:       1,
		UserLocationInformation: &UserLocationInformation{NR: &UserLocationInformationNR{NRCGI: NRCGI{PLMNIdentity: containerPLMN(), NRCellIdentity: NRCellIdentity(0x123456789)}, TAI: TAI{PLMNIdentity: containerPLMN(), TAC: 1}}},
		UESecurityCapabilities:  &UESecurityCapabilities{NREncryptionAlgorithms: SecurityAlgorithmsFrom(0x8000), NRIntegrityProtectionAlgorithms: SecurityAlgorithmsFrom(0x8000)},
		PDUSessionResourceToBeSwitchedDLList: PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 5, Transfer: TransferContainer(mustHexBytes("001fc0a80101000000010002"))},
		},
		SourceAMFGUAMI: &GUAMI{PLMNIdentity: containerPLMN(), AMFRegionID: 0xca, AMFSetID: 0x3fe, AMFPointer: 0x3f},
	}
}

func transparentContainerGoldenCases() []goldenRoundTrip {
	return []goldenRoundTrip{
		{
			"HandoverRequestSourceNGRANContainerTargetRANNodeID",
			goldContainerHandoverRequest(),
			func(v []byte) (any, error) { return ParseHandoverRequest(v) },
			"000d0080e000000c000a00020001001d000100000f40020400006e000a0c3b9aca00303b9aca0000770009100008000000000000005d002108000000000000000000000000000000000000000000000000000000000000000000490027000005002021000003008b000a01f0c0a8010100000001008600010000880007000100000900000000000200010065003a397001800040050501004000050140200145030002f8391234567890ff10320002f8390abcdef1800fff9ff604020b0002f83900001011000064001c00070002f83901004001c24002000301c740080002f83900048d14",
		},
		{
			"PathSwitchRequestSourceAMFGUAMI",
			goldPathSwitchRequestSourceAMFGUAMI(),
			func(v []byte) (any, error) { return ParsePathSwitchRequest(v) },
			"0019004e0000060055000200020064000200010079400f4002f839123456789002f83900000100774009100008000000000000004c00100000050c001fc0a8010100000001000201c540070002f839caffbf",
		},
	}
}

func TestTransparentContainerMessagesGolden(t *testing.T) {
	for _, c := range transparentContainerGoldenCases() {
		t.Run(c.name, func(t *testing.T) { checkGoldenRoundTrip(t, c) })
	}
}

func TestTransparentContainerMessagesWireCriticality(t *testing.T) {
	ho := wireIEs(t, goldContainerHandoverRequest().encodeBody)
	if last := ho[len(ho)-1]; last.ID != IDTargetRANNodeID || last.Criticality != CriticalityIgnore {
		t.Fatalf("HandoverRequest last IE = %d/%s, want Target-RAN-Node-ID ignore", last.ID, last.Criticality)
	}

	ps := wireIEs(t, goldPathSwitchRequestSourceAMFGUAMI().encodeBody)
	if last := ps[len(ps)-1]; last.ID != IDSourceAMFGUAMI || last.Criticality != CriticalityIgnore {
		t.Fatalf("PathSwitchRequest last IE = %d/%s, want SourceAMF-GUAMI ignore", last.ID, last.Criticality)
	}
}

func TestSourceNGRANNodeToTargetNGRANNodeTransparentContainerTshark(t *testing.T) {
	tshark, err1 := exec.LookPath("tshark")
	text2pcap, err2 := exec.LookPath("text2pcap")

	if err1 != nil || err2 != nil {
		t.Skip("tshark/text2pcap not installed")
	}

	var dump strings.Builder
	hexDump(&dump, marshalT(t, goldContainerHandoverRequest()))

	rows := tsharkFields(t, tshark, text2pcap, dump.String(),
		"ngap.NRCellIdentity", "ngap.EUTRACellIdentity", "ngap.indexToRFSP", "ngap.timeUEStayedInCell",
		"ngap.timeUEStayedInCellEnhancedGranularity", "ngap.cellSize", "ngap.pDUSessionID", "ngap.qosFlowIdentifier",
		"ngap.dRB_ID", "ngap.qosFlowMappingIndication", "ngap.e_RAB_ID", "ngap.dLForwarding",
		"_ws.malformed", "_ws.expert.message")
	if len(rows) != 1 {
		t.Fatalf("tshark decoded %d frames, want 1", len(rows))
	}

	want := []string{
		"0x0000000123456789", "0x00abcdef", "256", "4095", "40950", "3", "5,5", "1,1,2,1,2", "1", "1", "5,6", "0,0",
		"", "",
	}

	for i, w := range want {
		if rows[0][i] != w {
			t.Errorf("column %d = %q, want %q (row %q)", i, rows[0][i], w, strings.Join(rows[0], "|"))
		}
	}
}

func FuzzTransparentContainerRoundTrip(f *testing.F) {
	f.Add(mustHexBytes(goldenSourceNGRANContainer))

	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(data)
		if err != nil {
			return
		}

		b, err := c.Marshal()
		if err != nil {
			t.Fatalf("encode decoded value: %v", err)
		}

		again, err := ParseSourceNGRANNodeToTargetNGRANNodeTransparentContainer(b)
		if err != nil {
			t.Fatalf("decode re-encoded value: %v", err)
		}

		if !again.Equal(*c) {
			t.Fatalf("re-decoded\n got %+v\nwant %+v", again, c)
		}

		b2, err := again.Marshal()
		if err != nil || !bytes.Equal(b2, b) {
			t.Fatalf("re-encoded %x (%v), want %x", b2, err, b)
		}
	})
}
