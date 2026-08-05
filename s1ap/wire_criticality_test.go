// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

// The {id, criticality} pairs a body puts on the wire. The message parsers
// discard criticality, so this is the only way to assert what a peer receives.
func wireIEs(t *testing.T, body func(*per.Writer, per.Encoding) error) []rawIE {
	t.Helper()

	w := per.NewWriter()
	if err := body(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	r := per.NewReader(w.Bytes())

	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	return fields
}

// TestWireCriticality pins each stamped criticality against TS 36.413 §9.1.
// A round-trip test cannot: encode and decode agree with each other while both
// disagree with the spec.
func TestWireCriticality(t *testing.T) {
	type wireIE struct {
		id   ProtocolIEID
		crit Criticality
	}

	cause := Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 0})

	tests := []struct {
		name string
		body func(*per.Writer, per.Encoding) error
		want []wireIE // {id, criticality} in wire order
	}{
		{
			"HandoverCancel §9.1.5.11",
			(&HandoverCancel{Cause: cause}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"HandoverPreparationFailure §9.1.5.3",
			(&HandoverPreparationFailure{
				MMEUES1APID: Ptr(MMEUES1APID(1)), ENBUES1APID: Ptr(ENBUES1APID(2)), Cause: cause,
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"PathSwitchRequestFailure §9.1.5.10",
			(&PathSwitchRequestFailure{
				MMEUES1APID: Ptr(MMEUES1APID(1)), ENBUES1APID: Ptr(ENBUES1APID(2)), Cause: cause,
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"UplinkNASTransport §9.1.7.3",
			(&UplinkNASTransport{
				NASPDU:    NASPDU{0x07},
				EUTRANCGI: Ptr(EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}}),
				TAI:       Ptr(TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7}),
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idNASPDU, CriticalityReject},
				{idEUTRANCGI, CriticalityIgnore},
				{idTAI, CriticalityIgnore},
			},
		},
		{
			"UECapabilityInfoIndication §9.1.10",
			(&UECapabilityInfoIndication{
				MMEUES1APID:                1,
				ENBUES1APID:                2,
				UERadioCapability:          UERadioCapability{0x01, 0x02, 0x03},
				UERadioCapabilityForPaging: UERadioCapabilityForPaging{0x0a},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idUERadioCapability, CriticalityIgnore},
				{idUERadioCapabilityForPaging, CriticalityIgnore},
			},
		},
		{
			"ERABModificationIndication §9.1.3.8",
			(&ERABModificationIndication{
				MMEUES1APID:  1,
				ENBUES1APID:  2,
				ToBeModified: []ERABToBeModifiedItemBearerModInd{{ERABID: 1, TransportLayerAddress: goldTLA(), DLGTPTEID: 1}},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idERABToBeModifiedListBearerModInd, CriticalityReject},
			},
		},
		{
			"ERABModificationConfirm §9.1.3.9",
			(&ERABModificationConfirm{
				MMEUES1APID: Ptr(MMEUES1APID(1)),
				ENBUES1APID: Ptr(ENBUES1APID(2)),
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
			},
		},
		{
			"ERABModifyRequest §9.1.3.3",
			(&ERABModifyRequest{
				MMEUES1APID:               1,
				ENBUES1APID:               2,
				UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{},
				ERABToBeModified:          []ERABToBeModifiedItemBearerModReq{{ERABID: 1, QoS: goldQoS(), NASPDU: NASPDU{0x07}}},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idUEAggregateMaximumBitrate, CriticalityReject},
				{idERABToBeModifiedListBearerModReq, CriticalityReject},
			},
		},
		{
			"ERABModifyResponse §9.1.3.4",
			(&ERABModifyResponse{
				MMEUES1APID:            Ptr(MMEUES1APID(1)),
				ENBUES1APID:            Ptr(ENBUES1APID(2)),
				ERABModify:             []ERABModifyItemBearerModRes{{ERABID: 1}},
				ERABFailedToModify:     []ERABItem{goldERABItem()},
				CriticalityDiagnostics: &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idERABModifyListBearerModRes, CriticalityIgnore},
				{idERABFailedToModifyList, CriticalityIgnore},
				{idCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"ERABSetupRequest §9.1.3.1",
			(&ERABSetupRequest{
				MMEUES1APID:               1,
				ENBUES1APID:               2,
				UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{},
				ERABToBeSetup:             []ERABToBeSetupItemBearerSUReq{{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1}},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idUEAggregateMaximumBitrate, CriticalityReject},
				{idERABToBeSetupListBearerSUReq, CriticalityReject},
			},
		},
		{
			"ERABSetupResponse §9.1.3.2",
			(&ERABSetupResponse{
				MMEUES1APID:            Ptr(MMEUES1APID(1)),
				ENBUES1APID:            Ptr(ENBUES1APID(2)),
				ERABSetup:              []ERABSetupItemBearerSURes{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1}},
				ERABFailedToSetup:      []ERABItem{goldERABItem()},
				CriticalityDiagnostics: &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idERABSetupListBearerSURes, CriticalityIgnore},
				{idERABFailedToSetupListBearerSURes, CriticalityIgnore},
				{idCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"ERABReleaseCommand §9.1.3.5",
			(&ERABReleaseCommand{
				MMEUES1APID:      1,
				ENBUES1APID:      2,
				ERABToBeReleased: []ERABItem{goldERABItem()},
				NASPDU:           NASPDU{0x07},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idERABToBeReleasedList, CriticalityIgnore},
				{idNASPDU, CriticalityIgnore},
			},
		},
		{
			"ERABReleaseResponse §9.1.3.6",
			(&ERABReleaseResponse{
				MMEUES1APID:         Ptr(MMEUES1APID(1)),
				ENBUES1APID:         Ptr(ENBUES1APID(2)),
				ERABReleased:        []ERABReleaseItemBearerRelComp{{ERABID: 1}},
				ERABFailedToRelease: []ERABItem{goldERABItem()},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idERABReleaseListBearerRelComp, CriticalityIgnore},
				{idERABFailedToReleaseList, CriticalityIgnore},
			},
		},
		{
			"InitialContextSetupRequest §9.1.4.1",
			(&InitialContextSetupRequest{
				MMEUES1APID:       1,
				ENBUES1APID:       2,
				ERABToBeSetup:     []ERABToBeSetupItemCtxtSUReq{{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1}},
				UERadioCapability: UERadioCapability{0x01},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idUEAggregateMaximumBitrate, CriticalityReject},
				{idERABToBeSetupListCtxtSUReq, CriticalityReject},
				{idUESecurityCapabilities, CriticalityReject},
				{idSecurityKey, CriticalityReject},
				{idUERadioCapability, CriticalityIgnore},
			},
		},
		{
			"InitialContextSetupResponse §9.1.4.3",
			(&InitialContextSetupResponse{
				MMEUES1APID:            Ptr(MMEUES1APID(1)),
				ENBUES1APID:            Ptr(ENBUES1APID(2)),
				ERABSetup:              []ERABSetupItemCtxtSURes{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1}},
				ERABFailedToSetup:      []ERABItem{goldERABItem()},
				CriticalityDiagnostics: &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idERABSetupListCtxtSURes, CriticalityIgnore},
				{idERABFailedToSetupListCtxtSU, CriticalityIgnore},
				{idCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"InitialContextSetupFailure §9.1.4.4",
			(&InitialContextSetupFailure{
				MMEUES1APID:            Ptr(MMEUES1APID(1)),
				ENBUES1APID:            Ptr(ENBUES1APID(2)),
				Cause:                  cause,
				CriticalityDiagnostics: &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idCause, CriticalityIgnore},
				{idCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"UEContextReleaseRequest §9.1.4.5",
			(&UEContextReleaseRequest{
				MMEUES1APID: 1,
				ENBUES1APID: 2,
				Cause:       Ptr(Cause{Group: CauseGroupRadioNetwork, Value: 0}),
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"UEContextReleaseCommand §9.1.4.6",
			(&UEContextReleaseCommand{
				UES1APIDs: UES1APIDs{MMEUES1APID: 1, ENBUES1APID: 2, Pair: true},
				Cause:     Ptr(Cause{Group: CauseGroupNAS, Value: 0}),
			}).encodeBody,
			[]wireIE{
				{idUES1APIDs, CriticalityReject},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"UEContextReleaseComplete §9.1.4.7",
			(&UEContextReleaseComplete{
				MMEUES1APID:             Ptr(MMEUES1APID(1)),
				ENBUES1APID:             Ptr(ENBUES1APID(2)),
				CriticalityDiagnostics:  &CriticalityDiagnostics{},
				UserLocationInformation: &UserLocationInformation{},
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idCriticalityDiagnostics, CriticalityIgnore},
				{idUserLocationInformation, CriticalityIgnore},
			},
		},
		{
			"InitialUEMessage §9.1.7.1",
			(&InitialUEMessage{
				ENBUES1APID:           1,
				NASPDU:                NASPDU{0x07},
				TAI:                   TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
				EUTRANCGI:             Ptr(EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}}),
				RRCEstablishmentCause: Ptr(RRCCauseEmergency),
				STMSI:                 &STMSI{},
				GUMMEI:                &GUMMEI{},
			}).encodeBody,
			[]wireIE{
				{idENBUES1APID, CriticalityReject},
				{idNASPDU, CriticalityReject},
				{idTAI, CriticalityReject},
				{idEUTRANCGI, CriticalityIgnore},
				{idRRCEstablishmentCause, CriticalityIgnore},
				{idSTMSI, CriticalityReject},
				{idGUMMEI, CriticalityReject},
			},
		},
		{
			"DownlinkNASTransport §9.1.7.2",
			(&DownlinkNASTransport{MMEUES1APID: 1, ENBUES1APID: 2, NASPDU: NASPDU{0x07}}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idNASPDU, CriticalityReject},
			},
		},
		{
			"NASNonDeliveryIndication §9.1.7.4",
			(&NASNonDeliveryIndication{
				MMEUES1APID: 1,
				ENBUES1APID: 2,
				NASPDU:      NASPDU{0x07},
				Cause:       Ptr(Cause{Group: CauseGroupMisc, Value: CauseMiscUnspecified}),
			}).encodeBody,
			[]wireIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idNASPDU, CriticalityIgnore},
				{idCause, CriticalityIgnore},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wireIEs(t, tt.body)
			if len(got) != len(tt.want) {
				t.Fatalf("encoded %d IEs, want %d", len(got), len(tt.want))
			}

			for i, w := range tt.want {
				if got[i].id != w.id || got[i].crit != w.crit {
					t.Errorf("IE %d: got %v/%v, want %v/%v", i, got[i].id, got[i].crit, w.id, w.crit)
				}
			}
		})
	}
}
