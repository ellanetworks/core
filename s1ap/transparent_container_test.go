// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

const goldenSourceeNBContainer = "60018001004e40024500004e4001060002f839abcdef10ff100002f83900001011800fff800d0c0002f839000000010080001e"

const goldenNGAPLastVisitedNGRANCellInformation = "0002f839000000010080001e"

func containerPLMN() PLMNIdentity { return PLMNIdentity{0x02, 0xf8, 0x39} }

func mustHexBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return b
}

func goldSourceeNBContainer() *SourceeNBToTargeteNBTransparentContainer {
	return &SourceeNBToTargeteNBTransparentContainer{
		RRCContainer:              RRCContainer{0x80},
		ERABInformationList:       ERABInformationList{{ERABID: 5, DLForwarding: Ptr(DLForwardingProposed)}, {ERABID: 6}},
		TargetCellID:              EUTRANCGI{PLMNIdentity: containerPLMN(), CellID: 0xabcdef1},
		SubscriberProfileIDforRFP: Ptr(SubscriberProfileIDforRFP(256)),
		UEHistoryInformation: UEHistoryInformation{
			{EUTRANCell: &LastVisitedEUTRANCellInformation{
				GlobalCellID:       EUTRANCGI{PLMNIdentity: containerPLMN(), CellID: 0x0000101},
				CellType:           CellType{CellSize: CellSizeLarge},
				TimeUEStayedInCell: 4095,
			}},
			{NGRANCell: Ptr(LastVisitedNGRANCellInformation(mustHexBytes(goldenNGAPLastVisitedNGRANCellInformation)))},
		},
	}
}

func TestSourceeNBToTargeteNBTransparentContainerGolden(t *testing.T) {
	in := goldSourceeNBContainer()

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenSourceeNBContainer {
		t.Fatalf("encoded\n got %s\nwant %s", got, goldenSourceeNBContainer)
	}

	out, err := ParseSourceeNBToTargeteNBTransparentContainer(b)
	if err != nil {
		t.Fatal(err)
	}

	if !out.Equal(*in) {
		t.Fatalf("decoded\n got %+v\nwant %+v", out, in)
	}

	if out.TargetCellID != (EUTRANCGI{PLMNIdentity: containerPLMN(), CellID: 0xabcdef1}) {
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

func TestSourceeNBToTargeteNBTransparentContainerFromHandoverRequest(t *testing.T) {
	m := handoverRequestGoldenCases()[0].msg.(*HandoverRequest)

	c, err := ParseSourceeNBToTargeteNBTransparentContainer(m.SourceToTarget)
	if err != nil {
		t.Fatal(err)
	}

	if c.TargetCellID != (EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0000101}) {
		t.Fatalf("targetCell-ID = %+v", c.TargetCellID)
	}

	if !bytes.Equal(c.RRCContainer, []byte{0x80}) || len(c.UEHistoryInformation) != 1 || c.UEHistoryInformation[0].EUTRANCell == nil {
		t.Fatalf("container = %+v", c)
	}
}

func TestSourceeNBToTargeteNBTransparentContainerAlternatives(t *testing.T) {
	in := &SourceeNBToTargeteNBTransparentContainer{
		RRCContainer: RRCContainer{0x80},
		TargetCellID: EUTRANCGI{PLMNIdentity: containerPLMN(), CellID: 1},
		UEHistoryInformation: UEHistoryInformation{
			{UTRANCell: Ptr(LastVisitedUTRANCellInformation{0x01, 0x02})},
			{GERANCell: &LastVisitedGERANCellInformation{Undefined: &per.Null{}}},
		},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseSourceeNBToTargeteNBTransparentContainer(b)
	if err != nil {
		t.Fatal(err)
	}

	if !out.Equal(*in) {
		t.Fatalf("decoded\n got %+v\nwant %+v", out, in)
	}
}

func TestSourceeNBToTargeteNBTransparentContainerRejectsInvalid(t *testing.T) {
	cell := UEHistoryInformation{{UTRANCell: Ptr(LastVisitedUTRANCellInformation{0x01})}}

	for name, c := range map[string]*SourceeNBToTargeteNBTransparentContainer{
		"empty history":                      {RRCContainer: RRCContainer{0x80}},
		"history cell without alternative":   {RRCContainer: RRCContainer{0x80}, UEHistoryInformation: UEHistoryInformation{{}}},
		"history cell with two alternatives": {RRCContainer: RRCContainer{0x80}, UEHistoryInformation: UEHistoryInformation{{UTRANCell: Ptr(LastVisitedUTRANCellInformation{}), GERANCell: &LastVisitedGERANCellInformation{}}}},
		"subscriber profile out of range":    {RRCContainer: RRCContainer{0x80}, SubscriberProfileIDforRFP: Ptr(SubscriberProfileIDforRFP(0)), UEHistoryInformation: cell},
		"empty E-RAB list":                   {RRCContainer: RRCContainer{0x80}, ERABInformationList: ERABInformationList{}, UEHistoryInformation: cell},
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

func handChoiceAddition(t *testing.T, w *per.Writer, index int64) {
	t.Helper()

	w.WriteBit(true)
	mustNil(t, per.EncodeNormallySmall(w, per.Aligned, index))
	mustNil(t, per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0x01, 0x02, 0x03}))
}

func handSourceeNBContainer(t *testing.T, extCrit Criticality) []byte {
	t.Helper()

	w := per.NewWriter()
	enc := per.Aligned

	w.WriteBit(true)
	w.WriteBit(true)
	w.WriteBit(false)
	w.WriteBit(true)

	mustNil(t, RRCContainer{0x80}.MarshalPER(w, enc))
	mustNil(t, per.EncodeLength(w, enc, 1, maxnoofERABs, true, 1, func(int64) error {
		item := per.NewWriter()
		item.WriteBit(true)
		item.WriteBit(false)
		item.WriteBit(true)
		mustNil(t, ERABID(5).MarshalPER(item, enc))
		handTransparentExtensions(t, item, CriticalityReject)
		handAdditions(t, item)
		item.AlignToByte()

		return encodeContainerField(w, enc, ieField{ID: IDERABInformationListItem, Criticality: CriticalityIgnore, Value: ap.RawValue(item.Bytes())})
	}))
	mustNil(t, (&EUTRANCGI{PLMNIdentity: containerPLMN(), CellID: 0xabcdef1}).MarshalPER(w, enc))

	mustNil(t, per.EncodeLength(w, enc, 1, maxnoofCellsinUEHistoryInfo, true, 3, func(int64) error {
		handChoiceAddition(t, w, 7)

		w.WriteBit(true)
		mustNil(t, per.EncodeNormallySmall(w, enc, 0))
		mustNil(t, per.EncodeOpenTypeBytes(w, enc, []byte{0x02, 0xaa, 0xbb}))

		w.WriteBit(false)
		mustNil(t, per.EncodeConstrainedWholeNumber(w, enc, 0, 2, 2))
		handChoiceAddition(t, w, 4)

		return nil
	}))

	handTransparentExtensions(t, w, extCrit)
	handAdditions(t, w)
	w.AlignToByte()

	return w.Bytes()
}

func TestSourceeNBToTargeteNBTransparentContainerLossless(t *testing.T) {
	for _, crit := range []Criticality{CriticalityIgnore, CriticalityReject} {
		t.Run(crit.String(), func(t *testing.T) {
			b := handSourceeNBContainer(t, crit)

			c, err := DecodeSourceeNBToTargeteNBTransparentContainer(b)
			if err != nil {
				t.Fatal(err)
			}

			if d := c.Diagnostics(); len(d.IEs) == 0 {
				t.Fatal("unknown extensions in the container are not reported")
			}

			if c.TargetCellID != (EUTRANCGI{PLMNIdentity: containerPLMN(), CellID: 0xabcdef1}) {
				t.Fatalf("targetCell-ID = %+v", c.TargetCellID)
			}

			h := c.UEHistoryInformation
			if len(h) != 3 || h[0].Ext == nil || h[0].Ext.Index != 7 ||
				h[1].NGRANCell == nil || !bytes.Equal(*h[1].NGRANCell, []byte{0xaa, 0xbb}) ||
				h[2].GERANCell == nil || h[2].GERANCell.Ext == nil {
				t.Fatalf("UE history = %+v", h)
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

func TestSourceeNBToTargeteNBTransparentContainerTruncated(t *testing.T) {
	b, err := goldSourceeNBContainer().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	for n := range len(b) - 1 {
		if _, err := ParseSourceeNBToTargeteNBTransparentContainer(b[:n]); err == nil {
			t.Errorf("parse of %d/%d octets succeeded", n, len(b))
		}
	}
}

func TestSourceeNBToTargeteNBTransparentContainerExtensionValue(t *testing.T) {
	w := per.NewWriter()
	w.WriteBit(false)
	w.WriteBit(true)
	w.WriteBit(false)
	w.WriteBit(false)
	mustNil(t, RRCContainer{0x80}.MarshalPER(w, per.Aligned))
	mustNil(t, per.EncodeLength(w, per.Aligned, 1, maxnoofERABs, true, 1, func(int64) error {
		item := per.NewWriter()
		item.WriteBit(false)
		item.WriteBit(true)
		item.WriteBit(false)
		mustNil(t, ERABID(5).MarshalPER(item, per.Aligned))
		mustNil(t, per.EncodeEnumerated(item, per.Aligned, int64(len(enumDLForwarding.Root)), true, 1))
		item.AlignToByte()

		return encodeContainerField(w, per.Aligned, ieField{ID: IDERABInformationListItem, Criticality: CriticalityIgnore, Value: ap.RawValue(item.Bytes())})
	}))
	w.AlignToByte()

	mustNil(t, (&EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}).MarshalPER(w, per.Aligned))
	mustNil(t, per.EncodeLength(w, per.Aligned, 1, maxnoofCellsinUEHistoryInfo, true, 1, func(int64) error {
		w.WriteBit(false)
		mustNil(t, per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 2, 2))
		handChoiceAddition(t, w, 4)

		return nil
	}))
	w.AlignToByte()

	c, err := DecodeSourceeNBToTargeteNBTransparentContainer(w.Bytes())
	if err != nil {
		t.Fatalf("decode is lenient, got %v", err)
	}

	if got := *c.ERABInformationList[0].DLForwarding; got != DLForwarding(len(enumDLForwarding.Root)) {
		t.Fatalf("DL-Forwarding = %d, want the extension index kept", got)
	}

	if !c.Diagnostics().RootNotComprehended {
		t.Fatalf("diagnostics = %+v, want the extension value reported", c.Diagnostics())
	}

	var ase *ap.AbstractSyntaxError[ProtocolIEID]
	if _, err := ParseSourceeNBToTargeteNBTransparentContainer(w.Bytes()); !errors.As(err, &ase) {
		t.Fatalf("err = %v, want an abstract syntax error", err)
	}
}

func goldContainerHandoverRequest() *HandoverRequest {
	c, err := goldSourceeNBContainer().Marshal()
	if err != nil {
		panic(err)
	}

	return &HandoverRequest{
		MMEUES1APID:            1,
		HandoverType:           HandoverTypeIntraLTE,
		Cause:                  Ptr(CauseRadioNetworkHandoverDesirableForRadio.Cause()),
		UEAMBR:                 UEAggregateMaximumBitRate{DL: 1_000_000, UL: 500_000},
		ERABToBeSetup:          []ERABToBeSetupItemHOReq{{ERABID: 5, TransportLayerAddress: goldTLA(), GTPTEID: 1, QoS: goldQoS()}},
		SourceToTarget:         c,
		UESecurityCapabilities: UESecurityCapabilities{EncryptionAlgorithms: SecurityAlgorithms{Value: 0x8000}, IntegrityProtectionAlgorithms: SecurityAlgorithms{Value: 0xc000}},
		SecurityContext:        SecurityContext{NextHopChainingCount: 3},
	}
}

func goldPathSwitchRequestSourceMMEGUMMEI() *PathSwitchRequest {
	m := sampledPathSwitchRequest()
	m.SourceMMEGUMMEI = Ptr(GUMMEI{PLMNIdentity: containerPLMN(), MMEGroupID: MMEGroupID{0x80, 0x01}, MMECode: 0x02})

	return m
}

func transparentContainerGoldenCases() []newMessageCase {
	return []newMessageCase{
		{
			name:   "HandoverRequestSourceeNBContainer",
			msg:    goldContainerHandoverRequest(),
			parse:  func(b []byte) (any, error) { return ParseHandoverRequest(b) },
			choice: isInitiating,
			golden: "000100809c000008000000020001000100010000024002020000420008100f42404007a1200035001200001b000d0a1f0a00000100000001000904006800343360018001004e40024500004e4001060002f839abcdef10ff100002f83900001011800fff800d0c0002f839000000010080001e006b000510000c000000280021180000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name:   "PathSwitchRequestSourceMMEGUMMEI",
			msg:    goldPathSwitchRequestSourceMMEGUMMEI(),
			parse:  func(b []byte) (any, error) { return ParsePathSwitchRequest(b) },
			choice: isInitiating,
			golden: "0003004f0000070008000200020016000f000017000a0a1f0a0102030102030400580005c0020000bf006440080000f110123c6010004340060000f1100001006b400518000c0000009d40070002f839800102",
		},
	}
}

func TestPathSwitchRequestSourceMMEGUMMEIWireCriticality(t *testing.T) {
	ies := wireIEs(t, goldPathSwitchRequestSourceMMEGUMMEI().encodeBody)
	if last := ies[len(ies)-1]; last.ID != IDSourceMMEGUMMEI || last.Criticality != CriticalityIgnore {
		t.Fatalf("last IE = %d/%s, want SourceMME-GUMMEI ignore", last.ID, last.Criticality)
	}
}

func TestSourceeNBToTargeteNBTransparentContainerTshark(t *testing.T) {
	tshark, err1 := exec.LookPath("tshark")
	text2pcap, err2 := exec.LookPath("text2pcap")

	if err1 != nil || err2 != nil {
		t.Skip("tshark/text2pcap not installed")
	}

	b, err := goldContainerHandoverRequest().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	var dump strings.Builder
	hexDump(&dump, b)

	rows := tsharkFields(t, tshark, text2pcap, dump.String(),
		"s1ap.CellIdentity", "s1ap.subscriberProfileIDforRFP", "s1ap.e_RAB_ID", "s1ap.dL_Forwarding",
		"s1ap.cell_Size", "s1ap.time_UE_StayedInCell", "ngap.NRCellIdentity", "ngap.timeUEStayedInCell",
		"_ws.malformed", "_ws.expert.message")
	if len(rows) != 1 {
		t.Fatalf("tshark decoded %d frames, want 1", len(rows))
	}

	want := []string{"0x0abcdef1,0x00000101", "256", "5,5,6", "0", "3", "4095", "0x0000000000000010", "30", "", ""}

	for i, w := range want {
		if rows[0][i] != w {
			t.Errorf("column %d = %q, want %q (row %q)", i, rows[0][i], w, strings.Join(rows[0], "|"))
		}
	}
}

func FuzzTransparentContainerRoundTrip(f *testing.F) {
	f.Add(mustHexBytes(goldenSourceeNBContainer))

	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := ParseSourceeNBToTargeteNBTransparentContainer(data)
		if err != nil {
			return
		}

		b, err := c.Marshal()
		if err != nil {
			t.Fatalf("encode decoded value: %v", err)
		}

		again, err := ParseSourceeNBToTargeteNBTransparentContainer(b)
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
