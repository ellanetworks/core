// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// canonicalValues is a value part each optional element accepts, shared by every
// message that carries the element. Only the framing and the position matter
// here, so each is the shortest value its codec takes.
var canonicalValues = map[uint8][]byte{
	ieiAPNAMBR:                       {0xfe, 0xfe},
	ieiAuthFailureParameter:          rep(0xC7, 14),
	ieiT3402Value:                    {0x21},
	ieiAccessPointName:               {0x03, 'a', 'b', 'c'},
	ieiAdditionalUpdateType:          {0x00},
	ieiEMMCause:                      {uint8(EMMCauseIllegalUE)},
	ieiEPSBearerContextStatus:        {0x00, 0x00},
	ieiESMCause:                      {uint8(ESMCauseRegularDeactivation)},
	ieiESMInformationTransferFlag:    {0x01},
	ieiFullNameForNetwork:            {0x89, 0x41},
	ieiGUTI:                          {0xf6, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x01},
	ieiHashMME:                       rep(0x5A, 8),
	ieiIMEISVRequest:                 {0x01},
	ieiLocalTimeZone:                 {0x00},
	ieiMSNetworkCapability:           {0xe5, 0xe0, 0x00},
	ieiNetworkDaylightSavingTime:     {0x00},
	ieiNetworkFeatureSupport:         {0x00},
	ieiOldGUTIType:                   {0x00},
	ieiProtocolConfigurationOptions:  {0x80},
	ieiReplayedNASMessage:            {0x07, 0x41, 0x71, 0x00},
	ieiShortNameForNetwork:           {0x89, 0x41},
	ieiT3346Value:                    {0x21},
	ieiT3442Value:                    {0x21},
	ieiT3448Value:                    {0x21},
	ieiTAIList:                       {0x00, 0x00, 0xf1, 0x10, 0x00, 0x01},
	ieiUniversalTimeAndLocalTimeZone: {0x22, 0x70, 0x11, 0x02, 0x03, 0x04, 0x00},
}

// rep is a value of n identical octets.
func rep(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}

	return out
}

// canonicalCases lists every message whose spec table gives it more than one
// optional element, with that table's order. The orders are transcribed from the
// message-content tables of TS 24.301.
func canonicalCases(t *testing.T) []canonicalCase {
	t.Helper()

	// An IEI is message-scoped, so an element sharing one with another message's
	// element needs its own value here.
	guti := map[uint8][]byte{ieiAdditionalGUTI: {0xf6, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x01}}
	// EPS reuses 0x5B for the T3442 value in EMM and the new EPS QoS in ESM.
	qos := map[uint8][]byte{ieiNewEPSQoS: {0x09}}
	imeisv := map[uint8][]byte{ieiIMEISV: {0x33, 0x21, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43, 0xf5}}
	// EPS reuses 0x58 for the UE network capability in TAU REQUEST and the ESM
	// cause in ESM messages.
	ueNetCap := map[uint8][]byte{ieiUENetworkCapability: {0xf0, 0x70}}

	return []canonicalCase{
		{
			name: "AttachRequest (TS 24.301 §8.2.4)",
			bare: &AttachRequest{
				EPSAttachType:       AttachTypeEPS,
				NASKeySetIdentifier: nas.NoKeySet,
				EPSMobileIdentity:   IMSIIdentity(IMSI("001010000000001")),
				ESMMessageContainer: []byte{0x02, 0x01, 0xD0, 0x11},
			},
			order:  []canonicalIE{{ieiAdditionalGUTI, nas.IETLV}, {ieiMSNetworkCapability, nas.IETLV}, {ieiAdditionalUpdateType, nas.IETV1}, {ieiOldGUTIType, nas.IETV1}},
			values: guti,
		},
		{
			name: "AttachAccept (TS 24.301 §8.2.1)",
			bare: &AttachAccept{
				EPSAttachResult:     AttachResultEPS,
				TAIList:             TAIList{{Type: PartialTAIListConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1}}}},
				ESMMessageContainer: []byte{0x02, 0x01, 0xD0, 0x11},
			},
			order: []canonicalIE{{ieiGUTI, nas.IETLV}, {ieiEMMCause, nas.IETV3}, {ieiNetworkFeatureSupport, nas.IETLV}},
		},
		{
			name:  "EMMInformation (TS 24.301 §8.2.13)",
			bare:  &EMMInformation{},
			order: []canonicalIE{{ieiFullNameForNetwork, nas.IETLV}, {ieiShortNameForNetwork, nas.IETLV}, {ieiLocalTimeZone, nas.IETV3}, {ieiUniversalTimeAndLocalTimeZone, nas.IETV3}, {ieiNetworkDaylightSavingTime, nas.IETLV}},
		},
		{
			name:  "SecurityModeCommand (TS 24.301 §8.2.20)",
			bare:  &SecurityModeCommand{},
			order: []canonicalIE{{ieiIMEISVRequest, nas.IETV1}, {ieiHashMME, nas.IETLV}},
		},
		{
			name:   "SecurityModeComplete (TS 24.301 §8.2.21)",
			bare:   &SecurityModeComplete{},
			order:  []canonicalIE{{ieiIMEISV, nas.IETLV}, {ieiReplayedNASMessage, nas.IETLVE}},
			values: imeisv,
		},
		{
			name:  "ServiceReject (TS 24.301 §8.2.24)",
			bare:  &ServiceReject{},
			order: []canonicalIE{{ieiT3442Value, nas.IETV3}, {ieiT3346Value, nas.IETLV}, {ieiT3448Value, nas.IETLV}},
		},
		{
			name:  "ServiceAccept (TS 24.301 §8.2.34)",
			bare:  &ServiceAccept{},
			order: []canonicalIE{{ieiEPSBearerContextStatus, nas.IETLV}, {ieiT3448Value, nas.IETLV}},
		},
		{
			name:  "TrackingAreaUpdateAccept (TS 24.301 §8.2.26)",
			bare:  &TrackingAreaUpdateAccept{},
			order: []canonicalIE{{ieiGUTI, nas.IETLV}, {ieiTAIList, nas.IETLV}, {ieiEPSBearerContextStatus, nas.IETLV}, {ieiEMMCause, nas.IETV3}, {ieiNetworkFeatureSupport, nas.IETLV}},
		},
		{
			name: "ActivateDefaultEPSBearerContextRequest (TS 24.301 §8.3.6)",
			bare: &ActivateDefaultEPSBearerContextRequest{
				EPSBearerIdentity: 5,
				AccessPointName:   APN("internet"),
			},
			order: []canonicalIE{{ieiAPNAMBR, nas.IETLV}, {ieiESMCause, nas.IETV3}, {ieiProtocolConfigurationOptions, nas.IETLV}},
		},
		{
			name:  "ESMInformationResponse (TS 24.301 §8.3.14)",
			bare:  &ESMInformationResponse{},
			order: []canonicalIE{{ieiAccessPointName, nas.IETLV}, {ieiProtocolConfigurationOptions, nas.IETLV}},
		},
		{
			name:   "ModifyEPSBearerContextRequest (TS 24.301 §8.3.18)",
			bare:   &ModifyEPSBearerContextRequest{},
			order:  []canonicalIE{{ieiNewEPSQoS, nas.IETLV}, {ieiAPNAMBR, nas.IETLV}, {ieiProtocolConfigurationOptions, nas.IETLV}},
			values: qos,
		},
		{
			name:  "PDNConnectivityRequest (TS 24.301 §8.3.20)",
			bare:  &PDNConnectivityRequest{},
			order: []canonicalIE{{ieiESMInformationTransferFlag, nas.IETV1}, {ieiAccessPointName, nas.IETLV}, {ieiProtocolConfigurationOptions, nas.IETLV}},
		},
		{
			name:  "AttachReject (TS 24.301 §8.2)",
			bare:  &AttachReject{},
			order: []canonicalIE{{ieiT3402Value, nas.IETLV}},
		},
		{
			name:  "AuthenticationFailure (TS 24.301 §8.2)",
			bare:  &AuthenticationFailure{},
			order: []canonicalIE{{ieiAuthFailureParameter, nas.IETLV}},
		},
		{
			name:     "DetachRequestNetwork (TS 24.301 §8.2.11)",
			bare:     &DetachRequestNetwork{},
			order:    []canonicalIE{{ieiEMMCause, nas.IETV3}},
			downlink: true,
		},
		{
			name: "TrackingAreaUpdateRequest (TS 24.301 §8.2)",
			bare: &TrackingAreaUpdateRequest{
				EPSUpdateType:       EPSUpdateTypeTA,
				NASKeySetIdentifier: nas.NoKeySet,
				OldGUTI:             GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0, 0, 0, 1}}),
			},
			order: []canonicalIE{
				{ieiUENetworkCapability, nas.IETLV},
				{ieiEPSBearerContextStatus, nas.IETLV},
				{ieiMSNetworkCapability, nas.IETLV},
			},
			values: ueNetCap,
		},
	}
}
