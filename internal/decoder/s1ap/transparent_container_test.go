// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

func testSourceeNBContainer(t *testing.T) s1ap.TransparentContainer {
	t.Helper()

	b, err := (&s1ap.SourceeNBToTargeteNBTransparentContainer{
		RRCContainer:              s1ap.RRCContainer{0x80},
		ERABInformationList:       s1ap.ERABInformationList{{ERABID: 5, DLForwarding: s1ap.Ptr(s1ap.DLForwardingProposed)}, {ERABID: 6}},
		TargetCellID:              testCGI,
		SubscriberProfileIDforRFP: s1ap.Ptr(s1ap.SubscriberProfileIDforRFP(12)),
		UEHistoryInformation: s1ap.UEHistoryInformation{
			{EUTRANCell: &s1ap.LastVisitedEUTRANCellInformation{GlobalCellID: testCGI, CellType: s1ap.CellType{CellSize: s1ap.CellSizeSmall}, TimeUEStayedInCell: 30}},
			{UTRANCell: s1ap.Ptr(s1ap.LastVisitedUTRANCellInformation{0x01, 0x02})},
			{GERANCell: &s1ap.LastVisitedGERANCellInformation{}},
			{NGRANCell: s1ap.Ptr(s1ap.LastVisitedNGRANCellInformation{0x0a})},
		},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestDecodeSourceeNBToTargeteNBTransparentContainer(t *testing.T) {
	c := testSourceeNBContainer(t)

	msg := DecodeS1APMessage(mustMarshal(t, &s1ap.HandoverRequired{
		MMEUES1APID:  7,
		ENBUES1APID:  2,
		HandoverType: s1ap.HandoverTypeIntraLTE,
		Cause:        &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 16},
		TargetID: s1ap.TargetID{TargeteNBID: s1ap.TargeteNBID{
			GlobalENBID: s1ap.GlobalENBID{PLMNIdentity: testPLMN, ENBID: s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 2}},
			SelectedTAI: testTAI,
		}},
		SourceToTarget: c,
	}))

	got, ok := mustIE(t, msg, s1ap.IDSourceToTargetTransparentContainer).Value.(SourceToTargetTransparentContainer)
	if !ok {
		t.Fatalf("container rendered as %T", mustIE(t, msg, s1ap.IDSourceToTargetTransparentContainer).Value)
	}

	utran, geran, ngran := "0102", "undefined", "0a"
	dl := utils.NamedEnum(uint8(0), "dL-Forwarding-proposed")
	want := SourceToTargetTransparentContainer{
		Hex: got.Hex,
		SourceeNBToTargeteNB: &SourceeNBToTargeteNBTransparentContainer{
			RRCContainer:              "80",
			ERABInformationList:       []ERABInformation{{ERABID: 5, DLForwarding: &dl}, {ERABID: 6}},
			TargetCellID:              eutranCGI(testCGI),
			SubscriberProfileIDforRFP: s1ap.Ptr(uint16(12)),
			UEHistoryInformation: []LastVisitedCell{
				{EUTRANCell: &LastVisitedEUTRANCell{GlobalCellID: eutranCGI(testCGI), CellSize: utils.NamedEnum(uint8(1), "small"), TimeUEStayedInCell: 30}},
				{UTRANCell: &utran},
				{GERANCell: &geran},
				{NGRANCell: &ngran},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("container\n got %+v\nwant %+v", got, want)
	}
}

func TestDecodeSourceToTargetTransparentContainerUndecodable(t *testing.T) {
	got := sourceToTargetTransparentContainer([]byte{0xab}, true)
	if got.Hex != "ab" || got.SourceeNBToTargeteNB != nil || got.Error == "" {
		t.Fatalf("container = %+v", got)
	}

	got = sourceToTargetTransparentContainer(testSourceeNBContainer(t), false)
	if got.SourceeNBToTargeteNB != nil || got.Error != "" {
		t.Fatalf("non-eNB container = %+v", got)
	}
}
