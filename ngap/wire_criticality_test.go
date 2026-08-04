// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

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

// TestWireCriticality pins each stamped criticality against TS 38.413 §9.2.
// A round-trip test cannot: encode and decode agree with each other while both
// disagree with the spec.
func TestWireCriticality(t *testing.T) {
	type wireIE struct {
		id   ProtocolIEID
		crit Criticality
	}

	tests := []struct {
		name string
		body func(*per.Writer, per.Encoding) error
		want []wireIE // {id, criticality} in wire order
	}{
		{
			"NGSetupRequest §9.2.6.1",
			goldRequest().encodeBody,
			[]wireIE{
				{idGlobalRANNodeID, CriticalityReject},
				{idRANNodeName, CriticalityIgnore},
				{idSupportedTAList, CriticalityReject},
				{idDefaultPagingDRX, CriticalityIgnore},
			},
		},
		{
			"NGSetupResponse §9.2.6.2",
			goldResponse().encodeBody,
			[]wireIE{
				{idAMFName, CriticalityReject},
				{idServedGUAMIList, CriticalityReject},
				{idRelativeAMFCapacity, CriticalityIgnore},
				{idPLMNSupportList, CriticalityReject},
			},
		},
		{
			"NGSetupFailure §9.2.6.3",
			(&NGSetupFailure{
				Cause:                  Ptr(Cause{Group: CauseGroupMisc, Value: CauseMiscUnspecified}),
				TimeToWait:             Ptr(TimeToWaitV1s),
				CriticalityDiagnostics: &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{idCause, CriticalityIgnore},
				{idTimeToWait, CriticalityIgnore},
				{idCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"InitialUEMessage §9.2.5.1",
			(&InitialUEMessage{
				RANUENGAPID:             1,
				NASPDU:                  NASPDU{0x7e},
				UserLocationInformation: UserLocationInformation{Kind: UserLocationNR},
				RRCEstablishmentCause:   Ptr(RRCCauseEmergency),
				FiveGSTMSI:              &FiveGSTMSI{},
				AMFSetID:                Ptr(AMFSetID(1)),
				UEContextRequest:        Ptr(UEContextRequested),
				AllowedNSSAI:            AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
			}).encodeBody,
			[]wireIE{
				{idRANUENGAPID, CriticalityReject},
				{idNASPDU, CriticalityReject},
				{idUserLocationInformation, CriticalityReject},
				{idRRCEstablishmentCause, CriticalityIgnore},
				{idFiveGSTMSI, CriticalityReject},
				{idAMFSetID, CriticalityIgnore},
				{idUEContextRequest, CriticalityIgnore},
				{idAllowedNSSAI, CriticalityReject},
			},
		},
		{
			"DownlinkNASTransport §9.2.5.2",
			(&DownlinkNASTransport{AMFUENGAPID: 1, RANUENGAPID: 2, NASPDU: NASPDU{0x7e}}).encodeBody,
			[]wireIE{
				{idAMFUENGAPID, CriticalityReject},
				{idRANUENGAPID, CriticalityReject},
				{idNASPDU, CriticalityReject},
			},
		},
		{
			"UplinkNASTransport §9.2.5.3",
			(&UplinkNASTransport{
				AMFUENGAPID:             1,
				RANUENGAPID:             2,
				NASPDU:                  NASPDU{0x7e},
				UserLocationInformation: &UserLocationInformation{Kind: UserLocationNR},
			}).encodeBody,
			[]wireIE{
				{idAMFUENGAPID, CriticalityReject},
				{idRANUENGAPID, CriticalityReject},
				{idNASPDU, CriticalityReject},
				{idUserLocationInformation, CriticalityIgnore},
			},
		},
		{
			"UEContextReleaseRequest §9.2.2.4",
			(&UEContextReleaseRequest{
				AMFUENGAPID:            1,
				RANUENGAPID:            2,
				PDUSessionResourceList: PDUSessionResourceListCxtRelReq{{PDUSessionID: 5}},
				Cause:                  Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUserInactivity}),
			}).encodeBody,
			[]wireIE{
				{idAMFUENGAPID, CriticalityReject},
				{idRANUENGAPID, CriticalityReject},
				{idPDUSessionResourceListCxtRelReq, CriticalityReject},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"UEContextReleaseCommand §9.2.2.5",
			(&UEContextReleaseCommand{
				UENGAPIDs: UENGAPIDs{AMFUENGAPID: 1, RANUENGAPID: 2, Pair: true},
				Cause:     Ptr(Cause{Group: CauseGroupNAS, Value: CauseNASNormalRelease}),
			}).encodeBody,
			[]wireIE{
				{idUENGAPIDs, CriticalityReject},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"UEContextReleaseComplete §9.2.2.6",
			(&UEContextReleaseComplete{
				AMFUENGAPID:             Ptr(AMFUENGAPID(1)),
				RANUENGAPID:             Ptr(RANUENGAPID(2)),
				UserLocationInformation: &UserLocationInformation{Kind: UserLocationNR},
				PDUSessionResourceList:  PDUSessionResourceListCxtRelCpl{{PDUSessionID: 5}},
				CriticalityDiagnostics:  &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{idAMFUENGAPID, CriticalityIgnore},
				{idRANUENGAPID, CriticalityIgnore},
				{idUserLocationInformation, CriticalityIgnore},
				{idPDUSessionResourceListCxtRelCpl, CriticalityReject},
				{idCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"NASNonDeliveryIndication §9.2.5.4",
			(&NASNonDeliveryIndication{
				AMFUENGAPID: 1,
				RANUENGAPID: 2,
				NASPDU:      NASPDU{0x7e},
				Cause:       Ptr(Cause{Group: CauseGroupMisc, Value: CauseMiscUnspecified}),
			}).encodeBody,
			[]wireIE{
				{idAMFUENGAPID, CriticalityReject},
				{idRANUENGAPID, CriticalityReject},
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
