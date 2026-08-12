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
				{IDGlobalRANNodeID, CriticalityReject},
				{IDRANNodeName, CriticalityIgnore},
				{IDSupportedTAList, CriticalityReject},
				{IDDefaultPagingDRX, CriticalityIgnore},
			},
		},
		{
			"NGSetupResponse §9.2.6.2",
			goldResponse().encodeBody,
			[]wireIE{
				{IDAMFName, CriticalityReject},
				{IDServedGUAMIList, CriticalityReject},
				{IDRelativeAMFCapacity, CriticalityIgnore},
				{IDPLMNSupportList, CriticalityReject},
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
				{IDCause, CriticalityIgnore},
				{IDTimeToWait, CriticalityIgnore},
				{IDCriticalityDiagnostics, CriticalityIgnore},
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
				{IDRANUENGAPID, CriticalityReject},
				{IDNASPDU, CriticalityReject},
				{IDUserLocationInformation, CriticalityReject},
				{IDRRCEstablishmentCause, CriticalityIgnore},
				{IDFiveGSTMSI, CriticalityReject},
				{IDAMFSetID, CriticalityIgnore},
				{IDUEContextRequest, CriticalityIgnore},
				{IDAllowedNSSAI, CriticalityReject},
			},
		},
		{
			"DownlinkNASTransport §9.2.5.2",
			(&DownlinkNASTransport{AMFUENGAPID: 1, RANUENGAPID: 2, NASPDU: NASPDU{0x7e}}).encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDNASPDU, CriticalityReject},
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
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDNASPDU, CriticalityReject},
				{IDUserLocationInformation, CriticalityIgnore},
			},
		},
		{
			"InitialContextSetupRequest §9.2.2.1",
			goldInitialContextSetupRequest().encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDUEAggregateMaximumBitRate, CriticalityReject},
				{IDGUAMI, CriticalityReject},
				{IDPDUSessionResourceSetupListCxtReq, CriticalityReject},
				{IDAllowedNSSAI, CriticalityReject},
				{IDUESecurityCapabilities, CriticalityReject},
				{IDSecurityKey, CriticalityReject},
			},
		},
		{
			"InitialContextSetupResponse §9.2.2.2",
			(&InitialContextSetupResponse{
				AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
				RANUENGAPID:              Ptr(RANUENGAPID(2)),
				PDUSessionResourceSetup:  PDUSessionResourceSetupListCxtRes{{PDUSessionID: 5}},
				PDUSessionResourceFailed: PDUSessionResourceFailedToSetupListCxtRes{{PDUSessionID: 9}},
				CriticalityDiagnostics:   &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityIgnore},
				{IDRANUENGAPID, CriticalityIgnore},
				{IDPDUSessionResourceSetupListCxtRes, CriticalityIgnore},
				{IDPDUSessionResourceFailedToSetupListCxtRes, CriticalityIgnore},
				{IDCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"InitialContextSetupFailure §9.2.2.3",
			(&InitialContextSetupFailure{
				AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
				RANUENGAPID:              Ptr(RANUENGAPID(2)),
				PDUSessionResourceFailed: PDUSessionResourceFailedToSetupListCxtFail{{PDUSessionID: 5}},
				Cause:                    Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}),
				CriticalityDiagnostics:   &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityIgnore},
				{IDRANUENGAPID, CriticalityIgnore},
				{IDPDUSessionResourceFailedToSetupListCxtFail, CriticalityIgnore},
				{IDCause, CriticalityIgnore},
				{IDCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"PDUSessionResourceNotify §9.2.1.9",
			func() func(*per.Writer, per.Encoding) error {
				m := goldPDUSessionResourceNotify()
				m.UserLocationInformation = &UserLocationInformation{Kind: UserLocationNR}

				return m.encodeBody
			}(),
			[]wireIE{
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDPDUSessionResourceNotifyList, CriticalityReject},
				{IDPDUSessionResourceReleasedListNot, CriticalityIgnore},
				{IDUserLocationInformation, CriticalityIgnore},
			},
		},
		{
			"PDUSessionResourceModifyIndication §9.2.1.7",
			func() func(*per.Writer, per.Encoding) error {
				m := goldPDUSessionResourceModifyIndication()
				m.UserLocationInformation = &UserLocationInformation{Kind: UserLocationNR}

				return m.encodeBody
			}(),
			[]wireIE{
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDPDUSessionResourceModifyListModInd, CriticalityReject},
				{IDUserLocationInformation, CriticalityIgnore},
			},
		},
		{
			"PDUSessionResourceModifyConfirm §9.2.1.8",
			(&PDUSessionResourceModifyConfirm{
				AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
				RANUENGAPID:              Ptr(RANUENGAPID(2)),
				PDUSessionResourceModify: PDUSessionResourceModifyListModCfm{{PDUSessionID: 5}},
				PDUSessionResourceFailed: PDUSessionResourceFailedToModifyListModCfm{{PDUSessionID: 9}},
				CriticalityDiagnostics:   &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityIgnore},
				{IDRANUENGAPID, CriticalityIgnore},
				{IDPDUSessionResourceModifyListModCfm, CriticalityIgnore},
				{IDPDUSessionResourceFailedToModifyListModCfm, CriticalityIgnore},
				{IDCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"PDUSessionResourceModifyRequest §9.2.1.5",
			goldPDUSessionResourceModifyRequest().encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDPDUSessionResourceModifyListModReq, CriticalityReject},
			},
		},
		{
			"PDUSessionResourceModifyResponse §9.2.1.6",
			(&PDUSessionResourceModifyResponse{
				AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
				RANUENGAPID:              Ptr(RANUENGAPID(2)),
				PDUSessionResourceModify: PDUSessionResourceModifyListModRes{{PDUSessionID: 5}},
				PDUSessionResourceFailed: PDUSessionResourceFailedToModifyListModRes{{PDUSessionID: 9}},
				UserLocationInformation:  &UserLocationInformation{Kind: UserLocationNR},
				CriticalityDiagnostics:   &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityIgnore},
				{IDRANUENGAPID, CriticalityIgnore},
				{IDPDUSessionResourceModifyListModRes, CriticalityIgnore},
				{IDPDUSessionResourceFailedToModifyListModRes, CriticalityIgnore},
				{IDUserLocationInformation, CriticalityIgnore},
				{IDCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"PDUSessionResourceReleaseCommand §9.2.1.3",
			goldPDUSessionResourceReleaseCommand().encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDNASPDU, CriticalityIgnore},
				{IDPDUSessionResourceToReleaseListRelCmd, CriticalityReject},
			},
		},
		{
			"PDUSessionResourceReleaseResponse §9.2.1.4",
			(&PDUSessionResourceReleaseResponse{
				AMFUENGAPID:                Ptr(AMFUENGAPID(1)),
				RANUENGAPID:                Ptr(RANUENGAPID(2)),
				PDUSessionResourceReleased: PDUSessionResourceReleasedListRelRes{{PDUSessionID: 5}},
				UserLocationInformation:    &UserLocationInformation{Kind: UserLocationNR},
				CriticalityDiagnostics:     &CriticalityDiagnostics{},
			}).encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityIgnore},
				{IDRANUENGAPID, CriticalityIgnore},
				{IDPDUSessionResourceReleasedListRelRes, CriticalityIgnore},
				{IDUserLocationInformation, CriticalityIgnore},
				{IDCriticalityDiagnostics, CriticalityIgnore},
			},
		},
		{
			"PDUSessionResourceSetupRequest §9.2.1.1",
			goldPDUSessionResourceSetupRequest().encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDNASPDU, CriticalityReject},
				{IDPDUSessionResourceSetupListSUReq, CriticalityReject},
				{IDUEAggregateMaximumBitRate, CriticalityIgnore},
			},
		},
		{
			"PDUSessionResourceSetupResponse §9.2.1.2",
			(&PDUSessionResourceSetupResponse{
				AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
				RANUENGAPID:              Ptr(RANUENGAPID(2)),
				PDUSessionResourceSetup:  PDUSessionResourceSetupListSURes{{PDUSessionID: 5}},
				PDUSessionResourceFailed: PDUSessionResourceFailedToSetupListSURes{{PDUSessionID: 9}},
				CriticalityDiagnostics:   &CriticalityDiagnostics{},
				UserLocationInformation:  &UserLocationInformation{Kind: UserLocationNR},
			}).encodeBody,
			[]wireIE{
				{IDAMFUENGAPID, CriticalityIgnore},
				{IDRANUENGAPID, CriticalityIgnore},
				{IDPDUSessionResourceSetupListSURes, CriticalityIgnore},
				{IDPDUSessionResourceFailedToSetupListSURes, CriticalityIgnore},
				{IDCriticalityDiagnostics, CriticalityIgnore},
				{IDUserLocationInformation, CriticalityIgnore},
			},
		},
		{
			"UERadioCapabilityInfoIndication §9.2.3.1",
			func() func(*per.Writer, per.Encoding) error {
				m := goldUERadioCapabilityInfoIndication()
				m.UERadioCapabilityForPaging = &UERadioCapabilityForPaging{NR: &UERadioCapabilityForPagingOfNR{0x0a}}

				return m.encodeBody
			}(),
			[]wireIE{
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDUERadioCapability, CriticalityIgnore},
				{IDUERadioCapabilityForPaging, CriticalityIgnore},
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
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDPDUSessionResourceListCxtRelReq, CriticalityReject},
				{IDCause, CriticalityIgnore},
			},
		},
		{
			"UEContextReleaseCommand §9.2.2.5",
			(&UEContextReleaseCommand{
				UENGAPIDs: UENGAPIDs{AMFUENGAPID: 1, RANUENGAPID: 2, Pair: true},
				Cause:     Ptr(Cause{Group: CauseGroupNAS, Value: CauseNASNormalRelease}),
			}).encodeBody,
			[]wireIE{
				{IDUENGAPIDs, CriticalityReject},
				{IDCause, CriticalityIgnore},
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
				{IDAMFUENGAPID, CriticalityIgnore},
				{IDRANUENGAPID, CriticalityIgnore},
				{IDUserLocationInformation, CriticalityIgnore},
				{IDPDUSessionResourceListCxtRelCpl, CriticalityReject},
				{IDCriticalityDiagnostics, CriticalityIgnore},
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
				{IDAMFUENGAPID, CriticalityReject},
				{IDRANUENGAPID, CriticalityReject},
				{IDNASPDU, CriticalityIgnore},
				{IDCause, CriticalityIgnore},
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
