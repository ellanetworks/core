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
		},
		"SuccessfulOutcome": {
			lib.ProcNGSetup, lib.ProcInitialContextSetup, lib.ProcPDUSessionResourceSetup,
			lib.ProcUEContextRelease, lib.ProcPDUSessionResourceRelease,
		},
		"UnsuccessfulOutcome": {lib.ProcNGSetup, lib.ProcInitialContextSetup},
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
