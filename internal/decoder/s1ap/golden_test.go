// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ellanetworks/core/lppa"
	"github.com/ellanetworks/core/s1ap"
)

// Regenerate with: go test ./internal/decoder/s1ap/ -run TestDecoderGolden -update
var updateGolden = flag.Bool("update", false, "regenerate decoder golden JSON fixtures")

func mustHex(t *testing.T, h string) []byte {
	t.Helper()

	b, err := hex.DecodeString(h)
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
	testPLMN = s1ap.PLMNIdentity{0x00, 0xf1, 0x10}
	testTAI  = s1ap.TAI{PLMNIdentity: testPLMN, TAC: 1}
	testCGI  = s1ap.EUTRANCGI{PLMNIdentity: testPLMN, CellID: 0x0019b01}
)

func testDiagnostics() *s1ap.CriticalityDiagnostics {
	return &s1ap.CriticalityDiagnostics{
		ProcedureCode:        s1ap.Ptr(s1ap.ProcS1Setup),
		TriggeringMessage:    s1ap.Ptr(s1ap.TriggeringInitiatingMessage),
		ProcedureCriticality: s1ap.Ptr(s1ap.CriticalityReject),
		IEsCriticalityDiagnostics: []s1ap.CriticalityDiagnosticsIEItem{{
			IEID:          s1ap.IDGlobalENBID,
			IECriticality: s1ap.CriticalityReject,
			TypeOfError:   s1ap.TypeOfErrorMissing,
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
		"s1_setup_request":              mustHex(t, s1SetupRequestCapture),
		"s1_setup_response":             mustHex(t, s1SetupResponseCapture),
		"initial_context_setup_request": mustHex(t, initialContextSetupRequestCapture),
		"initial_context_setup_res":     mustHex(t, initialContextSetupResponseCapture),
		"initial_ue_message":            mustHex(t, initialUEMessageCapture),
		"uplink_nas_transport":          mustHex(t, uplinkNASTransportCapture),
		"downlink_nas_transport":        mustHex(t, downlinkNASTransportCapture),
		"ue_context_release_request":    mustHex(t, ueContextReleaseRequestCapture),
		"ue_context_release_command":    mustHex(t, ueContextReleaseCommandCapture),
		"ue_context_release_complete":   mustHex(t, ueContextReleaseCompleteCapture),
		"paging":                        mustHex(t, pagingCapture),
		"ue_capability_info_indication": mustHex(t, ueCapabilityInfoIndicationCapture),
		"erab_setup_request":            mustHex(t, eRABSetupRequestCapture),
		"erab_setup_response":           mustHex(t, eRABSetupResponseCapture),
		"erab_release_command":          mustHex(t, eRABReleaseCommandCapture),
		"erab_release_response":         mustHex(t, eRABReleaseResponseCapture),
		"invalid":                       {0xff, 0x00, 0x01},
	}

	corpus["erab_modify_request_full"] = mustMarshal(t, &s1ap.ERABModifyRequest{
		MMEUES1APID:               1,
		ENBUES1APID:               2,
		UEAggregateMaximumBitRate: &s1ap.UEAggregateMaximumBitRate{DL: 200000000, UL: 100000000},
		ERABToBeModified: []s1ap.ERABToBeModifiedItemBearerModReq{{
			ERABID: 5,
			QoS:    s1ap.ERABLevelQoSParameters{QCI: 9, ARP: s1ap.AllocationAndRetentionPriority{PriorityLevel: 15}},
			NASPDU: s1ap.NASPDU{0x07, 0x4b, 0x09},
		}},
	})

	corpus["erab_modify_response_full"] = mustMarshal(t, &s1ap.ERABModifyResponse{
		MMEUES1APID: s1ap.Ptr(s1ap.MMEUES1APID(1)),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(2)),
		ERABModify:  []s1ap.ERABModifyItemBearerModRes{{ERABID: 5}},
		ERABFailedToModify: []s1ap.ERABItem{
			{ERABID: 6, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}},
		},
		CriticalityDiagnostics:  &s1ap.CriticalityDiagnostics{ProcedureCode: s1ap.Ptr(s1ap.ProcERABModify)},
		UserLocationInformation: &s1ap.UserLocationInformation{EUTRANCGI: testCGI, TAI: testTAI},
	})

	corpus["erab_modification_indication_full"] = mustMarshal(t, &s1ap.ERABModificationIndication{
		MMEUES1APID: 1,
		ENBUES1APID: 2,
		ToBeModified: []s1ap.ERABToBeModifiedItemBearerModInd{
			{ERABID: 5, TransportLayerAddress: s1ap.TransportLayerAddress{10, 45, 0, 1}, DLGTPTEID: 0x21},
		},
		NotToBeModified: []s1ap.ERABToBeModifiedItemBearerModInd{
			{ERABID: 6, TransportLayerAddress: s1ap.TransportLayerAddress{10, 45, 0, 2}, DLGTPTEID: 0x22},
		},
		UserLocationInformation: &s1ap.UserLocationInformation{EUTRANCGI: testCGI, TAI: testTAI},
	})

	corpus["erab_modification_confirm_full"] = mustMarshal(t, &s1ap.ERABModificationConfirm{
		MMEUES1APID:            s1ap.Ptr(s1ap.MMEUES1APID(1)),
		ENBUES1APID:            s1ap.Ptr(s1ap.ENBUES1APID(2)),
		ModifiedERABs:          []s1ap.ERABID{5, 6},
		CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{ProcedureCode: s1ap.Ptr(s1ap.ProcERABModificationIndication)},
	})

	corpus["reset_all_full"] = mustMarshal(t, &s1ap.Reset{
		Cause:     &s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: s1ap.CauseMiscUnspecified},
		ResetType: s1ap.ResetType{All: true},
	})

	corpus["reset_part_full"] = mustMarshal(t, &s1ap.Reset{
		Cause: &s1ap.Cause{Group: s1ap.CauseGroupTransport, Value: 0},
		ResetType: s1ap.ResetType{Part: []s1ap.UEAssociatedLogicalS1ConnectionItem{
			{MMEUES1APID: s1ap.Ptr(s1ap.MMEUES1APID(7)), ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(3))},
			{ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(4))},
		}},
	})

	corpus["reset_acknowledge_full"] = mustMarshal(t, &s1ap.ResetAcknowledge{
		ConnectionList: []s1ap.UEAssociatedLogicalS1ConnectionItem{
			{MMEUES1APID: s1ap.Ptr(s1ap.MMEUES1APID(7)), ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(3))},
		},
		CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{ProcedureCode: s1ap.Ptr(s1ap.ProcReset)},
	})

	corpus["path_switch_request_full"] = mustMarshal(t, &s1ap.PathSwitchRequest{
		ENBUES1APID: 9,
		ERABToBeSwitchedDL: []s1ap.ERABToBeSwitchedDLItem{
			{ERABID: 5, TransportLayerAddress: s1ap.TransportLayerAddress{10, 45, 0, 1}, GTPTEID: 0x11},
		},
		SourceMMEUES1APID:      4,
		EUTRANCGI:              s1ap.Ptr(testCGI),
		TAI:                    s1ap.Ptr(testTAI),
		UESecurityCapabilities: s1ap.Ptr(s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xe000, IntegrityProtectionAlgorithms: 0xe000}),
	})

	corpus["path_switch_request_acknowledge_full"] = mustMarshal(t, &s1ap.PathSwitchRequestAcknowledge{
		MMEUES1APID:               s1ap.Ptr(s1ap.MMEUES1APID(4)),
		ENBUES1APID:               s1ap.Ptr(s1ap.ENBUES1APID(9)),
		UEAggregateMaximumBitRate: &s1ap.UEAggregateMaximumBitRate{DL: 200000000, UL: 100000000},
		ERABToBeReleased: []s1ap.ERABItem{
			{ERABID: 6, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}},
		},
		SecurityContext:        s1ap.SecurityContext{NextHopChainingCount: 3, NextHopParameter: s1ap.SecurityKey{0x01, 0x02, 0x03}},
		UESecurityCapabilities: s1ap.Ptr(s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xe000, IntegrityProtectionAlgorithms: 0xe000}),
	})

	corpus["path_switch_request_failure_full"] = mustMarshal(t, &s1ap.PathSwitchRequestFailure{
		MMEUES1APID:            s1ap.Ptr(s1ap.MMEUES1APID(4)),
		ENBUES1APID:            s1ap.Ptr(s1ap.ENBUES1APID(9)),
		Cause:                  &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 6},
		CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{ProcedureCode: s1ap.Ptr(s1ap.ProcPathSwitchRequest)},
	})

	corpus["nas_non_delivery_indication_full"] = mustMarshal(t, &s1ap.NASNonDeliveryIndication{
		MMEUES1APID: 4,
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU{0x07, 0x4b, 0x09},
		Cause:       &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 27},
	})

	corpus["s1_setup_request_full"] = mustMarshal(t, &s1ap.S1SetupRequest{
		GlobalENBID: s1ap.GlobalENBID{
			PLMNIdentity: testPLMN,
			ENBID:        s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 411},
		},
		ENBName: s1ap.Ptr("srsenb01"),
		SupportedTAs: s1ap.SupportedTAs{{
			TAC:            1,
			BroadcastPLMNs: s1ap.BPLMNs{testPLMN},
		}},
		DefaultPagingDRX:       s1ap.Ptr(s1ap.PagingDRXv128),
		UERetentionInformation: s1ap.Ptr(s1ap.UERetentionUesRetained),
	})

	corpus["s1_setup_response_full"] = mustMarshal(t, &s1ap.S1SetupResponse{
		MMEName: s1ap.Ptr("ella-core"),
		ServedGUMMEIs: s1ap.ServedGUMMEIs{{
			ServedPLMNs:    []s1ap.PLMNIdentity{testPLMN},
			ServedGroupIDs: []s1ap.MMEGroupID{{0x00, 0x01}},
			ServedMMECs:    []s1ap.MMECode{1},
		}},
		RelativeMMECapacity:    s1ap.Ptr(uint8(255)),
		CriticalityDiagnostics: testDiagnostics(),
		UERetentionInformation: s1ap.Ptr(s1ap.UERetentionUesRetained),
	})

	corpus["s1_setup_failure_full"] = mustMarshal(t, &s1ap.S1SetupFailure{
		Cause:                  &s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 3},
		TimeToWait:             s1ap.Ptr(s1ap.TimeToWaitV10s),
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["ue_capability_info_indication_full"] = mustMarshal(t, &s1ap.UECapabilityInfoIndication{
		MMEUES1APID:                10,
		ENBUES1APID:                810,
		UERadioCapability:          s1ap.UERadioCapability{0x01, 0x02},
		UERadioCapabilityForPaging: s1ap.UERadioCapabilityForPaging{0xab, 0xcd},
	})

	corpus["paging_full"] = mustMarshal(t, &s1ap.Paging{
		UEIdentityIndexValue:       s1ap.Ptr(uint16(42)),
		STMSI:                      &s1ap.STMSI{MMEC: 1, MTMSI: 0x01020304},
		PagingDRX:                  s1ap.Ptr(s1ap.PagingDRXv64),
		CNDomain:                   s1ap.Ptr(s1ap.CNDomainPS),
		TAIList:                    []s1ap.TAI{testTAI},
		PagingPriority:             s1ap.Ptr(s1ap.PagingPriorityLevel1),
		UERadioCapabilityForPaging: []byte{0xab, 0xcd},
	})

	corpus["error_indication_full"] = mustMarshal(t, &s1ap.ErrorIndication{
		MMEUES1APID:            s1ap.Ptr(s1ap.MMEUES1APID(7)),
		ENBUES1APID:            s1ap.Ptr(s1ap.ENBUES1APID(807)),
		Cause:                  &s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: 0},
		CriticalityDiagnostics: testDiagnostics(),
		STMSI:                  &s1ap.STMSI{MMEC: 1, MTMSI: 0x01020304},
	})

	corpus["initial_context_setup_failure_full"] = mustMarshal(t, &s1ap.InitialContextSetupFailure{
		MMEUES1APID:            s1ap.Ptr(s1ap.MMEUES1APID(7)),
		ENBUES1APID:            s1ap.Ptr(s1ap.ENBUES1APID(807)),
		Cause:                  &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["ue_context_release_complete_full"] = mustMarshal(t, &s1ap.UEContextReleaseComplete{
		MMEUES1APID:             s1ap.Ptr(s1ap.MMEUES1APID(7)),
		ENBUES1APID:             s1ap.Ptr(s1ap.ENBUES1APID(807)),
		CriticalityDiagnostics:  testDiagnostics(),
		UserLocationInformation: &s1ap.UserLocationInformation{EUTRANCGI: testCGI, TAI: testTAI},
	})

	corpus["handover_required_full"] = mustMarshal(t, &s1ap.HandoverRequired{
		MMEUES1APID:  7,
		ENBUES1APID:  2,
		HandoverType: s1ap.HandoverTypeIntraLTE,
		Cause:        &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 16},
		TargetID: s1ap.TargetID{TargeteNBID: s1ap.TargeteNBID{
			GlobalENBID: s1ap.GlobalENBID{PLMNIdentity: testPLMN, ENBID: s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 2}},
			SelectedTAI: testTAI,
		}},
		DirectForwardingPathAvailability: s1ap.Ptr(s1ap.DirectForwardingPathAvailable),
		SourceToTarget:                   s1ap.TransparentContainer{0xab, 0xcd},
	})

	corpus["handover_command_full"] = mustMarshal(t, &s1ap.HandoverCommand{
		MMEUES1APID:                     7,
		ENBUES1APID:                     2,
		HandoverType:                    s1ap.HandoverTypeLTEtoUTRAN,
		NASSecurityParametersfromEUTRAN: s1ap.NASSecurityParametersfromEUTRAN{0x01, 0x02},
		ERABSubjecttoDataForwarding: []s1ap.ERABDataForwardingItem{{
			ERABID:               5,
			DLTransportLayerAddr: s1ap.TransportLayerAddress{10, 4, 0, 2},
			DLGTPTEID:            s1ap.Ptr(s1ap.GTPTEID(0x99)),
		}},
		ERABToRelease:          []s1ap.ERABItem{{ERABID: 6, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:         s1ap.TransparentContainer{0x11},
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["handover_preparation_failure_full"] = mustMarshal(t, &s1ap.HandoverPreparationFailure{
		MMEUES1APID:            s1ap.Ptr(s1ap.MMEUES1APID(7)),
		ENBUES1APID:            s1ap.Ptr(s1ap.ENBUES1APID(2)),
		Cause:                  &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["handover_request_full"] = mustMarshal(t, &s1ap.HandoverRequest{
		MMEUES1APID:  7,
		HandoverType: s1ap.HandoverTypeUTRANtoLTE,
		Cause:        &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 16},
		UEAMBR:       s1ap.UEAggregateMaximumBitRate{DL: 100_000_000, UL: 50_000_000},
		ERABToBeSetup: []s1ap.ERABToBeSetupItemHOReq{{
			ERABID:                5,
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
			QoS:                   s1ap.ERABLevelQoSParameters{QCI: 9, ARP: s1ap.AllocationAndRetentionPriority{PriorityLevel: 15}},
		}},
		SourceToTarget:         s1ap.TransparentContainer{0xab},
		UESecurityCapabilities: s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000},
		HandoverRestrictionList: &s1ap.HandoverRestrictionList{
			ServingPLMN:        testPLMN,
			EquivalentPLMNs:    s1ap.EPLMNs{testPLMN},
			ForbiddenTAs:       s1ap.ForbiddenTAs{{PLMNIdentity: testPLMN, ForbiddenTACs: s1ap.ForbiddenTACs{7}}},
			ForbiddenLAs:       s1ap.ForbiddenLAs{{PLMNIdentity: testPLMN, ForbiddenLACs: s1ap.ForbiddenLACs{9}}},
			ForbiddenInterRATs: s1ap.Ptr(s1ap.ForbiddenInterRATsGERAN),
		},
		SecurityContext:               s1ap.SecurityContext{NextHopChainingCount: 1},
		NASSecurityParameterstoEUTRAN: s1ap.NASSecurityParameterstoEUTRAN{0x0a, 0x0b},
	})

	corpus["handover_request_acknowledge_full"] = mustMarshal(t, &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: s1ap.Ptr(s1ap.MMEUES1APID(7)),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(55)),
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                5,
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		ERABFailedToSetup:      []s1ap.ERABItem{{ERABID: 6, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:         s1ap.TransparentContainer{0x11},
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["handover_failure_full"] = mustMarshal(t, &s1ap.HandoverFailure{
		MMEUES1APID:            s1ap.Ptr(s1ap.MMEUES1APID(7)),
		Cause:                  &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["handover_notify_full"] = mustMarshal(t, &s1ap.HandoverNotify{
		MMEUES1APID: 7,
		ENBUES1APID: 2,
		EUTRANCGI:   &testCGI,
		TAI:         &testTAI,
	})

	corpus["handover_cancel_full"] = mustMarshal(t, &s1ap.HandoverCancel{
		MMEUES1APID: 7,
		ENBUES1APID: 2,
		Cause:       &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	})

	corpus["handover_cancel_acknowledge_full"] = mustMarshal(t, &s1ap.HandoverCancelAcknowledge{
		MMEUES1APID:            s1ap.Ptr(s1ap.MMEUES1APID(7)),
		ENBUES1APID:            s1ap.Ptr(s1ap.ENBUES1APID(2)),
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["enb_status_transfer"] = mustMarshal(t, &s1ap.ENBStatusTransfer{
		MMEUES1APID: 7,
		ENBUES1APID: 2,
		Container:   s1ap.StatusTransferContainer{0x01, 0x02},
	})

	corpus["mme_status_transfer"] = mustMarshal(t, &s1ap.MMEStatusTransfer{
		MMEUES1APID: 7,
		ENBUES1APID: 2,
		Container:   s1ap.StatusTransferContainer{0x03, 0x04},
	})

	lppaPDU, err := lppa.BuildECIDMeasurementInitiationRequest(11, []lppa.MeasurementQuantityValue{lppa.MeasCellID, lppa.MeasRSRP})
	if err != nil {
		t.Fatalf("build LPPa PDU: %v", err)
	}

	corpus["downlink_ue_associated_lppa_transport"] = mustMarshal(t, &s1ap.DownlinkUEAssociatedLPPaTransport{
		MMEUES1APID: 5,
		ENBUES1APID: 7,
		RoutingID:   3,
		LPPaPDU:     lppaPDU,
	})

	corpus["uplink_ue_associated_lppa_transport"] = mustMarshal(t, &s1ap.UplinkUEAssociatedLPPaTransport{
		MMEUES1APID: 5,
		ENBUES1APID: 7,
		RoutingID:   3,
		LPPaPDU:     lppaPDU,
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
			got, err := json.MarshalIndent(DecodeS1APMessage(corpus[name]), "", "  ")
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
	want := map[string][]s1ap.ProcedureCode{
		"InitiatingMessage": {
			s1ap.ProcS1Setup, s1ap.ProcInitialUEMessage, s1ap.ProcUplinkNASTransport,
			s1ap.ProcDownlinkNASTransport, s1ap.ProcInitialContextSetup,
			s1ap.ProcUEContextReleaseRequest, s1ap.ProcUEContextRelease,
			s1ap.ProcUECapabilityInfoIndication, s1ap.ProcPaging, s1ap.ProcErrorIndication,
			s1ap.ProcHandoverPreparation, s1ap.ProcHandoverResourceAllocation,
			s1ap.ProcHandoverNotification, s1ap.ProcHandoverCancel,
			s1ap.ProcENBStatusTransfer, s1ap.ProcMMEStatusTransfer,
			s1ap.ProcDownlinkUEAssociatedLPPaTransport, s1ap.ProcUplinkUEAssociatedLPPaTransport,
			s1ap.ProcERABSetup, s1ap.ProcERABRelease,
			s1ap.ProcReset, s1ap.ProcPathSwitchRequest, s1ap.ProcNASNonDeliveryIndication,
			s1ap.ProcERABModify, s1ap.ProcERABModificationIndication,
		},
		"SuccessfulOutcome": {
			s1ap.ProcS1Setup, s1ap.ProcInitialContextSetup, s1ap.ProcUEContextRelease,
			s1ap.ProcHandoverPreparation, s1ap.ProcHandoverResourceAllocation, s1ap.ProcHandoverCancel,
			s1ap.ProcERABSetup, s1ap.ProcERABRelease,
			s1ap.ProcReset, s1ap.ProcPathSwitchRequest,
			s1ap.ProcERABModify, s1ap.ProcERABModificationIndication,
		},
		"UnsuccessfulOutcome": {
			s1ap.ProcS1Setup, s1ap.ProcInitialContextSetup,
			s1ap.ProcHandoverPreparation, s1ap.ProcHandoverResourceAllocation,
			s1ap.ProcPathSwitchRequest,
		},
	}

	covered := map[string]map[int64]bool{}

	for _, raw := range goldenCorpus(t) {
		msg := DecodeS1APMessage(raw)
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
