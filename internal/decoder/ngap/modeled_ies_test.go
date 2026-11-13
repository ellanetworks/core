// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func modeledIEsCorpus(t *testing.T) map[string][]byte {
	t.Helper()

	guamis := lib.GUAMIList{
		{GUAMI: lib.GUAMI{PLMNIdentity: testPLMN, AMFRegionID: 0xca, AMFSetID: 0x3f8, AMFPointer: 0x01}},
	}

	var key lib.SecurityKey

	return map[string][]byte{
		"overload_start_guami_list": mustMarshal(t, &lib.OverloadStart{GUAMIList: guamis}),
		"overload_stop_guami_list":  mustMarshal(t, &lib.OverloadStop{GUAMIList: guamis}),
		"amf_configuration_update_plmn_support_list": mustMarshal(t, &lib.AMFConfigurationUpdate{
			PLMNSupportList: lib.PLMNSupportList{{
				PLMNIdentity:     testPLMN,
				SliceSupportList: lib.SliceSupportList{{SNSSAI: lib.SNSSAI{SST: 1}}},
			}},
		}),
		"initial_context_setup_request_trace_activation": mustMarshal(t, &lib.InitialContextSetupRequest{
			AMFUENGAPID:  1,
			RANUENGAPID:  2,
			GUAMI:        guamis[0].GUAMI,
			AllowedNSSAI: lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
			SecurityKey:  key,
			TraceActivation: &lib.TraceActivation{
				NGRANTraceID:                   lib.NGRANTraceID{0x02, 0xf8, 0x39, 0x0a, 0x0b, 0x0c, 0x12, 0x34},
				InterfacesToTrace:              lib.InterfaceToTraceNGC,
				TraceDepth:                     lib.TraceDepthMinimum,
				TraceCollectionEntityIPAddress: lib.TransportLayerAddress{10, 0, 0, 1},
			},
		}),
		"ue_context_modification_request_new_amf_ue_ngap_id": mustMarshal(t, &lib.UEContextModificationRequest{
			AMFUENGAPID:    1,
			RANUENGAPID:    2,
			NewAMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(3)),
		}),
		"ue_context_release_request_gw_context_release_indication": mustMarshal(t, &lib.UEContextReleaseRequest{
			AMFUENGAPID:                1,
			RANUENGAPID:                2,
			Cause:                      &lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: lib.CauseRadioNetworkUserInactivity},
			GWContextReleaseIndication: lib.Ptr(lib.GWContextReleaseIndicationTrue),
		}),
	}
}

func TestDecodeNGAPMessage_ModeledIEs(t *testing.T) {
	for name, raw := range modeledIEsCorpus(t) {
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

func TestDecodeNGAPMessage_OverloadStopGUAMIList(t *testing.T) {
	msg := DecodeNGAPMessage(modeledIEsCorpus(t)["overload_stop_guami_list"])

	if len(msg.Value.IEs) != 1 || msg.Value.IEs[0].ID.Value != int64(lib.IDGUAMIList) {
		t.Fatalf("IEs = %+v, want the GUAMI List alone", msg.Value.IEs)
	}

	list, ok := msg.Value.IEs[0].Value.([]Guami)
	if !ok || len(list) != 1 || list[0].AMFRegionID == "" {
		t.Fatalf("GUAMI List = %+v", msg.Value.IEs[0].Value)
	}
}
