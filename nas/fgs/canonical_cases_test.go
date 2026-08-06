// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// canonicalValues is a value part each optional element accepts, shared by every
// message that carries the element. Only the framing and the position matter
// here, so each is the shortest value its codec takes.
var canonicalValues = map[uint8][]byte{
	iei5GSMCapability:                {0x00},
	iei5GSMCause:                     {uint8(GSMCauseRegularDeactivation)},
	ieiABBA:                          {0x00, 0x00},
	ieiAUTN:                          rep(0xA5, 16),
	ieiAdditional5GSec:               {0x00},
	ieiAdditionalInfo:                {0x01},
	ieiAuthFailureParam:              rep(0xC7, 14),
	ieiAllowedNSSAI:                  {0x01, 0x01},
	ieiAllowedPDUSessionStatus:       {0x00, 0x00},
	ieiAuthResponseParam:             rep(0xAB, 16),
	ieiBackoffTimer:                  {0x21},
	ieiCause5GMM:                     {uint8(GMMCausePLMNNotAllowed)},
	ieiConfigUpdateInd:               {0x01},
	ieiConfiguredNSSAI:               {0x01, 0x01},
	ieiEAPMessage:                    {0x01, 0x00, 0x00, 0x04},
	ieiEPSBearerContextStatus:        {0x00, 0x00},
	ieiEquivalentPlmns:               {0x00, 0xf1, 0x10},
	ieiExtendedPCO:                   {0x80},
	ieiFullNameForNet:                {0x89, 0x41},
	ieiGMMCapability:                 {0x00},
	ieiGUTI5G:                        {0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01},
	ieiIMEISVRequest:                 {0x01},
	ieiLocalTimeZone:                 {0x00},
	ieiMICOIndication:                {0x01},
	ieiUEStatus:                      {0x01},
	ieiNASMessageContainer:           {uint8(EPD5GMM), 0x00, uint8(MsgRegistrationRequest), 0x01, 0x00, 0x01, 0x00},
	ieiNetworkDaylightSavingTime:     {0x00},
	ieiNon3GppDeregTimer:             {0x21},
	ieiPDUAddress:                    {0x01, 10, 0, 0, 1},
	ieiPDUReactErrCause:              {0x01, uint8(GSMCauseRegularDeactivation)},
	ieiPDUReactResult:                {0x00, 0x00},
	ieiPDUSessionStatus:              {0x00, 0x00},
	ieiPDUSessionType:                {uint8(PDUSessionTypeIPv4)},
	ieiPduSessionID2:                 {0x05},
	ieiRAND:                          rep(0x5A, 16),
	ieiRQTimerValue:                  {0x05},
	ieiReplayedS1UESecCap:            {0x00, 0x00},
	ieiRequestType:                   {0x01},
	ieiRequestedDRXParameters:        {0x01},
	ieiRequestedNSSAI:                {0x01, 0x01},
	ieiSNSSAI:                        {0x01},
	ieiSORContainer:                  rep(0x00, 17),
	ieiSSCMode:                       {uint8(SSCMode1)},
	ieiSelectedEPSNASSecAlg:          {0x00},
	ieiSessionAMBR:                   {0x06, 0x00, 0x64, 0x06, 0x00, 0x32},
	ieiShortNameForNet:               {0x89, 0x41},
	ieiT3346Value:                    {0x21},
	ieiT3502Value:                    {0x21},
	ieiT3512Value:                    {0x21},
	ieiTAIList:                       {0x00, 0x00, 0xf1, 0x10, 0x00, 0x00, 0x01},
	ieiUESecurityCapability:          {0xf0, 0xf0},
	ieiUniversalTimeAndLocalTimeZone: {0x22, 0x70, 0x11, 0x02, 0x03, 0x04, 0x00},
	ieiUpdateType5GS:                 {0x00},
	ieiUplinkDataStatus:              {0x00, 0x00},
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
// message-content tables of TS 24.501.
func canonicalCases(t *testing.T) []canonicalCase {
	t.Helper()

	qosRules, err := QoSRules{{Identifier: 1, OperationCode: QoSRuleOpCreate, DQR: 1, Parameters: &QoSRuleParameters{Precedence: 1}}}.MarshalBinary()
	if err != nil {
		t.Fatalf("encode QoS rules: %v", err)
	}

	qosFlows, err := QoSFlowDescriptions{{QFI: 1, OperationCode: QoSFlowOpCreate}}.MarshalBinary()
	if err != nil {
		t.Fatalf("encode QoS flow descriptions: %v", err)
	}

	// An IEI is message-scoped, so an element sharing one with another message's
	// element needs its own value here (TS 24.501 §8: 0x25 is the allowed PDU
	// session status in 5GMM and the DNN in 5GSM, 0x77 the 5G-GUTI and the IMEISV).
	dnn := map[uint8][]byte{ieiDNN: {0x03, 'a', 'b', 'c'}, ieiOldPDUSessionID: {0x05}}
	accept := map[uint8][]byte{ieiNetworkFeature: {0x00, 0x00}, ieiNegotiatedDRX: {0x01}}
	imeisv := map[uint8][]byte{ieiIMEISV: {0x35, 0x21, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43, 0xf5}}
	qos := map[uint8][]byte{
		ieiQoSFlowDescription: qosFlows,
		ieiAuthorizedQoSRules: qosRules,
		ieiDNN:                {0x03, 'a', 'b', 'c'},
		ieiEAPMessageSession:  {0x01, 0x00, 0x00, 0x04},
		ieiAlwaysOnIndication: {0x01},
		ieiAlwaysOnRequested:  {0x01},
	}

	return []canonicalCase{
		{
			name:  "AuthenticationRequest (TS 24.501 §8.2.1)",
			bare:  &AuthenticationRequest{},
			order: []canonicalIE{{ieiRAND, nas.IETV3}, {ieiAUTN, nas.IETLV}, {ieiEAPMessage, nas.IETLVE}},
		},
		{
			name:  "AuthenticationResponse (TS 24.501 §8.2.2)",
			bare:  &AuthenticationResponse{},
			order: []canonicalIE{{ieiAuthResponseParam, nas.IETLV}, {ieiEAPMessage, nas.IETLVE}},
		},
		{
			name:  "ConfigurationUpdateCommand (TS 24.501 §8.2.19)",
			bare:  &ConfigurationUpdateCommand{},
			order: []canonicalIE{{ieiConfigUpdateInd, nas.IETV1}, {ieiGUTI5G, nas.IETLVE}, {ieiFullNameForNet, nas.IETLV}, {ieiShortNameForNet, nas.IETLV}, {ieiLocalTimeZone, nas.IETV3}, {ieiUniversalTimeAndLocalTimeZone, nas.IETV3}, {ieiNetworkDaylightSavingTime, nas.IETLV}},
		},
		{
			name:  "DeregistrationRequestUETerminated (TS 24.501 §8.2.14)",
			bare:  &DeregistrationRequestUETerminated{},
			order: []canonicalIE{{ieiCause5GMM, nas.IETV3}, {ieiT3346Value, nas.IETLV}},
		},
		{
			name:  "RegistrationRequest (TS 24.501 §8.2.6)",
			bare:  &RegistrationRequest{},
			order: []canonicalIE{{ieiGMMCapability, nas.IETLV}, {ieiUESecurityCapability, nas.IETLV}, {ieiRequestedNSSAI, nas.IETLV}, {ieiUplinkDataStatus, nas.IETLV}, {ieiPDUSessionStatus, nas.IETLV}, {ieiMICOIndication, nas.IETV1}, {ieiUEStatus, nas.IETLV}, {ieiAllowedPDUSessionStatus, nas.IETLV}, {ieiRequestedDRXParameters, nas.IETLV}, {ieiUpdateType5GS, nas.IETLV}, {ieiNASMessageContainer, nas.IETLVE}},
		},
		{
			name:   "RegistrationAccept (TS 24.501 §8.2.7)",
			bare:   &RegistrationAccept{},
			order:  []canonicalIE{{ieiGUTI5G, nas.IETLVE}, {ieiEquivalentPlmns, nas.IETLV}, {ieiTAIList, nas.IETLV}, {ieiAllowedNSSAI, nas.IETLV}, {ieiConfiguredNSSAI, nas.IETLV}, {ieiNetworkFeature, nas.IETLV}, {ieiPDUSessionStatus, nas.IETLV}, {ieiPDUReactResult, nas.IETLV}, {ieiPDUReactErrCause, nas.IETLVE}, {ieiMICOIndication, nas.IETV1}, {ieiT3512Value, nas.IETLV}, {ieiNon3GppDeregTimer, nas.IETLV}, {ieiT3502Value, nas.IETLV}, {ieiSORContainer, nas.IETLVE}, {ieiEAPMessage, nas.IETLVE}, {ieiNegotiatedDRX, nas.IETLV}, {ieiEPSBearerContextStatus, nas.IETLV}},
			values: accept,
		},
		{
			name:  "RegistrationReject (TS 24.501 §8.2.9)",
			bare:  &RegistrationReject{},
			order: []canonicalIE{{ieiT3346Value, nas.IETLV}, {ieiT3502Value, nas.IETLV}, {ieiEAPMessage, nas.IETLVE}},
		},
		{
			name:  "ServiceRequest (TS 24.501 §8.2.16)",
			bare:  &ServiceRequest{},
			order: []canonicalIE{{ieiUplinkDataStatus, nas.IETLV}, {ieiPDUSessionStatus, nas.IETLV}, {ieiAllowedPDUSessionStatus, nas.IETLV}, {ieiNASMessageContainer, nas.IETLVE}},
		},
		{
			name:  "ServiceAccept (TS 24.501 §8.2.17)",
			bare:  &ServiceAccept{},
			order: []canonicalIE{{ieiPDUSessionStatus, nas.IETLV}, {ieiPDUReactResult, nas.IETLV}, {ieiPDUReactErrCause, nas.IETLVE}, {ieiEAPMessage, nas.IETLVE}},
		},
		{
			name:  "ServiceReject (TS 24.501 §8.2.18)",
			bare:  &ServiceReject{},
			order: []canonicalIE{{ieiPDUSessionStatus, nas.IETLV}, {ieiT3346Value, nas.IETLV}, {ieiEAPMessage, nas.IETLVE}},
		},
		{
			name:  "SecurityModeCommand (TS 24.501 §8.2.25)",
			bare:  &SecurityModeCommand{},
			order: []canonicalIE{{ieiIMEISVRequest, nas.IETV1}, {ieiSelectedEPSNASSecAlg, nas.IETV3}, {ieiAdditional5GSec, nas.IETLV}, {ieiEAPMessage, nas.IETLVE}, {ieiABBA, nas.IETLV}, {ieiReplayedS1UESecCap, nas.IETLV}},
		},
		{
			name:   "SecurityModeComplete (TS 24.501 §8.2.26)",
			bare:   &SecurityModeComplete{},
			order:  []canonicalIE{{ieiIMEISV, nas.IETLVE}, {ieiNASMessageContainer, nas.IETLVE}},
			values: imeisv,
		},
		{
			name:  "DLNASTransport (TS 24.501 §8.2.11)",
			bare:  &DLNASTransport{},
			order: []canonicalIE{{ieiPduSessionID2, nas.IETV3}, {ieiAdditionalInfo, nas.IETLV}, {ieiCause5GMM, nas.IETV3}, {ieiBackoffTimer, nas.IETLV}},
		},
		{
			name:   "ULNASTransport (TS 24.501 §8.2.10)",
			bare:   &ULNASTransport{},
			order:  []canonicalIE{{ieiPduSessionID2, nas.IETV3}, {ieiOldPDUSessionID, nas.IETV3}, {ieiRequestType, nas.IETV1}, {ieiSNSSAI, nas.IETLV}, {ieiDNN, nas.IETLV}, {ieiAdditionalInfo, nas.IETLV}},
			values: dnn,
		},
		{
			name:   "PDUSessionEstablishmentRequest (TS 24.501 §8.3.1)",
			bare:   &PDUSessionEstablishmentRequest{},
			order:  []canonicalIE{{ieiPDUSessionType, nas.IETV1}, {ieiSSCMode, nas.IETV1}, {iei5GSMCapability, nas.IETLV}, {ieiAlwaysOnRequested, nas.IETV1}, {ieiExtendedPCO, nas.IETLVE}},
			values: qos,
		},
		{
			name:   "PDUSessionEstablishmentAccept (TS 24.501 §8.3.2)",
			bare:   &PDUSessionEstablishmentAccept{},
			order:  []canonicalIE{{iei5GSMCause, nas.IETV3}, {ieiPDUAddress, nas.IETLV}, {ieiRQTimerValue, nas.IETV3}, {ieiSNSSAI, nas.IETLV}, {ieiAlwaysOnIndication, nas.IETV1}, {ieiEAPMessageSession, nas.IETLVE}, {ieiQoSFlowDescription, nas.IETLVE}, {ieiExtendedPCO, nas.IETLVE}, {ieiDNN, nas.IETLV}},
			values: qos,
		},
		{
			name:   "PDUSessionModificationRequest (TS 24.501 §8.3.7)",
			bare:   &PDUSessionModificationRequest{},
			order:  []canonicalIE{{iei5GSMCapability, nas.IETLV}, {iei5GSMCause, nas.IETV3}, {ieiAlwaysOnRequested, nas.IETV1}, {ieiAuthorizedQoSRules, nas.IETLVE}, {ieiQoSFlowDescription, nas.IETLVE}, {ieiExtendedPCO, nas.IETLVE}},
			values: qos,
		},
		{
			name:  "PDUSessionModificationReject (TS 24.501 §8.3.8)",
			bare:  &PDUSessionModificationReject{},
			order: []canonicalIE{{ieiBackoffTimer, nas.IETLV}, {ieiExtendedPCO, nas.IETLVE}},
		},
		{
			name:   "PDUSessionModificationCommand (TS 24.501 §8.3.9)",
			bare:   &PDUSessionModificationCommand{},
			order:  []canonicalIE{{ieiSessionAMBR, nas.IETLV}, {ieiQoSFlowDescription, nas.IETLVE}, {ieiExtendedPCO, nas.IETLVE}},
			values: qos,
		},
		{
			name:  "AuthenticationReject (TS 24.501 §8.2.5)",
			bare:  &AuthenticationReject{},
			order: []canonicalIE{{ieiEAPMessage, nas.IETLVE}},
		},
		{
			name:  "AuthenticationFailure (TS 24.501 §8.2.4)",
			bare:  &AuthenticationFailure{},
			order: []canonicalIE{{ieiAuthFailureParam, nas.IETLV}},
		},
		{
			name:  "NotificationResponse (TS 24.501 §8.2)",
			bare:  &NotificationResponse{},
			order: []canonicalIE{{ieiPDUSessionStatus, nas.IETLV}},
		},
		{
			name:  "RegistrationComplete (TS 24.501 §8.2.8)",
			bare:  &RegistrationComplete{},
			order: []canonicalIE{{ieiSORContainer, nas.IETLVE}},
		},
		{
			name:  "PDUSessionReleaseRequest (TS 24.501 §8.3.12)",
			bare:  &PDUSessionReleaseRequest{},
			order: []canonicalIE{{iei5GSMCause, nas.IETV3}},
		},
		{
			name:  "PDUSessionReleaseComplete (TS 24.501 §8.3.15)",
			bare:  &PDUSessionReleaseComplete{},
			order: []canonicalIE{{iei5GSMCause, nas.IETV3}},
		},
	}
}
