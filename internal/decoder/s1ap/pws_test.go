// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

var (
	testCGI2   = s1ap.EUTRANCGI{PLMNIdentity: testPLMN, CellID: 0x0019b02}
	testEAI    = s1ap.EmergencyAreaID{0x01, 0x02, 0x03}
	testENB    = s1ap.GlobalENBID{PLMNIdentity: testPLMN, ENBID: s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 0x12345}}
	testGUMMEI = s1ap.GUMMEI{PLMNIdentity: testPLMN, MMEGroupID: s1ap.MMEGroupID{0x80, 0x01}, MMECode: 1}
)

func testWarningAreaList() *s1ap.WarningAreaList {
	return &s1ap.WarningAreaList{CellIDList: []s1ap.EUTRANCGI{testCGI, testCGI2}}
}

func addPWSOverloadCorpus(t *testing.T, corpus map[string][]byte) {
	t.Helper()

	contents := s1ap.WarningMessageContents{0x01, 0x41, 0x42, 0x43}
	coords := s1ap.WarningAreaCoordinates{0x0a, 0x0b}

	corpus["write_replace_warning_request_full"] = mustMarshal(t, &s1ap.WriteReplaceWarningRequest{
		MessageIdentifier:                 0x1112,
		SerialNumber:                      0x3001,
		WarningAreaList:                   testWarningAreaList(),
		RepetitionPeriod:                  60,
		ExtendedRepetitionPeriod:          s1ap.Ptr(uint32(5000)),
		NumberofBroadcastRequest:          3,
		WarningType:                       &s1ap.WarningType{0x01, 0x80},
		WarningSecurityInfo:               &s1ap.WarningSecurityInfo{0xaa},
		DataCodingScheme:                  s1ap.Ptr(s1ap.DataCodingScheme(0x0f)),
		WarningMessageContents:            &contents,
		ConcurrentWarningMessageIndicator: s1ap.Ptr(s1ap.ConcurrentWarningMessageIndicatorTrue),
		WarningAreaCoordinates:            &coords,
	})

	corpus["write_replace_warning_response_full"] = mustMarshal(t, &s1ap.WriteReplaceWarningResponse{
		MessageIdentifier: 0x1112,
		SerialNumber:      0x3001,
		BroadcastCompletedAreaList: &s1ap.BroadcastCompletedAreaList{
			TAIBroadcast: []s1ap.TAIBroadcastItem{{TAI: testTAI, CompletedCellInTAI: []s1ap.CompletedCellInTAIItem{{ECGI: testCGI}}}},
		},
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["kill_request_full"] = mustMarshal(t, &s1ap.KillRequest{
		MessageIdentifier:      0x1112,
		SerialNumber:           0x3001,
		WarningAreaList:        &s1ap.WarningAreaList{EmergencyAreaIDList: []s1ap.EmergencyAreaID{testEAI}},
		KillAllWarningMessages: s1ap.Ptr(s1ap.KillAllWarningMessagesTrue),
	})

	corpus["kill_response_full"] = mustMarshal(t, &s1ap.KillResponse{
		MessageIdentifier: 0x1112,
		SerialNumber:      0x3001,
		BroadcastCancelledAreaList: &s1ap.BroadcastCancelledAreaList{
			EmergencyAreaIDCancelled: []s1ap.EmergencyAreaIDCancelledItem{{
				EmergencyAreaID:    testEAI,
				CancelledCellInEAI: []s1ap.CancelledCellInEAIItem{{ECGI: testCGI, NumberOfBroadcasts: 4}},
			}},
		},
		CriticalityDiagnostics: testDiagnostics(),
	})

	corpus["pws_restart_indication_full"] = mustMarshal(t, &s1ap.PWSRestartIndication{
		ECGIListForRestart:            []s1ap.EUTRANCGI{testCGI},
		GlobalENBID:                   testENB,
		TAIListForRestart:             []s1ap.TAI{testTAI},
		EmergencyAreaIDListForRestart: []s1ap.EmergencyAreaID{testEAI},
	})

	corpus["pws_failure_indication_full"] = mustMarshal(t, &s1ap.PWSFailureIndication{
		PWSFailedECGIList: []s1ap.EUTRANCGI{testCGI, testCGI2},
		GlobalENBID:       testENB,
	})

	corpus["overload_start_full"] = mustMarshal(t, &s1ap.OverloadStart{
		OverloadResponse:               s1ap.OverloadResponse{OverloadAction: s1ap.OverloadActionRejectRRCCRSignalling},
		GUMMEIList:                     []s1ap.GUMMEI{testGUMMEI},
		TrafficLoadReductionIndication: s1ap.Ptr(uint8(40)),
	})

	corpus["overload_stop_full"] = mustMarshal(t, &s1ap.OverloadStop{GUMMEIList: []s1ap.GUMMEI{testGUMMEI}})

	corpus["reroute_nas_request_full"] = mustMarshal(t, &s1ap.RerouteNASRequest{
		ENBUES1APID:    2,
		MMEUES1APID:    s1ap.Ptr(s1ap.MMEUES1APID(7)),
		S1Message:      s1ap.S1Message{0x00, 0x0c, 0x40, 0x00},
		MMEGroupID:     s1ap.MMEGroupID{0x80, 0x02},
		AdditionalGUTI: &s1ap.AdditionalGUTI{GUMMEI: testGUMMEI, MTMSI: 0xdeadbeef},
		UEUsageType:    s1ap.Ptr(uint8(5)),
	})
}

func TestPWSOverloadDecode(t *testing.T) {
	corpus := map[string][]byte{}
	addPWSOverloadCorpus(t, corpus)

	want := map[string]struct {
		summary string
		ies     int
	}{
		"write_replace_warning_request_full":  {"Write-Replace Warning Request (message 4370, serial 12289)", 12},
		"write_replace_warning_response_full": {"Write-Replace Warning Response (message 4370, serial 12289)", 4},
		"kill_request_full":                   {"Kill Request (message 4370, serial 12289)", 4},
		"kill_response_full":                  {"Kill Response (message 4370, serial 12289)", 4},
		"pws_restart_indication_full":         {"PWS Restart Indication (1 cell(s))", 4},
		"pws_failure_indication_full":         {"PWS Failure Indication (2 cell(s))", 2},
		"overload_start_full":                 {"Overload Start (action 1)", 3},
		"overload_stop_full":                  {"Overload Stop", 1},
		"reroute_nas_request_full":            {"Reroute NAS Request (eNB UE 2)", 6},
	}

	for name, w := range want {
		msg := DecodeS1APMessage(corpus[name])

		if msg.Value.Error != "" {
			t.Errorf("%s: %s", name, msg.Value.Error)
			continue
		}

		if msg.Summary != w.summary {
			t.Errorf("%s: summary %q, want %q", name, msg.Summary, w.summary)
		}

		if len(msg.Value.IEs) != w.ies {
			t.Errorf("%s: %d IEs, want %d", name, len(msg.Value.IEs), w.ies)
		}

		for _, e := range msg.Value.IEs {
			if e.ID.Unknown || e.ID.Label == "" {
				t.Errorf("%s: IE %d has no name", name, e.ID.Value)
			}
		}
	}
}
