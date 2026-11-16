// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/s1ap"
)

func extensionIEsCorpus(t *testing.T) map[string][]byte {
	t.Helper()

	gw := ap.TransportLayerAddressFromOctets([]byte{10, 0, 0, 9})
	bbf := &s1ap.TunnelInformation{TransportLayerAddress: ap.TransportLayerAddressFromOctets([]byte{10, 0, 0, 7}), UDPPortNumber: &s1ap.PortNumber{0x08, 0x68}}

	return map[string][]byte{
		"s1_removal_request": mustMarshal(t, &s1ap.S1RemovalRequest{
			GlobalENBID: s1ap.GlobalENBID{PLMNIdentity: testPLMN, ENBID: s1ap.ENBID{MacroENBID: s1ap.Ptr(s1ap.MacroENBID(0x12345))}},
		}),
		"s1_removal_response": mustMarshal(t, &s1ap.S1RemovalResponse{
			MMEName:                s1ap.Ptr(s1ap.Name("mme01")),
			CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{ProcedureCode: s1ap.Ptr(s1ap.ProcS1Removal)},
		}),
		"s1_removal_failure": mustMarshal(t, &s1ap.S1RemovalFailure{
			Cause:                  s1ap.Ptr(s1ap.CauseMisc(0).Cause()),
			CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{ProcedureCode: s1ap.Ptr(s1ap.ProcS1Removal)},
		}),
		"initial_ue_message_extensions": mustMarshal(t, &s1ap.InitialUEMessage{
			ENBUES1APID:                   2,
			NASPDU:                        s1ap.NASPDU{0x07, 0x41},
			TAI:                           testTAI,
			EUTRANCGI:                     &testCGI,
			RRCEstablishmentCause:         s1ap.Ptr(s1ap.RRCCauseMOSignalling),
			GWTransportLayerAddress:       &gw,
			TunnelInformationForBBF:       bbf,
			SIPTOLGWTransportLayerAddress: &gw,
			CoarseUELocationRequested:     s1ap.Ptr(s1ap.CoarseUELocationRequestedTrue),
			CSGId:                         s1ap.Ptr(s1ap.CSGId(0x123)),
			CellAccessMode:                s1ap.Ptr(s1ap.CellAccessModeHybrid),
			LHNID:                         s1ap.Ptr(s1ap.LHNID(make([]byte, 32))),
			DCNID:                         s1ap.Ptr(s1ap.DCNID(5)),
		}),
		"uplink_nas_transport_extensions": mustMarshal(t, &s1ap.UplinkNASTransport{
			MMEUES1APID:                   1,
			ENBUES1APID:                   2,
			NASPDU:                        s1ap.NASPDU{0x07, 0x41},
			EUTRANCGI:                     &testCGI,
			TAI:                           &testTAI,
			GWTransportLayerAddress:       &gw,
			SIPTOLGWTransportLayerAddress: &gw,
		}),
		"downlink_nas_transport_extensions": mustMarshal(t, &s1ap.DownlinkNASTransport{
			MMEUES1APID:      1,
			ENBUES1APID:      2,
			NASPDU:           s1ap.NASPDU{0x07, 0x41},
			CoarseUELocation: s1ap.Ptr(s1ap.CoarseUELocation{0x01, 0x02, 0x03}),
			NRUESecurityCapabilities: &s1ap.NRUESecurityCapabilities{
				NRencryptionAlgorithms:          s1ap.NRencryptionAlgorithms{BitString: s1ap.BitStringFrom([]byte{0xc0, 0x00}, 16)},
				NRintegrityProtectionAlgorithms: s1ap.NRintegrityProtectionAlgorithms{BitString: s1ap.BitStringFrom([]byte{0xc0, 0x00}, 16)},
			},
			SubscriberProfileIDforRFP: s1ap.Ptr(s1ap.SubscriberProfileIDforRFP(7)),
		}),
		"s1_setup_request_extensions": mustMarshal(t, &s1ap.S1SetupRequest{
			GlobalENBID:           s1ap.GlobalENBID{PLMNIdentity: testPLMN, ENBID: s1ap.ENBID{MacroENBID: s1ap.Ptr(s1ap.MacroENBID(0x12345))}},
			SupportedTAs:          s1ap.SupportedTAs{{TAC: 1, BroadcastPLMNs: []s1ap.PLMNIdentity{testPLMN}}},
			DefaultPagingDRX:      s1ap.Ptr(s1ap.PagingDRX(1)),
			CSGIdList:             s1ap.CSGIdList{{CSGId: 0x123}},
			NBIoTDefaultPagingDRX: s1ap.Ptr(s1ap.NBIoTDefaultPagingDRX(1)),
		}),
		"s1_setup_response_extensions": mustMarshal(t, &s1ap.S1SetupResponse{
			ServedGUMMEIs:            s1ap.ServedGUMMEIs{{ServedPLMNs: []s1ap.PLMNIdentity{testPLMN}, ServedGroupIDs: []s1ap.MMEGroupID{{0x00, 0x01}}, ServedMMECs: []s1ap.MMECode{1}}},
			RelativeMMECapacity:      s1ap.Ptr(uint8(10)),
			ServedDCNs:               &s1ap.ServedDCNs{{DCNID: 5, RelativeDCNCapacity: 10}},
			MMERelaySupportIndicator: s1ap.Ptr(s1ap.MMERelaySupportIndicator(0)),
		}),
		"paging_extensions": mustMarshal(t, &s1ap.Paging{
			UEIdentityIndexValue:      s1ap.Ptr(uint16(5)),
			UEPagingID:                &s1ap.UEPagingID{STMSI: &s1ap.STMSI{MMEC: 1, MTMSI: 0x01020304}},
			CNDomain:                  s1ap.Ptr(s1ap.CNDomainPS),
			TAIList:                   s1ap.TAIList{{TAI: testTAI}},
			PagingeDRXInformation:     &s1ap.PagingeDRXInformation{PagingeDRXCycle: s1ap.PagingeDRXCycleHf2},
			NBIoTUEIdentityIndexValue: s1ap.Ptr(s1ap.NBIoTUEIdentityIndexValue(0x0ab)),
		}),
		"location_report_extensions": mustMarshal(t, &s1ap.LocationReport{
			MMEUES1APID:       1,
			ENBUES1APID:       2,
			EUTRANCGI:         &testCGI,
			TAI:               &testTAI,
			RequestType:       &s1ap.RequestType{EventType: s1ap.EventTypeDirect, ReportArea: s1ap.ReportAreaECGI},
			PSCellInformation: &s1ap.PSCellInformation{NCGI: s1ap.NRCGI{PLMNIdentity: testPLMN, NRCellIdentity: 0x10}},
		}),
		"handover_notify_extensions": mustMarshal(t, &s1ap.HandoverNotify{
			MMEUES1APID:             1,
			ENBUES1APID:             2,
			EUTRANCGI:               &testCGI,
			TAI:                     &testTAI,
			TunnelInformationForBBF: bbf,
		}),
		"ue_context_modification_request_extensions": mustMarshal(t, &s1ap.UEContextModificationRequest{
			MMEUES1APID:         1,
			ENBUES1APID:         2,
			TimeRefDistribution: s1ap.Ptr(s1ap.TimeRefDistributionTrue),
		}),
	}
}

func TestExtensionIEsRendered(t *testing.T) {
	for name, raw := range extensionIEsCorpus(t) {
		t.Run(name, func(t *testing.T) {
			msg := DecodeS1APMessage(raw)
			if msg.Value.Error != "" {
				t.Fatalf("decode error: %s", msg.Value.Error)
			}

			pdu, err := s1ap.ParseRawPDU(raw)
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
