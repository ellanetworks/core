// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/nrppa"
)

// Regenerate with: go test ./internal/decoder/ngap/ -run TestDecoderGolden -update
var updateGolden = flag.Bool("update", false, "regenerate decoder golden JSON fixtures")

func mustB64(t *testing.T, s string) []byte {
	t.Helper()

	b, err := decodeB64(s)
	if err != nil {
		t.Fatalf("decode capture: %v", err)
	}

	return b
}

func mustMarshal(t *testing.T, m interface{ Marshal() ([]byte, error) }) []byte {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return b
}

var (
	testPLMN = lib.PLMNIdentity{0x00, 0xf1, 0x10}
	testTAI  = lib.TAI{PLMNIdentity: testPLMN, TAC: 1}
)

func testDiagnostics() *lib.CriticalityDiagnostics {
	return &lib.CriticalityDiagnostics{
		ProcedureCode:        lib.Ptr(lib.ProcNGSetup),
		TriggeringMessage:    lib.Ptr(lib.TriggeringInitiatingMessage),
		ProcedureCriticality: lib.Ptr(lib.CriticalityReject),
		IEsCriticalityDiagnostics: []lib.CriticalityDiagnosticsIEItem{{
			IEID:          lib.IDGlobalRANNodeID,
			IECriticality: lib.CriticalityReject,
			TypeOfError:   lib.TypeOfErrorMissing,
		}},
	}
}

// goldenCorpus is the PDU behind every fixture: wire captures where the message
// occurs in normal operation, and codec-built messages elsewhere. The "_full"
// entries carry every IE the message's table names, so a renderer that drops
// one shows up as a fixture diff.
func goldenCorpus(t *testing.T) map[string][]byte {
	t.Helper()

	corpus := map[string][]byte{
		"ng_setup_request":                      mustB64(t, ngSetupRequestCapture),
		"ng_setup_response":                     mustB64(t, ngSetupResponseCapture),
		"ng_setup_failure":                      mustB64(t, ngSetupFailureCapture),
		"initial_ue_message":                    mustB64(t, initialUEMessageCapture),
		"downlink_nas_transport":                mustB64(t, downlinkNASTransportCapture),
		"uplink_nas_transport":                  mustB64(t, uplinkNASTransportCapture),
		"initial_context_setup_request":         mustB64(t, initialContextSetupRequestCapture),
		"initial_context_setup_response":        mustB64(t, initialContextSetupResponseCapture),
		"initial_context_setup_failure":         mustB64(t, initialContextSetupFailureCapture),
		"pdu_session_resource_setup_request":    mustB64(t, pduSessionResourceSetupRequestCapture),
		"pdu_session_resource_setup_response":   mustB64(t, pduSessionResourceSetupResponseCapture),
		"pdu_session_resource_release_command":  mustB64(t, pduSessionResourceReleaseCommandCapture),
		"pdu_session_resource_release_response": mustB64(t, pduSessionResourceReleaseResponseCapture),
		"ue_context_release_request":            mustB64(t, ueContextReleaseRequestCapture),
		"ue_context_release_command":            mustB64(t, ueContextReleaseCommandCapture),
		"ue_context_release_complete":           mustB64(t, ueContextReleaseCompleteCapture),
		"ue_radio_capability_info_indication":   mustB64(t, ueRadioCapabilityInfoIndicationCapture),
		"amf_status_indication":                 mustB64(t, amfStatusIndicationCapture),
		"paging":                                mustB64(t, pagingCapture),
		"invalid":                               {0xff, 0x00, 0x01},
	}

	psReqTransfer, err := (&lib.PathSwitchRequestTransfer{
		DLNGUUPTNLInformation: lib.UPTransportLayerInformation{
			GTPTunnel: lib.GTPTunnel{TransportLayerAddress: lib.TransportLayerAddress{10, 45, 0, 1}, GTPTEID: 0x21},
		},
		QosFlowAccepted: lib.QosFlowAcceptedList{{QosFlowIdentifier: 1}},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Path Switch Request Transfer: %v", err)
	}

	psAckTransfer, err := (&lib.PathSwitchRequestAcknowledgeTransfer{
		ULNGUUPTNLInformation: &lib.UPTransportLayerInformation{
			GTPTunnel: lib.GTPTunnel{TransportLayerAddress: lib.TransportLayerAddress{10, 46, 0, 1}, GTPTEID: 0x31},
		},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Path Switch Request Acknowledge Transfer: %v", err)
	}

	psFailTransfer, err := (&lib.PathSwitchRequestUnsuccessfulTransfer{
		Cause: lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Path Switch Request Unsuccessful Transfer: %v", err)
	}

	corpus["path_switch_request_full"] = mustMarshal(t, &lib.PathSwitchRequest{
		RANUENGAPID:       3,
		SourceAMFUENGAPID: 7,
		UserLocationInformation: &lib.UserLocationInformation{
			Kind:         lib.UserLocationNR,
			PLMNIdentity: testPLMN,
			CellIdentity: 0x000000010,
			TAI:          testTAI,
		},
		UESecurityCapabilities: &lib.UESecurityCapabilities{
			NREncryptionAlgorithms: 0xe000, NRIntegrityProtectionAlgorithms: 0xe000,
			EUTRAEncryptionAlgorithms: 0xe000, EUTRAIntegrityProtectionAlgorithms: 0xe000,
		},
		PDUSessionResourceToBeSwitchedDLList: lib.PDUSessionResourceToBeSwitchedDLList{
			{PDUSessionID: 1, Transfer: psReqTransfer},
		},
	})

	corpus["path_switch_request_acknowledge_full"] = mustMarshal(t, &lib.PathSwitchRequestAcknowledge{
		AMFUENGAPID:     lib.Ptr(lib.AMFUENGAPID(7)),
		RANUENGAPID:     lib.Ptr(lib.RANUENGAPID(3)),
		SecurityContext: lib.SecurityContext{NextHopChainingCount: 3, NextHopNH: lib.SecurityKey{0x01, 0x02}},
		PDUSessionResourceSwitchedList: lib.PDUSessionResourceSwitchedList{
			{PDUSessionID: 1, Transfer: psAckTransfer},
		},
		PDUSessionResourceReleased: lib.PDUSessionResourceReleasedListPSAck{
			{PDUSessionID: 2, Transfer: psFailTransfer},
		},
		AllowedNSSAI: lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
	})

	corpus["path_switch_request_failure_full"] = mustMarshal(t, &lib.PathSwitchRequestFailure{
		AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(7)),
		RANUENGAPID: lib.Ptr(lib.RANUENGAPID(3)),
		PDUSessionResourceReleased: lib.PDUSessionResourceReleasedListPSFail{
			{PDUSessionID: 1, Transfer: psFailTransfer},
		},
		CriticalityDiagnostics: &lib.CriticalityDiagnostics{ProcedureCode: lib.Ptr(lib.ProcPathSwitchRequest)},
	})

	modReqTransfer, err := (&lib.PDUSessionResourceModifyRequestTransfer{
		PDUSessionAggregateMaximumBitRate: &lib.PDUSessionAggregateMaximumBitRate{DL: 200000000, UL: 100000000},
		QosFlowAddOrModifyRequest: lib.QosFlowAddOrModifyRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: &lib.QosFlowLevelQosParameters{
				QosCharacteristics: lib.QosCharacteristics{
					Kind:          lib.QosCharacteristicsNonDynamic5QI,
					NonDynamic5QI: lib.NonDynamic5QIDescriptor{FiveQI: 9},
				},
				AllocationAndRetentionPriority: lib.AllocationAndRetentionPriority{PriorityLevelARP: 8},
			},
		}},
		QosFlowToRelease: lib.QosFlowListWithCause{
			{QosFlowIdentifier: 2, Cause: lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0}},
		},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Modify Request Transfer: %v", err)
	}

	modRespTransfer, err := (&lib.PDUSessionResourceModifyResponseTransfer{
		DLNGUUPTNLInformation: &lib.UPTransportLayerInformation{
			GTPTunnel: lib.GTPTunnel{TransportLayerAddress: lib.TransportLayerAddress{10, 45, 0, 1}, GTPTEID: 0x41},
		},
		QosFlowAddOrModifyResponse: lib.QosFlowAddOrModifyResponseList{{QosFlowIdentifier: 1}},
		QosFlowFailedToAddOrModify: lib.QosFlowListWithCause{
			{QosFlowIdentifier: 2, Cause: lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0}},
		},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Modify Response Transfer: %v", err)
	}

	modIndTransfer, err := (&lib.PDUSessionResourceModifyIndicationTransfer{
		DLQosFlowPerTNLInformation: lib.QosFlowPerTNLInformation{
			UPTransportLayerInformation: lib.UPTransportLayerInformation{
				GTPTunnel: lib.GTPTunnel{TransportLayerAddress: lib.TransportLayerAddress{10, 46, 0, 1}, GTPTEID: 0x51},
			},
			AssociatedQosFlowList: lib.AssociatedQosFlowList{{QosFlowIdentifier: 1}},
		},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Modify Indication Transfer: %v", err)
	}

	modCfmTransfer, err := (&lib.PDUSessionResourceModifyConfirmTransfer{
		QosFlowModifyConfirm: lib.QosFlowModifyConfirmList{{QosFlowIdentifier: 1}},
		ULNGUUPTNLInformation: lib.UPTransportLayerInformation{
			GTPTunnel: lib.GTPTunnel{TransportLayerAddress: lib.TransportLayerAddress{10, 47, 0, 1}, GTPTEID: 0x61},
		},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Modify Confirm Transfer: %v", err)
	}

	modFailTransfer, err := (&lib.PDUSessionResourceModifyIndicationUnsuccessfulTransfer{
		Cause: lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Modify Indication Unsuccessful Transfer: %v", err)
	}

	corpus["pdu_session_resource_modify_request_full"] = mustMarshal(t, &lib.PDUSessionResourceModifyRequest{
		AMFUENGAPID: 7,
		RANUENGAPID: 3,
		PDUSessionResourceModify: lib.PDUSessionResourceModifyListModReq{
			{PDUSessionID: 1, NASPDU: &lib.NASPDU{0x7e, 0x00, 0x68}, Transfer: modReqTransfer},
		},
	})

	corpus["pdu_session_resource_modify_response_full"] = mustMarshal(t, &lib.PDUSessionResourceModifyResponse{
		AMFUENGAPID:              lib.Ptr(lib.AMFUENGAPID(7)),
		RANUENGAPID:              lib.Ptr(lib.RANUENGAPID(3)),
		PDUSessionResourceModify: lib.PDUSessionResourceModifyListModRes{{PDUSessionID: 1, Transfer: modRespTransfer}},
		PDUSessionResourceFailed: lib.PDUSessionResourceFailedToModifyListModRes{{PDUSessionID: 2, Transfer: modFailTransfer}},
		CriticalityDiagnostics:   &lib.CriticalityDiagnostics{ProcedureCode: lib.Ptr(lib.ProcPDUSessionResourceModify)},
	})

	corpus["pdu_session_resource_modify_indication_full"] = mustMarshal(t, &lib.PDUSessionResourceModifyIndication{
		AMFUENGAPID:              7,
		RANUENGAPID:              3,
		PDUSessionResourceModify: lib.PDUSessionResourceModifyListModInd{{PDUSessionID: 1, Transfer: modIndTransfer}},
	})

	corpus["pdu_session_resource_modify_confirm_full"] = mustMarshal(t, &lib.PDUSessionResourceModifyConfirm{
		AMFUENGAPID:              lib.Ptr(lib.AMFUENGAPID(7)),
		RANUENGAPID:              lib.Ptr(lib.RANUENGAPID(3)),
		PDUSessionResourceModify: lib.PDUSessionResourceModifyListModCfm{{PDUSessionID: 1, Transfer: modCfmTransfer}},
		PDUSessionResourceFailed: lib.PDUSessionResourceFailedToModifyListModCfm{{PDUSessionID: 2, Transfer: modFailTransfer}},
	})

	corpus["pdu_session_resource_notify_full"] = mustMarshal(t, &lib.PDUSessionResourceNotify{
		AMFUENGAPID:              7,
		RANUENGAPID:              3,
		PDUSessionResourceNotify: lib.PDUSessionResourceNotifyList{{PDUSessionID: 1, Transfer: lib.TransferContainer{0x01, 0x02}}},
	})

	hoRequiredTransfer, err := (&lib.HandoverRequiredTransfer{
		DirectForwardingPathAvailability: lib.Ptr(lib.DirectForwardingPathAvailable),
	}).Marshal()
	if err != nil {
		t.Fatalf("build Handover Required Transfer: %v", err)
	}

	hoCommandTransfer, err := (&lib.HandoverCommandTransfer{
		DLForwardingUPTNLInformation: &lib.UPTransportLayerInformation{
			GTPTunnel: lib.GTPTunnel{TransportLayerAddress: lib.TransportLayerAddress{10, 45, 0, 9}, GTPTEID: 0x71},
		},
		QosFlowToBeForwarded: lib.QosFlowToBeForwardedList{{QosFlowIdentifier: 1}},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Handover Command Transfer: %v", err)
	}

	hoAckTransfer, err := (&lib.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: lib.UPTransportLayerInformation{
			GTPTunnel: lib.GTPTunnel{TransportLayerAddress: lib.TransportLayerAddress{10, 46, 0, 9}, GTPTEID: 0x81},
		},
		SecurityResult:       &lib.SecurityResult{IntegrityProtectionResult: lib.IntegrityProtectionPerformed},
		QosFlowSetupResponse: lib.QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Handover Request Acknowledge Transfer: %v", err)
	}

	hoAllocFailTransfer, err := (&lib.HandoverResourceAllocationUnsuccessfulTransfer{
		Cause: lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Handover Resource Allocation Unsuccessful Transfer: %v", err)
	}

	hoPrepFailTransfer, err := (&lib.HandoverPreparationUnsuccessfulTransfer{
		Cause: lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0},
	}).Marshal()
	if err != nil {
		t.Fatalf("build Handover Preparation Unsuccessful Transfer: %v", err)
	}

	hoSetupTransfer, err := (&lib.PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation: lib.UPTransportLayerInformation{
			GTPTunnel: lib.GTPTunnel{TransportLayerAddress: lib.TransportLayerAddress{10, 47, 0, 9}, GTPTEID: 0x91},
		},
		PDUSessionType: lib.PDUSessionTypeIPv4,
		QosFlowSetupRequest: lib.QosFlowSetupRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: lib.QosFlowLevelQosParameters{
				QosCharacteristics: lib.QosCharacteristics{
					Kind:          lib.QosCharacteristicsNonDynamic5QI,
					NonDynamic5QI: lib.NonDynamic5QIDescriptor{FiveQI: 9},
				},
				AllocationAndRetentionPriority: lib.AllocationAndRetentionPriority{PriorityLevelARP: 8},
			},
		}},
	}).Marshal()
	if err != nil {
		t.Fatalf("build HO setup transfer: %v", err)
	}

	corpus["handover_required_full"] = mustMarshal(t, &lib.HandoverRequired{
		AMFUENGAPID:  7,
		RANUENGAPID:  3,
		HandoverType: lib.HandoverTypeIntra5GS,
		Cause:        &lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 16},
		TargetID: lib.TargetID{TargetRANNodeID: &lib.TargetRANNodeID{
			GlobalRANNodeID: lib.GlobalRANNodeID{Kind: lib.RANNodeIDGNB, PLMNIdentity: testPLMN, Value: 2, Bits: 22},
			SelectedTAI:     testTAI,
		}},
		DirectForwardingPathAvailability: lib.Ptr(lib.DirectForwardingPathAvailable),
		PDUSessionResourceListHORqd: lib.PDUSessionResourceListHORqd{
			{PDUSessionID: 1, Transfer: hoRequiredTransfer},
		},
		SourceToTargetTransparentContainer: lib.SourceToTargetTransparentContainer{0x01, 0x02},
	})

	corpus["handover_command_full"] = mustMarshal(t, &lib.HandoverCommand{
		AMFUENGAPID:                    7,
		RANUENGAPID:                    3,
		HandoverType:                   lib.HandoverTypeIntra5GS,
		PDUSessionResourceHandoverList: lib.PDUSessionResourceHandoverList{{PDUSessionID: 1, Transfer: hoCommandTransfer}},
		PDUSessionResourceToReleaseList: lib.PDUSessionResourceToReleaseListHOCmd{
			{PDUSessionID: 2, Transfer: hoPrepFailTransfer},
		},
		TargetToSourceTransparentContainer: lib.TargetToSourceTransparentContainer{0x03, 0x04},
	})

	corpus["handover_preparation_failure_full"] = mustMarshal(t, &lib.HandoverPreparationFailure{
		AMFUENGAPID:            lib.Ptr(lib.AMFUENGAPID(7)),
		RANUENGAPID:            lib.Ptr(lib.RANUENGAPID(3)),
		Cause:                  &lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0},
		CriticalityDiagnostics: &lib.CriticalityDiagnostics{ProcedureCode: lib.Ptr(lib.ProcHandoverPreparation)},
	})

	corpus["handover_request_full"] = mustMarshal(t, &lib.HandoverRequest{
		AMFUENGAPID:               7,
		HandoverType:              lib.HandoverTypeIntra5GS,
		Cause:                     &lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 16},
		UEAggregateMaximumBitRate: lib.UEAggregateMaximumBitRate{DL: 200000000, UL: 100000000},
		UESecurityCapabilities: lib.UESecurityCapabilities{
			NREncryptionAlgorithms: 0xe000, NRIntegrityProtectionAlgorithms: 0xe000,
			EUTRAEncryptionAlgorithms: 0xe000, EUTRAIntegrityProtectionAlgorithms: 0xe000,
		},
		SecurityContext:                    lib.SecurityContext{NextHopChainingCount: 1, NextHopNH: lib.SecurityKey{0x0a}},
		NewSecurityContextInd:              lib.Ptr(lib.NewSecurityContextIndTrue),
		PDUSessionResourceSetupListHOReq:   lib.PDUSessionResourceSetupListHOReq{{PDUSessionID: 1, SNSSAI: lib.SNSSAI{SST: 1}, Transfer: hoSetupTransfer}},
		AllowedNSSAI:                       lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
		SourceToTargetTransparentContainer: lib.SourceToTargetTransparentContainer{0x05, 0x06},
		MobilityRestrictionList: &lib.MobilityRestrictionList{
			ServingPLMN:     testPLMN,
			EquivalentPLMNs: lib.EquivalentPLMNs{testPLMN},
			RATRestrictions: lib.RATRestrictions{{PLMNIdentity: testPLMN, RATRestrictionInformation: lib.RATRestrictionEUTRA}},
		},
		GUAMI: lib.GUAMI{PLMNIdentity: testPLMN, AMFRegionID: 1, AMFSetID: 1, AMFPointer: 0},
	})

	corpus["handover_request_acknowledge_full"] = mustMarshal(t, &lib.HandoverRequestAcknowledge{
		AMFUENGAPID:                    lib.Ptr(lib.AMFUENGAPID(7)),
		RANUENGAPID:                    lib.Ptr(lib.RANUENGAPID(3)),
		PDUSessionResourceAdmittedList: lib.PDUSessionResourceAdmittedList{{PDUSessionID: 1, Transfer: hoAckTransfer}},
		PDUSessionResourceFailedToSetup: lib.PDUSessionResourceFailedToSetupListHOAck{
			{PDUSessionID: 2, Transfer: hoAllocFailTransfer},
		},
		TargetToSourceTransparentContainer: lib.TargetToSourceTransparentContainer{0x07, 0x08},
	})

	corpus["handover_failure_full"] = mustMarshal(t, &lib.HandoverFailure{
		AMFUENGAPID:            lib.Ptr(lib.AMFUENGAPID(7)),
		Cause:                  &lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0},
		CriticalityDiagnostics: &lib.CriticalityDiagnostics{ProcedureCode: lib.Ptr(lib.ProcHandoverResourceAllocation)},
	})

	corpus["handover_notify_full"] = mustMarshal(t, &lib.HandoverNotify{
		AMFUENGAPID: 7,
		RANUENGAPID: 3,
		UserLocationInformation: &lib.UserLocationInformation{
			Kind: lib.UserLocationNR, PLMNIdentity: testPLMN, CellIdentity: 0x10, TAI: testTAI,
		},
		NotifySourceNGRANNode: lib.Ptr(lib.NotifySourceNGRANNodeNotifySource),
	})

	corpus["handover_cancel_full"] = mustMarshal(t, &lib.HandoverCancel{
		AMFUENGAPID: 7, RANUENGAPID: 3,
		Cause: &lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0},
	})

	corpus["handover_cancel_acknowledge_full"] = mustMarshal(t, &lib.HandoverCancelAcknowledge{
		AMFUENGAPID:            lib.Ptr(lib.AMFUENGAPID(7)),
		RANUENGAPID:            lib.Ptr(lib.RANUENGAPID(3)),
		CriticalityDiagnostics: &lib.CriticalityDiagnostics{ProcedureCode: lib.Ptr(lib.ProcHandoverCancel)},
	})

	corpus["ran_configuration_update_full"] = mustMarshal(t, &lib.RANConfigurationUpdate{
		RANNodeName:      lib.Ptr("gnb01"),
		SupportedTAList:  lib.SupportedTAList{{TAC: 1, BroadcastPLMNList: lib.BroadcastPLMNList{{PLMNIdentity: testPLMN, TAISliceSupportList: lib.SliceSupportList{{SNSSAI: lib.SNSSAI{SST: 1}}}}}}},
		DefaultPagingDRX: lib.Ptr(lib.PagingDRXv128),
		GlobalRANNodeID:  &lib.GlobalRANNodeID{Kind: lib.RANNodeIDGNB, PLMNIdentity: testPLMN, Value: 1, Bits: 22},
		NGRANTNLAssociationToRemoveList: lib.NGRANTNLAssociationToRemoveList{{
			TNLAssociationTransportLayerAddress:    lib.CPTransportLayerInformation{EndpointIPAddress: lib.TransportLayerAddress{10, 0, 0, 1}},
			TNLAssociationTransportLayerAddressAMF: &lib.CPTransportLayerInformation{EndpointIPAddress: lib.TransportLayerAddress{10, 0, 0, 2}},
		}},
	})

	corpus["ran_configuration_update_acknowledge_full"] = mustMarshal(t, &lib.RANConfigurationUpdateAcknowledge{
		CriticalityDiagnostics: &lib.CriticalityDiagnostics{ProcedureCode: lib.Ptr(lib.ProcRANConfigurationUpdate)},
	})

	corpus["ran_configuration_update_failure_full"] = mustMarshal(t, &lib.RANConfigurationUpdateFailure{
		Cause:                  &lib.Cause{Group: lib.CauseGroupMisc, Value: 3},
		TimeToWait:             lib.Ptr(lib.TimeToWaitV10s),
		CriticalityDiagnostics: &lib.CriticalityDiagnostics{ProcedureCode: lib.Ptr(lib.ProcRANConfigurationUpdate)},
	})

	corpus["uplink_ran_configuration_transfer_full"] = mustMarshal(t, &lib.UplinkRANConfigurationTransfer{
		SONConfigurationTransfer: lib.SONConfigurationTransfer{0x01, 0x02, 0x03},
	})

	corpus["downlink_ran_configuration_transfer_full"] = mustMarshal(t, &lib.DownlinkRANConfigurationTransfer{
		SONConfigurationTransfer: lib.SONConfigurationTransfer{0x04, 0x05, 0x06},
	})

	corpus["uplink_ran_status_transfer_full"] = mustMarshal(t, &lib.UplinkRANStatusTransfer{
		AMFUENGAPID: 7, RANUENGAPID: 3, Container: lib.StatusTransferContainer{0x0a, 0x0b},
	})

	corpus["downlink_ran_status_transfer_full"] = mustMarshal(t, &lib.DownlinkRANStatusTransfer{
		AMFUENGAPID: 7, RANUENGAPID: 3, Container: lib.StatusTransferContainer{0x0c, 0x0d},
	})

	corpus["ng_reset_all_full"] = mustMarshal(t, &lib.NGReset{
		Cause:     &lib.Cause{Group: lib.CauseGroupMisc, Value: 3},
		ResetType: lib.ResetType{All: true},
	})

	corpus["ng_reset_part_full"] = mustMarshal(t, &lib.NGReset{
		Cause: &lib.Cause{Group: lib.CauseGroupTransport, Value: 0},
		ResetType: lib.ResetType{Part: lib.UEAssociatedLogicalNGConnectionList{
			{AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(7)), RANUENGAPID: lib.Ptr(lib.RANUENGAPID(3))},
			{RANUENGAPID: lib.Ptr(lib.RANUENGAPID(4))},
		}},
	})

	corpus["ng_reset_acknowledge_full"] = mustMarshal(t, &lib.NGResetAcknowledge{
		ConnectionList: lib.UEAssociatedLogicalNGConnectionList{
			{AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(7)), RANUENGAPID: lib.Ptr(lib.RANUENGAPID(3))},
		},
		CriticalityDiagnostics: &lib.CriticalityDiagnostics{ProcedureCode: lib.Ptr(lib.ProcNGReset)},
	})

	corpus["nas_non_delivery_indication_full"] = mustMarshal(t, &lib.NASNonDeliveryIndication{
		AMFUENGAPID: 7,
		RANUENGAPID: 3,
		NASPDU:      lib.NASPDU{0x7e, 0x00, 0x44},
		Cause:       &lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 27},
	})

	corpus["paging_full"] = mustMarshal(t, &lib.Paging{
		FiveGSTMSI:       &lib.FiveGSTMSI{AMFSetID: 1, AMFPointer: 2, FiveGTMSI: 0x01020304},
		PagingDRX:        lib.Ptr(lib.PagingDRXv128),
		TAIListForPaging: []lib.TAI{testTAI},
		PagingPriority:   lib.Ptr(lib.PagingPriority(0)),
		UERadioCapabilityForPaging: &lib.UERadioCapabilityForPaging{
			NR: lib.Ptr(lib.UERadioCapabilityForPagingOfNR{0xab, 0xcd}),
		},
		PagingOrigin: lib.Ptr(lib.PagingOriginNon3GPP),
	})

	corpus["initial_context_setup_request_full"] = mustMarshal(t, &lib.InitialContextSetupRequest{
		AMFUENGAPID:  1,
		RANUENGAPID:  2,
		GUAMI:        lib.GUAMI{PLMNIdentity: testPLMN, AMFRegionID: 2, AMFSetID: 1, AMFPointer: 0},
		AllowedNSSAI: lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
		UESecurityCapabilities: lib.UESecurityCapabilities{
			NREncryptionAlgorithms: 0xc000, NRIntegrityProtectionAlgorithms: 0xc000,
			EUTRAEncryptionAlgorithms: 0xc000, EUTRAIntegrityProtectionAlgorithms: 0xc000,
		},
		SecurityKey:       lib.SecurityKey{},
		UERadioCapability: lib.UERadioCapability{0x01},
		UERadioCapabilityForPaging: &lib.UERadioCapabilityForPaging{
			NR: lib.Ptr(lib.UERadioCapabilityForPagingOfNR{0xab}),
		},
	})

	corpus["error_indication_full"] = mustMarshal(t, &lib.ErrorIndication{
		AMFUENGAPID:            lib.Ptr(lib.AMFUENGAPID(1)),
		RANUENGAPID:            lib.Ptr(lib.RANUENGAPID(2)),
		Cause:                  &lib.Cause{Group: lib.CauseGroupProtocol, Value: 0},
		CriticalityDiagnostics: testDiagnostics(),
		FiveGSTMSI:             &lib.FiveGSTMSI{AMFSetID: 1, AMFPointer: 2, FiveGTMSI: 0x01020304},
	})

	corpus["ue_radio_capability_info_indication_full"] = mustMarshal(t, &lib.UERadioCapabilityInfoIndication{
		AMFUENGAPID:       1,
		RANUENGAPID:       2,
		UERadioCapability: lib.UERadioCapability{0x01, 0x02},
		UERadioCapabilityForPaging: &lib.UERadioCapabilityForPaging{
			NR:    lib.Ptr(lib.UERadioCapabilityForPagingOfNR{0xab}),
			EUTRA: lib.Ptr(lib.UERadioCapabilityForPagingOfEUTRA{0xcd}),
		},
	})

	corpus["location_report_full"] = mustMarshal(t, &lib.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 2,
		UserLocationInformation: &lib.UserLocationInformation{
			Kind:         lib.UserLocationNR,
			PLMNIdentity: testPLMN,
			CellIdentity: 0x000000010,
			TAI:          testTAI,
		},
		UEPresenceInAreaOfInterestList: lib.UEPresenceInAreaOfInterestList{{
			LocationReportingReferenceID: 1,
			UEPresence:                   lib.UEPresenceIn,
		}},
		LocationReportingRequestType: &lib.LocationReportingRequestType{
			EventType:  lib.EventTypeDirect,
			ReportArea: lib.ReportAreaCell,
		},
	})

	corpus["location_reporting_control_full"] = mustMarshal(t, &lib.LocationReportingControl{
		AMFUENGAPID: 1,
		RANUENGAPID: 2,
		LocationReportingRequestType: &lib.LocationReportingRequestType{
			EventType:  lib.EventTypeDirect,
			ReportArea: lib.ReportAreaCell,
		},
	})

	nrppaPDU, err := nrppa.BuildECIDMeasurementInitiationRequest(11, []nrppa.MeasurementQuantityValue{nrppa.MeasSSRSRP})
	if err != nil {
		t.Fatalf("build NRPPa PDU: %v", err)
	}

	corpus["downlink_ue_associated_nrppa_transport"] = mustMarshal(t, &lib.DownlinkUEAssociatedNRPPaTransport{
		AMFUENGAPID: 1, RANUENGAPID: 2, RoutingID: lib.RoutingID{0x03}, NRPPaPDU: nrppaPDU,
	})
	corpus["uplink_ue_associated_nrppa_transport"] = mustMarshal(t, &lib.UplinkUEAssociatedNRPPaTransport{
		AMFUENGAPID: 1, RANUENGAPID: 2, RoutingID: lib.RoutingID{0x03}, NRPPaPDU: nrppaPDU,
	})
	corpus["downlink_non_ue_associated_nrppa_transport"] = mustMarshal(t, &lib.DownlinkNonUEAssociatedNRPPaTransport{
		RoutingID: lib.RoutingID{0x03}, NRPPaPDU: nrppaPDU,
	})
	corpus["uplink_non_ue_associated_nrppa_transport"] = mustMarshal(t, &lib.UplinkNonUEAssociatedNRPPaTransport{
		RoutingID: lib.RoutingID{0x03}, NRPPaPDU: nrppaPDU,
	})

	return corpus
}

func TestDecoderGolden(t *testing.T) {
	dir := filepath.Join("testdata", "golden")

	if *updateGolden {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
	}

	corpus := goldenCorpus(t)

	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got, err := json.MarshalIndent(DecodeNGAPMessage(corpus[name]), "", "  ")
			if err != nil {
				t.Fatalf("marshal decoded JSON: %v", err)
			}

			got = append(got, '\n')
			path := filepath.Join(dir, name+".json")

			if *updateGolden {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write golden %q: %v", path, err)
				}

				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %q (run with -update to create): %v", path, err)
			}

			if !bytes.Equal(got, want) {
				t.Errorf("decoder JSON changed for %q.\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}

// Every procedure the decoder renders needs a fixture, or a change to its
// renderer lands unreviewed.
func TestGoldenCoversEveryRenderedProcedure(t *testing.T) {
	want := map[string][]lib.ProcedureCode{
		"InitiatingMessage": {
			lib.ProcNGSetup, lib.ProcInitialUEMessage, lib.ProcDownlinkNASTransport,
			lib.ProcUplinkNASTransport, lib.ProcInitialContextSetup,
			lib.ProcPDUSessionResourceSetup, lib.ProcUEContextReleaseRequest,
			lib.ProcUEContextRelease, lib.ProcPDUSessionResourceRelease,
			lib.ProcUERadioCapabilityInfoIndication, lib.ProcAMFStatusIndication,
			lib.ProcPaging, lib.ProcDownlinkUEAssociatedNRPPaTransport,
			lib.ProcUplinkUEAssociatedNRPPaTransport,
			lib.ProcDownlinkNonUEAssociatedNRPPaTransport,
			lib.ProcUplinkNonUEAssociatedNRPPaTransport, lib.ProcErrorIndication,
			lib.ProcLocationReport, lib.ProcLocationReportingControl,
			lib.ProcNGReset, lib.ProcPathSwitchRequest, lib.ProcNASNonDeliveryIndication,
			lib.ProcPDUSessionResourceModify, lib.ProcPDUSessionResourceModifyIndication,
			lib.ProcPDUSessionResourceNotify, lib.ProcRANConfigurationUpdate,
			lib.ProcUplinkRANConfigurationTransfer, lib.ProcDownlinkRANConfigurationTransfer,
			lib.ProcUplinkRANStatusTransfer, lib.ProcDownlinkRANStatusTransfer,
			lib.ProcHandoverPreparation, lib.ProcHandoverResourceAllocation,
			lib.ProcHandoverNotification, lib.ProcHandoverCancel,
		},
		"SuccessfulOutcome": {
			lib.ProcNGSetup, lib.ProcInitialContextSetup, lib.ProcPDUSessionResourceSetup,
			lib.ProcUEContextRelease, lib.ProcPDUSessionResourceRelease,
			lib.ProcNGReset, lib.ProcPathSwitchRequest,
			lib.ProcPDUSessionResourceModify, lib.ProcPDUSessionResourceModifyIndication,
			lib.ProcRANConfigurationUpdate, lib.ProcHandoverPreparation,
			lib.ProcHandoverResourceAllocation, lib.ProcHandoverCancel,
		},
		"UnsuccessfulOutcome": {
			lib.ProcNGSetup, lib.ProcInitialContextSetup, lib.ProcPathSwitchRequest, lib.ProcRANConfigurationUpdate,
			lib.ProcHandoverPreparation, lib.ProcHandoverResourceAllocation,
		},
	}

	covered := map[string]map[int64]bool{}

	for _, raw := range goldenCorpus(t) {
		msg := DecodeNGAPMessage(raw)
		if msg.Value.Error != "" {
			continue
		}

		if covered[msg.PDUType] == nil {
			covered[msg.PDUType] = map[int64]bool{}
		}

		covered[msg.PDUType][msg.ProcedureCode.Value] = true
	}

	for pduType, procedures := range want {
		for _, p := range procedures {
			if !covered[pduType][int64(p)] {
				t.Errorf("no golden fixture decodes %s %s", pduType, p)
			}
		}
	}
}
