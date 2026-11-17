// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/per/ap"
)

func tunnel(teid uint32) lib.UPTransportLayerInformation {
	return lib.UPTransportLayerInformation{GTPTunnel: &lib.GTPTunnel{
		TransportLayerAddress: ap.TransportLayerAddressFromOctets([]byte{10, 0, 0, 1}),
		GTPTEID:               lib.GTPTEID(teid),
	}}
}

func tunnelPtr(teid uint32) *lib.UPTransportLayerInformation {
	u := tunnel(teid)
	return &u
}

func flowTunnel(teid uint32) lib.QosFlowPerTNLInformation {
	return lib.QosFlowPerTNLInformation{UPTransportLayerInformation: tunnel(teid), AssociatedQosFlowList: lib.AssociatedQosFlowList{{QosFlowIdentifier: 1}}}
}

func flowTunnels(teid uint32) lib.QosFlowPerTNLInformationList {
	return lib.QosFlowPerTNLInformationList{{QosFlowPerTNLInformation: flowTunnel(teid)}}
}

func tunnels(teid uint32) lib.UPTransportLayerInformationList {
	return lib.UPTransportLayerInformationList{{NGUUPTNLInformation: tunnel(teid)}}
}

func tunnelPairs(ul, dl uint32) lib.UPTransportLayerInformationPairList {
	return lib.UPTransportLayerInformationPairList{{ULNGUUPTNLInformation: tunnel(ul), DLNGUUPTNLInformation: tunnel(dl)}}
}

func mustTransfer(t *testing.T, m interface {
	Marshal() (lib.TransferContainer, error)
},
) lib.TransferContainer {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

type tnlRenderCase struct {
	name  string
	raw   []byte
	keys  []string
	teids []uint32
}

func tnlRenderCases(t *testing.T) []tnlRenderCase {
	t.Helper()

	redundantFlow := flowTunnel(0x203)

	setupReq := mustTransfer(t, &lib.PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation: tunnel(0x101),
		PDUSessionType:        lib.PDUSessionTypeIPv4,
		QosFlowSetupRequest: lib.QosFlowSetupRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: lib.QosFlowLevelQosParameters{
				QosCharacteristics:             lib.QosCharacteristics{NonDynamic5QI: &lib.NonDynamic5QIDescriptor{FiveQI: 9}},
				AllocationAndRetentionPriority: lib.AllocationAndRetentionPriority{PriorityLevelARP: 8},
			},
		}},
		RedundantULNGUUPTNLInformation:           tunnelPtr(0x102),
		AdditionalRedundantULNGUUPTNLInformation: tunnels(0x103),
	})

	setupRes := mustTransfer(t, &lib.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation:                    flowTunnel(0x201),
		AdditionalDLQosFlowPerTNLInformation:          flowTunnels(0x202),
		RedundantDLQosFlowPerTNLInformation:           &redundantFlow,
		AdditionalRedundantDLQosFlowPerTNLInformation: flowTunnels(0x204),
	})

	modReq := mustTransfer(t, &lib.PDUSessionResourceModifyRequestTransfer{
		ULNGUUPTNLModify: lib.ULNGUUPTNLModifyList{{
			ULNGUUPTNLInformation:          tunnel(0x301),
			DLNGUUPTNLInformation:          tunnel(0x302),
			RedundantULNGUUPTNLInformation: tunnelPtr(0x303),
			RedundantDLNGUUPTNLInformation: tunnelPtr(0x304),
		}},
		QosFlowAddOrModifyRequest:                lib.QosFlowAddOrModifyRequestList{{QosFlowIdentifier: 1, ULNGUUPTNLInformation: tunnelPtr(0x305)}},
		AdditionalRedundantULNGUUPTNLInformation: tunnels(0x306),
		RedundantULNGUUPTNLInformation:           tunnelPtr(0x307),
		UserPlaneFailureIndication: &lib.UserPlaneFailureIndication{
			UserPlaneFailureType:  lib.UserPlaneFailureTypeUPPathFailure,
			ULNGUUPTNLInformation: tunnel(0x308),
			DLNGUUPTNLInformation: tunnel(0x309),
		},
	})

	modRes := mustTransfer(t, &lib.PDUSessionResourceModifyResponseTransfer{
		QosFlowAddOrModifyResponse:                    lib.QosFlowAddOrModifyResponseList{{QosFlowIdentifier: 1, DLNGUUPTNLInformation: tunnelPtr(0x401)}},
		AdditionalNGUUPTNLInformation:                 tunnelPairs(0x402, 0x403),
		RedundantDLNGUUPTNLInformation:                tunnelPtr(0x404),
		RedundantULNGUUPTNLInformation:                tunnelPtr(0x405),
		AdditionalRedundantDLQosFlowPerTNLInformation: flowTunnels(0x406),
		AdditionalRedundantNGUUPTNLInformation:        tunnelPairs(0x407, 0x408),
	})

	redundantInd := flowTunnel(0x502)

	modInd := mustTransfer(t, &lib.PDUSessionResourceModifyIndicationTransfer{
		DLQosFlowPerTNLInformation:                    flowTunnel(0x501),
		RedundantDLQosFlowPerTNLInformation:           &redundantInd,
		AdditionalRedundantDLQosFlowPerTNLInformation: flowTunnels(0x503),
	})

	modCfm := mustTransfer(t, &lib.PDUSessionResourceModifyConfirmTransfer{
		QosFlowModifyConfirm:                   lib.QosFlowModifyConfirmList{{QosFlowIdentifier: 1}},
		ULNGUUPTNLInformation:                  tunnel(0x601),
		RedundantULNGUUPTNLInformation:         tunnelPtr(0x602),
		AdditionalRedundantNGUUPTNLInformation: tunnelPairs(0x603, 0x604),
	})

	psReq := mustTransfer(t, &lib.PathSwitchRequestTransfer{
		DLNGUUPTNLInformation:                         tunnel(0x701),
		QosFlowAccepted:                               lib.QosFlowAcceptedList{{QosFlowIdentifier: 1}},
		AdditionalDLQosFlowPerTNLInformation:          flowTunnels(0x702),
		RedundantDLNGUUPTNLInformation:                tunnelPtr(0x703),
		RedundantDLNGUTNLInformationReused:            lib.Ptr(lib.DLNGUTNLInformationReusedTrue),
		AdditionalRedundantDLQosFlowPerTNLInformation: flowTunnels(0x704),
	})

	psAck := mustTransfer(t, &lib.PathSwitchRequestAcknowledgeTransfer{
		AdditionalNGUUPTNLInformation:          tunnelPairs(0x801, 0x802),
		RedundantULNGUUPTNLInformation:         tunnelPtr(0x803),
		AdditionalRedundantNGUUPTNLInformation: tunnelPairs(0x804, 0x805),
	})

	hoAck := mustTransfer(t, &lib.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: tunnel(0x901),
		QosFlowSetupResponse:  lib.QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
		AdditionalDLUPTNLInformationForHO: lib.AdditionalDLUPTNLInformationForHOList{{
			AdditionalDLNGUUPTNLInformation:          tunnel(0x902),
			AdditionalQosFlowSetupResponse:           lib.QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
			AdditionalDLForwardingUPTNLInformation:   tunnelPtr(0x903),
			AdditionalRedundantDLNGUUPTNLInformation: tunnelPtr(0x904),
		}},
		ULForwardingUPTNLInformation:           tunnelPtr(0x905),
		AdditionalULForwardingUPTNLInformation: tunnels(0x906),
		DataForwardingResponseERAB:             lib.DataForwardingResponseERABList{{ERABID: 5, DLForwardingUPTNLInformation: tunnel(0x907)}},
		RedundantDLNGUUPTNLInformation:         tunnelPtr(0x908),
	})

	hoCmd := mustTransfer(t, &lib.HandoverCommandTransfer{
		AdditionalDLForwardingUPTNLInformation: flowTunnels(0xa01),
		ULForwardingUPTNLInformation:           tunnelPtr(0xa02),
		AdditionalULForwardingUPTNLInformation: tunnels(0xa03),
		DataForwardingResponseERAB:             lib.DataForwardingResponseERABList{{ERABID: 5, DLForwardingUPTNLInformation: tunnel(0xa04)}},
	})

	return []tnlRenderCase{
		{
			name: "setup_request",
			raw: mustMarshal(t, &lib.PDUSessionResourceSetupRequest{
				AMFUENGAPID: 1, RANUENGAPID: 2,
				PDUSessionResourceSetup: lib.PDUSessionResourceSetupListSUReq{{PDUSessionID: 5, SNSSAI: lib.SNSSAI{SST: 1}, Transfer: setupReq}},
			}),
			keys:  []string{"redundant_ul_ng_u_up_tnl_information", "additional_redundant_ul_ng_u_up_tnl_information"},
			teids: []uint32{0x101, 0x102, 0x103},
		},
		{
			name: "setup_response",
			raw: mustMarshal(t, &lib.PDUSessionResourceSetupResponse{
				AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(1)), RANUENGAPID: lib.Ptr(lib.RANUENGAPID(2)),
				PDUSessionResourceSetup: lib.PDUSessionResourceSetupListSURes{{PDUSessionID: 5, Transfer: setupRes}},
			}),
			keys:  []string{"additional_dl_qos_flow_per_tnl_information", "redundant_dl_qos_flow_per_tnl_information", "additional_redundant_dl_qos_flow_per_tnl_information"},
			teids: []uint32{0x201, 0x202, 0x203, 0x204},
		},
		{
			name: "modify_request",
			raw: mustMarshal(t, &lib.PDUSessionResourceModifyRequest{
				AMFUENGAPID: 1, RANUENGAPID: 2,
				PDUSessionResourceModify: lib.PDUSessionResourceModifyListModReq{{PDUSessionID: 5, Transfer: modReq, SNSSAI: &lib.SNSSAI{SST: 1}}},
			}),
			keys: []string{
				"snssai",
				"redundant_ul_ng_u_up_tnl_information", "redundant_dl_ng_u_up_tnl_information",
				"additional_redundant_ul_ng_u_up_tnl_information", "user_plane_failure_indication", "up-path-failure",
			},
			teids: []uint32{0x301, 0x302, 0x303, 0x304, 0x305, 0x306, 0x307, 0x308, 0x309},
		},
		{
			name: "modify_response",
			raw: mustMarshal(t, &lib.PDUSessionResourceModifyResponse{
				AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(1)), RANUENGAPID: lib.Ptr(lib.RANUENGAPID(2)),
				PDUSessionResourceModify: lib.PDUSessionResourceModifyListModRes{{PDUSessionID: 5, Transfer: modRes}},
			}),
			keys: []string{
				"additional_ng_u_up_tnl_information", "redundant_dl_ng_u_up_tnl_information", "redundant_ul_ng_u_up_tnl_information",
				"additional_redundant_dl_qos_flow_per_tnl_information", "additional_redundant_ng_u_up_tnl_information",
			},
			teids: []uint32{0x401, 0x402, 0x403, 0x404, 0x405, 0x406, 0x407, 0x408},
		},
		{
			name: "modify_indication",
			raw: mustMarshal(t, &lib.PDUSessionResourceModifyIndication{
				AMFUENGAPID: 1, RANUENGAPID: 2,
				PDUSessionResourceModify: lib.PDUSessionResourceModifyListModInd{{PDUSessionID: 5, Transfer: modInd}},
			}),
			keys:  []string{"redundant_dl_qos_flow_per_tnl_information", "additional_redundant_dl_qos_flow_per_tnl_information"},
			teids: []uint32{0x501, 0x502, 0x503},
		},
		{
			name: "modify_confirm",
			raw: mustMarshal(t, &lib.PDUSessionResourceModifyConfirm{
				AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(1)), RANUENGAPID: lib.Ptr(lib.RANUENGAPID(2)),
				PDUSessionResourceModify: lib.PDUSessionResourceModifyListModCfm{{PDUSessionID: 5, Transfer: modCfm}},
			}),
			keys:  []string{"redundant_ul_ng_u_up_tnl_information", "additional_redundant_ng_u_up_tnl_information"},
			teids: []uint32{0x601, 0x602, 0x603, 0x604},
		},
		{
			name: "path_switch_request",
			raw: mustMarshal(t, &lib.PathSwitchRequest{
				RANUENGAPID: 2, SourceAMFUENGAPID: 1,
				UserLocationInformation:              &lib.UserLocationInformation{NR: &lib.UserLocationInformationNR{NRCGI: lib.NRCGI{PLMNIdentity: testPLMN, NRCellIdentity: lib.NRCellIdentity(0)}, TAI: testTAI}},
				UESecurityCapabilities:               &lib.UESecurityCapabilities{},
				PDUSessionResourceToBeSwitchedDLList: lib.PDUSessionResourceToBeSwitchedDLList{{PDUSessionID: 5, Transfer: psReq}},
			}),
			keys: []string{
				"additional_dl_qos_flow_per_tnl_information", "redundant_dl_ng_u_up_tnl_information",
				"redundant_dl_ng_u_tnl_information_reused", "additional_redundant_dl_qos_flow_per_tnl_information",
			},
			teids: []uint32{0x701, 0x702, 0x703, 0x704},
		},
		{
			name: "path_switch_request_acknowledge",
			raw: mustMarshal(t, &lib.PathSwitchRequestAcknowledge{
				AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(1)), RANUENGAPID: lib.Ptr(lib.RANUENGAPID(2)),
				PDUSessionResourceSwitchedList: lib.PDUSessionResourceSwitchedList{{PDUSessionID: 5, Transfer: psAck}},
				AllowedNSSAI:                   lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
			}),
			keys:  []string{"additional_ng_u_up_tnl_information", "redundant_ul_ng_u_up_tnl_information", "additional_redundant_ng_u_up_tnl_information"},
			teids: []uint32{0x801, 0x802, 0x803, 0x804, 0x805},
		},
		{
			name: "handover_request_acknowledge",
			raw: mustMarshal(t, &lib.HandoverRequestAcknowledge{
				AMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(1)), RANUENGAPID: lib.Ptr(lib.RANUENGAPID(2)),
				PDUSessionResourceAdmittedList:     lib.PDUSessionResourceAdmittedList{{PDUSessionID: 5, Transfer: hoAck}},
				TargetToSourceTransparentContainer: lib.TargetToSourceTransparentContainer{0x00, 0x00},
			}),
			keys: []string{
				"additional_dl_up_tnl_information_for_ho", "additional_redundant_dl_ng_u_up_tnl_information", "ul_forwarding_up_tnl_information",
				"additional_ul_forwarding_up_tnl_information", "data_forwarding_response_erab", "redundant_dl_ng_u_up_tnl_information",
			},
			teids: []uint32{0x901, 0x902, 0x903, 0x904, 0x905, 0x906, 0x907, 0x908},
		},
		{
			name: "handover_command",
			raw: mustMarshal(t, &lib.HandoverCommand{
				AMFUENGAPID: 1, RANUENGAPID: 2, HandoverType: lib.HandoverTypeIntra5GS,
				PDUSessionResourceHandoverList:     lib.PDUSessionResourceHandoverList{{PDUSessionID: 5, Transfer: hoCmd}},
				TargetToSourceTransparentContainer: lib.TargetToSourceTransparentContainer{0x00, 0x00},
			}),
			keys: []string{
				"additional_dl_forwarding_up_tnl_information", "ul_forwarding_up_tnl_information",
				"additional_ul_forwarding_up_tnl_information", "data_forwarding_response_erab",
			},
			teids: []uint32{0xa01, 0xa02, 0xa03, 0xa04},
		},
	}
}

func TestDecodeNGAPMessage_TNLExtensions(t *testing.T) {
	for _, c := range tnlRenderCases(t) {
		t.Run(c.name, func(t *testing.T) {
			msg := DecodeNGAPMessage(c.raw)
			if msg.Value.Error != "" {
				t.Fatalf("decode error: %s", msg.Value.Error)
			}

			out, err := json.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}

			s := string(out)

			if strings.Contains(s, `"error"`) || strings.Contains(s, "transfer_hex") {
				t.Fatalf("a transfer did not decode: %s", s)
			}

			for _, k := range c.keys {
				if !strings.Contains(s, k) {
					t.Errorf("rendering lacks %q: %s", k, s)
				}
			}

			for _, teid := range c.teids {
				if !strings.Contains(s, fmt.Sprintf(`"gtp_teid":%d,`, teid)) {
					t.Errorf("rendering lacks GTP-TEID %#x: %s", teid, s)
				}
			}
		})
	}
}
