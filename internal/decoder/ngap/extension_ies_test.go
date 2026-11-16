// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/json"
	"strings"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/per/ap"
)

func extensionIEsCorpus(t *testing.T) map[string][]byte {
	t.Helper()

	sidelink := &lib.NRUESidelinkAggregateMaximumBitrate{UESidelinkAggregateMaximumBitRate: 1000000}
	diff := &lib.UEDifferentiationInfo{
		PeriodicCommunicationIndicator: lib.Ptr(lib.UEDifferentiationInfoPeriodicCommunicationIndicatorPeriodically),
		PeriodicTime:                   lib.Ptr(lib.UEDifferentiationInfoPeriodicTime(60)),
	}
	inactive := &lib.CoreNetworkAssistanceInformationForInactive{
		UEIdentityIndexValue:            lib.UEIdentityIndexValue{IndexLength10: lib.Ptr(lib.UEIdentityIndexValueIndexLength10(0x155))},
		PeriodicRegistrationUpdateTimer: 0x21,
		MICOModeIndication:              lib.Ptr(lib.MICOModeIndicationTrue),
		TAIListForInactive:              lib.TAIListForInactive{{TAI: testTAI}},
		EUTRAPagingeDRXInformation:      &lib.EUTRAPagingeDRXInformation{EUTRApagingeDRXCycle: lib.EUTRAPagingeDRXCycleHf2},
	}

	setupTransfer, err := (&lib.PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation: lib.UPTransportLayerInformation{GTPTunnel: &lib.GTPTunnel{TransportLayerAddress: ap.TransportLayerAddressFromOctets([]byte{10, 0, 0, 1}), GTPTEID: 0x11}},
		PDUSessionType:        lib.PDUSessionTypeIPv4,
		QosFlowSetupRequest: lib.QosFlowSetupRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: lib.QosFlowLevelQosParameters{
				QosCharacteristics:             lib.QosCharacteristics{NonDynamic5QI: &lib.NonDynamic5QIDescriptor{FiveQI: 9}},
				AllocationAndRetentionPriority: lib.AllocationAndRetentionPriority{PriorityLevelARP: 8},
			},
		}},
		CommonNetworkInstance:          lib.CommonNetworkInstance{0x69, 0x6e},
		RedundantPDUSessionInformation: &lib.RedundantPDUSessionInformation{RSN: lib.RSNV2},
	}).Marshal()
	if err != nil {
		t.Fatalf("build setup transfer: %v", err)
	}

	return map[string][]byte{
		"downlink_nas_transport_extensions": mustMarshal(t, &lib.DownlinkNASTransport{
			AMFUENGAPID:               1,
			RANUENGAPID:               2,
			NASPDU:                    lib.NASPDU{0x7e, 0x00, 0x44},
			OldAMF:                    lib.Ptr(lib.Name("amf-old")),
			RANPagingPriority:         lib.Ptr(lib.RANPagingPriority(3)),
			IndexToRFSP:               lib.Ptr(lib.IndexToRFSP(7)),
			UEAggregateMaximumBitRate: &lib.UEAggregateMaximumBitRate{DL: 2000, UL: 1000},
			AllowedNSSAI:              lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
			SRVCCOperationPossible:    lib.Ptr(lib.SRVCCOperationPossiblePossible),
			UEDifferentiationInfo:     diff,
			EndIndication:             lib.Ptr(lib.EndIndicationNoFurtherData),
			UERadioCapabilityID:       lib.Ptr(lib.UERadioCapabilityID{0xca, 0xfe}),
			MaskedIMEISV:              lib.Ptr(lib.MaskedIMEISV(0x0123456789abcdef)),
			PartiallyAllowedNSSAI:     lib.PartiallyAllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 2}}},
		}),
		"initial_context_setup_request_extensions": mustMarshal(t, &lib.InitialContextSetupRequest{
			AMFUENGAPID:  1,
			RANUENGAPID:  2,
			GUAMI:        lib.GUAMI{PLMNIdentity: testPLMN, AMFRegionID: 2, AMFSetID: 1},
			AllowedNSSAI: lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
			UESecurityCapabilities: lib.UESecurityCapabilities{
				NREncryptionAlgorithms: lib.SecurityAlgorithmsFrom(0xc000), NRIntegrityProtectionAlgorithms: lib.SecurityAlgorithmsFrom(0xc000),
				EUTRAEncryptionAlgorithms: lib.SecurityAlgorithmsFrom(0xc000), EUTRAIntegrityProtectionAlgorithms: lib.SecurityAlgorithmsFrom(0xc000),
			},
			CoreNetworkAssistanceInformationForInactive: inactive,
			NRV2XServicesAuthorized:                     &lib.NRV2XServicesAuthorized{VehicleUE: lib.Ptr(lib.VehicleUEAuthorized)},
			NRUESidelinkAggregateMaximumBitrate:         sidelink,
			ManagementBasedMDTPLMNList:                  lib.MDTPLMNList{testPLMN},
			UESliceMaximumBitRateList:                   lib.UESliceMaximumBitRateList{{SNSSAI: lib.SNSSAI{SST: 1}, UESliceMaximumBitRateDL: 300, UESliceMaximumBitRateUL: 200}},
			FiveGProSeUEPC5AggregateMaximumBitRate:      sidelink,
		}),
		"pdu_session_resource_setup_request_transfer_extensions": mustMarshal(t, &lib.PDUSessionResourceSetupRequest{
			AMFUENGAPID:       1,
			RANUENGAPID:       2,
			RANPagingPriority: lib.Ptr(lib.RANPagingPriority(1)),
			PDUSessionResourceSetup: lib.PDUSessionResourceSetupListSUReq{{
				PDUSessionID: 1,
				SNSSAI:       lib.SNSSAI{SST: 1},
				Transfer:     setupTransfer,
			}},
		}),
		"ng_setup_request_extensions": mustMarshal(t, &lib.NGSetupRequest{
			GlobalRANNodeID:       lib.NewGlobalGNBID(testPLMN, 1, 22),
			SupportedTAList:       lib.SupportedTAList{{TAC: 1, BroadcastPLMNList: lib.BroadcastPLMNList{{PLMNIdentity: testPLMN, TAISliceSupportList: lib.SliceSupportList{{SNSSAI: lib.SNSSAI{SST: 1}}}}}}},
			DefaultPagingDRX:      lib.Ptr(lib.PagingDRXv128),
			ExtendedRANNodeName:   &lib.ExtendedRANNodeName{RANNodeNameVisibleString: lib.Ptr(lib.RANNodeNameVisibleString("gnb-ext"))},
			NBIoTDefaultPagingDRX: lib.Ptr(lib.NBIoTDefaultPagingDRXRf256),
		}),
		"paging_extensions": mustMarshal(t, &lib.Paging{
			UEPagingIdentity: &lib.UEPagingIdentity{FiveGSTMSI: &lib.FiveGSTMSI{AMFSetID: 1, AMFPointer: 2, FiveGTMSI: 0x01020304}},
			TAIListForPaging: lib.TAIListForPaging{{TAI: testTAI}},
			PagingCause:      lib.Ptr(lib.PagingCauseVoice),
		}),
		"ue_context_modification_response_extensions": mustMarshal(t, &lib.UEContextModificationResponse{
			AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(7)),
			RANUENGAPID: lib.Ptr(lib.RANUENGAPID(3)),
			RRCState:    lib.Ptr(lib.RRCStateConnected),
		}),
		"uplink_nas_transport_extensions": mustMarshal(t, &lib.UplinkNASTransport{
			AMFUENGAPID:             1,
			RANUENGAPID:             2,
			NASPDU:                  lib.NASPDU{0x7e, 0x00, 0x43},
			UserLocationInformation: &lib.UserLocationInformation{NR: &lib.UserLocationInformationNR{NRCGI: lib.NRCGI{PLMNIdentity: testPLMN, NRCellIdentity: 0x10}, TAI: testTAI}},
			TNGFIdentityInformation: lib.Ptr(lib.IdentityInformation{0x01, 0x02}),
		}),
	}
}

func TestExtensionIEsRendered(t *testing.T) {
	for name, raw := range extensionIEsCorpus(t) {
		t.Run(name, func(t *testing.T) {
			msg := DecodeNGAPMessage(raw)
			if msg.Value.Error != "" {
				t.Fatalf("decode error: %s", msg.Value.Error)
			}

			pdu, err := lib.ParseRawPDU(raw)
			if err != nil {
				t.Fatal(err)
			}

			wire := pdu.IEs()
			if len(wire) != len(msg.Value.IEs) {
				t.Fatalf("rendered %d IEs, want the %d on the wire", len(msg.Value.IEs), len(wire))
			}

			for i, w := range wire {
				got := msg.Value.IEs[i]
				if got.ID.Value != int64(w.ID) {
					t.Errorf("IE %d = %d, want %d in wire order", i, got.ID.Value, w.ID)
				}

				if got.ID.Unknown {
					t.Errorf("IE %d rendered as unknown", got.ID.Value)
				}

				if _, raw := got.Value.(rawIEValue); raw {
					t.Errorf("IE %s rendered as unmodeled", got.ID.Label)
				}
			}
		})
	}
}

func TestTransferExtensionsRendered(t *testing.T) {
	msg := DecodeNGAPMessage(extensionIEsCorpus(t)["pdu_session_resource_setup_request_transfer_extensions"])

	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`"common_network_instance":{"hex":"696e"}`,
		`"redundant_pdu_session_information":{"rsn":{"type":"enum","value":1,"label":"v2","unknown":false}}`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("rendered transfer lacks %s:\n%s", want, b)
		}
	}
}
