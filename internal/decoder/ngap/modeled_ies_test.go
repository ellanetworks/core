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
		"amf_configuration_update_tnl_and_extended_name": mustMarshal(t, &lib.AMFConfigurationUpdate{
			AMFTNLAssociationToAddList: lib.AMFTNLAssociationToAddList{{
				AMFTNLAssociationAddress: lib.CPTransportLayerInformation{EndpointIPAddressAndPort: &lib.EndpointIPAddressAndPort{
					EndpointIPAddress: lib.TransportLayerAddress{10, 0, 0, 3}, PortNumber: 38412,
				}},
				TNLAssociationUsage:    lib.Ptr(lib.TNLAssociationUsageBoth),
				TNLAddressWeightFactor: 1,
			}},
			AMFTNLAssociationToRemoveList: lib.AMFTNLAssociationToRemoveList{{
				AMFTNLAssociationAddress:                 lib.CPTransportLayerInformation{EndpointIPAddress: lib.TransportLayerAddress{10, 0, 0, 4}},
				TNLAssociationTransportLayerAddressNGRAN: &lib.CPTransportLayerInformation{EndpointIPAddress: lib.TransportLayerAddress{10, 0, 1, 4}},
			}},
			AMFTNLAssociationToUpdateList: lib.AMFTNLAssociationToUpdateList{{
				AMFTNLAssociationAddress: lib.CPTransportLayerInformation{EndpointIPAddress: lib.TransportLayerAddress{10, 0, 0, 2}},
			}},
			ExtendedAMFName: &lib.ExtendedAMFName{AMFNameUTF8String: lib.Ptr(lib.AMFNameUTF8String("ella"))},
		}),
		"initial_context_setup_request_amf_ue_ngap_id_2": mustMarshal(t, &lib.InitialContextSetupRequest{
			AMFUENGAPID:  1,
			RANUENGAPID:  2,
			GUAMI:        guamis[0].GUAMI,
			AllowedNSSAI: lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
			SecurityKey:  key,
			AMFUENGAPID2: lib.Ptr(lib.AMFUENGAPID(9)),
		}),
		"path_switch_request_acknowledge_amf_ue_ngap_id_2": mustMarshal(t, &lib.PathSwitchRequestAcknowledge{
			AMFUENGAPID:                    lib.Ptr(lib.AMFUENGAPID(1)),
			RANUENGAPID:                    lib.Ptr(lib.RANUENGAPID(2)),
			PDUSessionResourceSwitchedList: lib.PDUSessionResourceSwitchedList{{PDUSessionID: 5, Transfer: lib.TransferContainer{0x00}}},
			AllowedNSSAI:                   lib.AllowedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
			AMFUENGAPID2:                   lib.Ptr(lib.AMFUENGAPID(9)),
		}),
		"initial_ue_message_guami_type_requested_nssai": mustMarshal(t, &lib.InitialUEMessage{
			RANUENGAPID:             1,
			NASPDU:                  lib.NASPDU{0x7e, 0x00},
			UserLocationInformation: lib.UserLocationInformation{Kind: lib.UserLocationNR, PLMNIdentity: testPLMN, TAI: testTAI},
			RRCEstablishmentCause:   lib.Ptr(lib.RRCCauseMOSignalling),
			GUAMI:                   &guamis[0].GUAMI,
			GUAMIType:               lib.Ptr(lib.GUAMITypeMapped),
			RequestedNSSAI:          lib.RequestedNSSAI{{SNSSAI: lib.SNSSAI{SST: 1}}},
		}),
		"ue_context_modification_request_new_amf_ue_ngap_id": mustMarshal(t, &lib.UEContextModificationRequest{
			AMFUENGAPID:    1,
			RANUENGAPID:    2,
			NewAMFUENGAPID: lib.Ptr(lib.AMFUENGAPID(3)),
		}),
		"amf_configuration_update_acknowledge_tnl_lists": mustMarshal(t, &lib.AMFConfigurationUpdateAcknowledge{
			AMFTNLAssociationSetupList: lib.AMFTNLAssociationSetupList{{
				AMFTNLAssociationAddress: lib.CPTransportLayerInformation{EndpointIPAddress: lib.TransportLayerAddress{10, 0, 0, 2}},
			}},
			AMFTNLAssociationFailedToSetupList: lib.TNLAssociationList{{
				TNLAssociationAddress: lib.CPTransportLayerInformation{EndpointIPAddress: lib.TransportLayerAddress{10, 0, 0, 5}},
				Cause:                 lib.Cause{Group: lib.CauseGroupTransport, Value: lib.CauseTransportResourceUnavailable},
			}},
		}),
		"amf_configuration_update_served_guami_extensions": mustMarshal(t, &lib.AMFConfigurationUpdate{
			ServedGUAMIList: lib.ServedGUAMIList{{
				GUAMI:                 guamis[0].GUAMI,
				BackupAMFName:         lib.Ptr(lib.Name("backup")),
				GUAMIType:             lib.Ptr(lib.GUAMITypeMapped),
				ExtendedBackupAMFName: &lib.ExtendedAMFName{AMFNameVisibleString: lib.Ptr(lib.AMFNameVisibleString("backup-extended"))},
			}},
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

func TestDecodeNGAPMessage_AMFConfigurationUpdateAcknowledgeTNLLists(t *testing.T) {
	msg := DecodeNGAPMessage(modeledIEsCorpus(t)["amf_configuration_update_acknowledge_tnl_lists"])

	if len(msg.Value.IEs) != 2 {
		t.Fatalf("IEs = %+v, want the setup and failed-to-setup lists", msg.Value.IEs)
	}

	setup, ok := msg.Value.IEs[0].Value.([]AMFTNLAssociation)
	if !ok || msg.Value.IEs[0].ID.Value != int64(lib.IDAMFTNLAssociationSetupList) || len(setup) != 1 || setup[0].Address != "10.0.0.2" {
		t.Fatalf("setup list = %+v", msg.Value.IEs[0])
	}

	failed, ok := msg.Value.IEs[1].Value.([]TNLAssociation)
	if !ok || msg.Value.IEs[1].ID.Value != int64(lib.IDAMFTNLAssociationFailedToSetupList) || len(failed) != 1 || failed[0].Address != "10.0.0.5" {
		t.Fatalf("failed list = %+v", msg.Value.IEs[1])
	}

	if failed[0].Cause != cause(lib.Cause{Group: lib.CauseGroupTransport, Value: lib.CauseTransportResourceUnavailable}) {
		t.Fatalf("cause = %+v", failed[0].Cause)
	}
}

func TestDecodeNGAPMessage_ServedGUAMIExtensions(t *testing.T) {
	msg := DecodeNGAPMessage(modeledIEsCorpus(t)["amf_configuration_update_served_guami_extensions"])

	list, ok := msg.Value.IEs[0].Value.([]ServedGUAMI)
	if !ok || len(list) != 1 {
		t.Fatalf("Served GUAMI List = %+v", msg.Value.IEs[0].Value)
	}

	got := list[0]
	if got.AMFRegionID == "" || got.BackupAMFName == nil || *got.BackupAMFName != "backup" {
		t.Fatalf("served GUAMI = %+v", got)
	}

	if got.GUAMIType == nil || got.GUAMIType.Value != int64(lib.GUAMITypeMapped) {
		t.Fatalf("GUAMI Type = %+v", got.GUAMIType)
	}

	if got.ExtendedBackupAMFName == nil || got.ExtendedBackupAMFName.VisibleString == nil || *got.ExtendedBackupAMFName.VisibleString != "backup-extended" {
		t.Fatalf("Extended Backup AMF Name = %+v", got.ExtendedBackupAMFName)
	}
}
