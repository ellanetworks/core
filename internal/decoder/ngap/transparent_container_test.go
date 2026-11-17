// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/utils"
	lib "github.com/ellanetworks/core/ngap"
)

func testSourceNGRANContainer(t *testing.T) lib.SourceToTargetTransparentContainer {
	t.Helper()

	b, err := (&lib.SourceNGRANNodeToTargetNGRANNodeTransparentContainer{
		RRCContainer: lib.RRCContainer{0x80},
		PDUSessionResourceInformationList: lib.PDUSessionResourceInformationList{{
			PDUSessionID:           1,
			QosFlowInformationList: lib.QosFlowInformationList{{QosFlowIdentifier: 1, DLForwarding: lib.Ptr(lib.DLForwardingProposed)}, {QosFlowIdentifier: 2}},
			DRBsToQosFlowsMappingList: lib.DRBsToQosFlowsMappingList{{
				DRBID:                 1,
				AssociatedQosFlowList: lib.AssociatedQosFlowList{{QosFlowIdentifier: 1, QosFlowMappingIndication: lib.Ptr(lib.QosFlowMappingUL)}, {QosFlowIdentifier: 2}},
			}},
		}},
		ERABInformationList: lib.ERABInformationList{{ERABID: 5, DLForwarding: lib.Ptr(lib.DLForwardingProposed)}},
		TargetCellID:        lib.NGRANCGI{NRCGI: &lib.NRCGI{PLMNIdentity: testPLMN, NRCellIdentity: 0x10}},
		IndexToRFSP:         lib.Ptr(lib.IndexToRFSP(7)),
		UEHistoryInformation: lib.UEHistoryInformation{
			{LastVisitedCellInformation: lib.LastVisitedCellInformation{NGRANCell: &lib.LastVisitedNGRANCellInformation{
				GlobalCellID:                          lib.NGRANCGI{EUTRACGI: &lib.EUTRACGI{PLMNIdentity: testPLMN, EUTRACellIdentity: 0x101}},
				CellType:                              lib.CellType{CellSize: lib.CellSizeMedium},
				TimeUEStayedInCell:                    30,
				TimeUEStayedInCellEnhancedGranularity: lib.Ptr(lib.TimeUEStayedInCellEnhancedGranularity(300)),
				HOCauseValue:                          new(lib.CauseRadioNetwork(16).Cause()),
			}}},
			{LastVisitedCellInformation: lib.LastVisitedCellInformation{EUTRANCell: lib.Ptr(lib.LastVisitedEUTRANCellInformation{0x01})}},
			{LastVisitedCellInformation: lib.LastVisitedCellInformation{UTRANCell: lib.Ptr(lib.LastVisitedUTRANCellInformation{0x02})}},
			{LastVisitedCellInformation: lib.LastVisitedCellInformation{GERANCell: lib.Ptr(lib.LastVisitedGERANCellInformation{0x03})}},
		},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestDecodeSourceNGRANNodeToTargetNGRANNodeTransparentContainer(t *testing.T) {
	got := sourceToTargetTransparentContainer(testSourceNGRANContainer(t), true)

	nr := nrCGI(lib.NRCGI{PLMNIdentity: testPLMN, NRCellIdentity: 0x10})
	eutra := eutraCGI(lib.EUTRACGI{PLMNIdentity: testPLMN, EUTRACellIdentity: 0x101})
	dl := utils.NamedEnum(uint8(0), "dl-forwarding-proposed")
	ul := utils.NamedEnum(uint8(0), "ul")
	hoCause := cause(lib.CauseRadioNetwork(16).Cause())
	e, u, g := "01", "02", "03"

	want := SourceToTargetTransparentContainer{
		Hex: got.Hex,
		SourceNGRANNodeToTargetNGRANNode: &SourceNGRANNodeToTargetNGRANNodeTransparentContainer{
			RRCContainer: "80",
			PDUSessionResourceInformationList: []PDUSessionResourceInformation{{
				PDUSessionID:           1,
				QosFlowInformationList: []QosFlowInformation{{QosFlowIdentifier: 1, DLForwarding: &dl}, {QosFlowIdentifier: 2}},
				DRBsToQosFlowsMappingList: []DRBToQosFlowsMapping{{
					DRBID:                 1,
					AssociatedQosFlowList: []MappedAssociatedQosFlow{{QosFlowIdentifier: 1, QosFlowMappingIndication: &ul}, {QosFlowIdentifier: 2}},
				}},
			}},
			ERABInformationList: []ERABInformation{{ERABID: 5, DLForwarding: &dl}},
			TargetCellID:        NGRANCGI{NRCGI: &nr},
			IndexToRFSP:         lib.Ptr(uint16(7)),
			UEHistoryInformation: []LastVisitedCell{
				{NGRANCell: &LastVisitedNGRANCell{
					GlobalCellID:                          NGRANCGI{EUTRACGI: &eutra},
					CellSize:                              utils.NamedEnum(uint8(2), "medium"),
					TimeUEStayedInCell:                    30,
					TimeUEStayedInCellEnhancedGranularity: lib.Ptr(uint16(300)),
					HOCauseValue:                          &hoCause,
				}},
				{EUTRANCell: &e},
				{UTRANCell: &u},
				{GERANCell: &g},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("container\n got %+v\nwant %+v", got, want)
	}
}

func TestDecodeSourceToTargetTransparentContainerUndecodable(t *testing.T) {
	got := sourceToTargetTransparentContainer([]byte{0x05, 0x06}, true)
	if got.Hex != "0506" || got.SourceNGRANNodeToTargetNGRANNode != nil || got.Error == "" {
		t.Fatalf("container = %+v", got)
	}

	got = sourceToTargetTransparentContainer(testSourceNGRANContainer(t), false)
	if got.SourceNGRANNodeToTargetNGRANNode != nil || got.Error != "" {
		t.Fatalf("non-NG-RAN container = %+v", got)
	}
}

func TestDecodeNGAPMessage_TargetRANNodeIDAndSourceAMFGUAMI(t *testing.T) {
	corpus := modeledIEsCorpus(t)

	ho := DecodeNGAPMessage(corpus["handover_request_target_ran_node_id"]).Value.IEs
	if last := ho[len(ho)-1]; last.ID.Value != int64(lib.IDTargetRANNodeID) || last.Criticality.Value != int64(lib.CriticalityIgnore) ||
		!reflect.DeepEqual(last.Value, buildGlobalRANNodeID(lib.NewGlobalGNBID(testPLMN, 2, 22))) {
		t.Fatalf("last IE = %+v, want Target-RAN-Node-ID", last)
	}

	ps := DecodeNGAPMessage(corpus["path_switch_request_source_amf_guami"]).Value.IEs
	if last := ps[len(ps)-1]; last.ID.Value != int64(lib.IDSourceAMFGUAMI) || last.Criticality.Value != int64(lib.CriticalityIgnore) ||
		!reflect.DeepEqual(last.Value, guami(lib.GUAMI{PLMNIdentity: testPLMN, AMFRegionID: 0xca, AMFSetID: 0x3f8, AMFPointer: 0x01})) {
		t.Fatalf("last IE = %+v, want SourceAMF-GUAMI", last)
	}
}
