// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/utils"
	lib "github.com/ellanetworks/core/ngap"
)

var (
	pwsNRCGI    = lib.NRCGI{PLMNIdentity: testPLMN, NRCellIdentity: 0x123456789}
	pwsEUTRACGI = lib.EUTRACGI{PLMNIdentity: testPLMN, EUTRACellIdentity: 0x1234567}
	pwsGNB      = lib.GlobalRANNodeID{Kind: lib.RANNodeIDGNB, PLMNIdentity: testPLMN, Value: 1, Bits: 22}
)

func pwsCBSData() lib.WarningMessageContents {
	page := make([]byte, 82)
	copy(page, []byte{0xd4, 0xf2, 0x9c, 0x0e})

	return append(append(lib.WarningMessageContents{0x01}, page...), 0x04)
}

func pwsOverloadCorpus(t *testing.T) map[string][]byte {
	t.Helper()

	return map[string][]byte{
		"overload_start_full": mustMarshal(t, &lib.OverloadStart{
			AMFOverloadResponse:               &lib.OverloadResponse{OverloadAction: lib.OverloadActionRejectRRCCRSignalling},
			AMFTrafficLoadReductionIndication: lib.Ptr(lib.TrafficLoadReductionIndication(50)),
			OverloadStartNSSAIList: lib.OverloadStartNSSAIList{{
				SliceOverloadList:                   []lib.SliceOverloadItem{{SNSSAI: lib.SNSSAI{SST: 1, SD: &lib.SD{0x00, 0x00, 0x01}}}},
				SliceOverloadResponse:               &lib.OverloadResponse{OverloadAction: lib.OverloadActionPermitEmergencySessionsAndMTServicesOnly},
				SliceTrafficLoadReductionIndication: lib.Ptr(lib.TrafficLoadReductionIndication(20)),
			}},
		}),
		"overload_stop": mustMarshal(t, &lib.OverloadStop{}),
		"write_replace_warning_request_full": mustMarshal(t, &lib.WriteReplaceWarningRequest{
			MessageIdentifier:           0x1112,
			SerialNumber:                0x3456,
			WarningAreaList:             &lib.WarningAreaList{NRCGIs: []lib.NRCGI{pwsNRCGI}},
			RepetitionPeriod:            10,
			NumberOfBroadcastsRequested: 1,
			WarningType:                 &lib.WarningType{0x05, 0x80},
			WarningSecurityInfo:         &lib.WarningSecurityInfo{0x01},
			DataCodingScheme:            lib.Ptr(lib.DataCodingScheme(0x0f)),
			WarningMessageContents:      pwsCBSData(),
			ConcurrentWarningMessageInd: lib.Ptr(lib.ConcurrentWarningMessageIndTrue),
			WarningAreaCoordinates:      lib.WarningAreaCoordinates{0xaa, 0xbb},
		}),
		"write_replace_warning_response_full": mustMarshal(t, &lib.WriteReplaceWarningResponse{
			MessageIdentifier: 0x1112,
			SerialNumber:      0x3456,
			BroadcastCompletedAreaList: &lib.BroadcastCompletedAreaList{TAIBroadcastNR: []lib.TAIBroadcastNRItem{{
				TAI:                 testTAI,
				CompletedCellsInTAI: []lib.CompletedCellsInTAINRItem{{NRCGI: pwsNRCGI}},
			}}},
			CriticalityDiagnostics: testDiagnostics(),
		}),
		"pws_cancel_request_full": mustMarshal(t, &lib.PWSCancelRequest{
			MessageIdentifier:        0x1112,
			SerialNumber:             0x3456,
			WarningAreaList:          &lib.WarningAreaList{TAIs: []lib.TAI{testTAI}},
			CancelAllWarningMessages: lib.Ptr(lib.CancelAllWarningMessagesTrue),
		}),
		"pws_cancel_response_full": mustMarshal(t, &lib.PWSCancelResponse{
			MessageIdentifier: 0x1112,
			SerialNumber:      0x3456,
			BroadcastCancelledAreaList: &lib.BroadcastCancelledAreaList{EmergencyAreaIDCancelledEUTRA: []lib.EmergencyAreaIDCancelledEUTRAItem{{
				EmergencyAreaID:     lib.EmergencyAreaID{0x01, 0x02, 0x03},
				CancelledCellsInEAI: []lib.CancelledCellsInEAIEUTRAItem{{EUTRACGI: pwsEUTRACGI, NumberOfBroadcasts: 3}},
			}}},
			CriticalityDiagnostics: testDiagnostics(),
		}),
		"pws_restart_indication_full": mustMarshal(t, &lib.PWSRestartIndication{
			CellIDListForRestart:          lib.CellIDListForRestart{NRCGIs: []lib.NRCGI{pwsNRCGI}},
			GlobalRANNodeID:               pwsGNB,
			TAIListForRestart:             lib.TAIListForRestart{testTAI},
			EmergencyAreaIDListForRestart: lib.EmergencyAreaIDListForRestart{{0x01, 0x02, 0x03}},
		}),
		"pws_failure_indication": mustMarshal(t, &lib.PWSFailureIndication{
			PWSFailedCellIDList: lib.PWSFailedCellIDList{EUTRACGIs: []lib.EUTRACGI{pwsEUTRACGI}},
			GlobalRANNodeID:     pwsGNB,
		}),
		"reroute_nas_request_full": mustMarshal(t, &lib.RerouteNASRequest{
			RANUENGAPID:  5,
			AMFUENGAPID:  lib.Ptr(lib.AMFUENGAPID(7)),
			NGAPMessage:  lib.NGAPMessage{0x00, 0x0f, 0x40, 0x00},
			AMFSetID:     1,
			AllowedNSSAI: lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
			SourceToTargetAMFInformationReroute: &lib.SourceToTargetAMFInformationReroute{
				RejectedNSSAIinPLMN: &lib.RejectedNSSAIinPLMN{0x01},
			},
			PartiallyAllowedNSSAI: lib.PartiallyAllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 2}}},
		}),
	}
}

func TestDecodeNGAPMessage_PWSAndOverload(t *testing.T) {
	for name, raw := range pwsOverloadCorpus(t) {
		t.Run(name, func(t *testing.T) {
			msg := DecodeNGAPMessage(raw)

			if msg.Value.Error != "" {
				t.Fatalf("decode error: %s", msg.Value.Error)
			}

			for _, e := range msg.Value.IEs {
				if e.ID.Unknown {
					t.Errorf("IE %d rendered as unknown", e.ID.Value)
				}

				if _, raw := e.Value.(rawIEValue); raw {
					t.Errorf("IE %s rendered as unmodeled", e.ID.Label)
				}
			}
		})
	}
}

func TestDecodeNGAPMessage_RerouteNASRequestSummary(t *testing.T) {
	msg := DecodeNGAPMessage(pwsOverloadCorpus(t)["reroute_nas_request_full"])

	if want := "RerouteNASRequest, RAN-UE=5, AMF-UE=7"; msg.Summary != want {
		t.Fatalf("summary %q, want %q", msg.Summary, want)
	}
}

func TestDecodeNGAPMessage_OverloadStartAction(t *testing.T) {
	msg := DecodeNGAPMessage(pwsOverloadCorpus(t)["overload_start_full"])

	if len(msg.Value.IEs) != 3 {
		t.Fatalf("got %d IEs, want 3", len(msg.Value.IEs))
	}

	action, ok := msg.Value.IEs[0].Value.(utils.EnumField)
	if !ok || action.Label != "reject-rrc-cr-signalling" || action.Unknown {
		t.Fatalf("AMF Overload Response = %+v", msg.Value.IEs[0].Value)
	}
}
