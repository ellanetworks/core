// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	ctxt "context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/nassecurity"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

func BuildDLNASTransport(ue *context.AmfUe, payloadContainerType uint8, nasPdu []byte, pduSessionID uint8, cause *uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDLNASTransport)

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
	}

	dLNASTransport := nasMessage.NewDLNASTransport(0)
	dLNASTransport.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	dLNASTransport.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	dLNASTransport.SetMessageType(nas.MsgTypeDLNASTransport)
	dLNASTransport.SpareHalfOctetAndPayloadContainerType.SetPayloadContainerType(payloadContainerType)
	dLNASTransport.PayloadContainer.SetLen(uint16(len(nasPdu)))
	dLNASTransport.PayloadContainer.SetPayloadContainerContents(nasPdu)

	if pduSessionID != 0 {
		dLNASTransport.PduSessionID2Value = new(nasType.PduSessionID2Value)
		dLNASTransport.PduSessionID2Value.SetIei(nasMessage.DLNASTransportPduSessionID2ValueType)
		dLNASTransport.PduSessionID2Value.SetPduSessionID2Value(pduSessionID)
	}
	if cause != nil {
		dLNASTransport.Cause5GMM = new(nasType.Cause5GMM)
		dLNASTransport.Cause5GMM.SetIei(nasMessage.DLNASTransportCause5GMMType)
		dLNASTransport.Cause5GMM.SetCauseValue(*cause)
	}

	m.GmmMessage.DLNASTransport = dLNASTransport

	return nassecurity.Encode(ue, m)
}

func BuildIdentityRequest(typeOfIdentity uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeIdentityRequest)

	identityRequest := nasMessage.NewIdentityRequest(0)
	identityRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	identityRequest.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	identityRequest.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	identityRequest.IdentityRequestMessageIdentity.SetMessageType(nas.MsgTypeIdentityRequest)
	identityRequest.SpareHalfOctetAndIdentityType.SetTypeOfIdentity(typeOfIdentity)

	m.GmmMessage.IdentityRequest = identityRequest

	return m.PlainNasEncode()
}

func BuildAuthenticationRequest(ue *context.AmfUe) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeAuthenticationRequest)

	authenticationRequest := nasMessage.NewAuthenticationRequest(0)
	authenticationRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	authenticationRequest.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	authenticationRequest.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	authenticationRequest.AuthenticationRequestMessageIdentity.SetMessageType(nas.MsgTypeAuthenticationRequest)
	authenticationRequest.SpareHalfOctetAndNgksi = util.SpareHalfOctetAndNgksiToNas(ue.NgKsi)
	authenticationRequest.ABBA.SetLen(uint8(len(ue.ABBA)))
	authenticationRequest.ABBA.SetABBAContents(ue.ABBA)

	var tmpArray [16]byte

	rand, err := hex.DecodeString(ue.AuthenticationCtx.Rand)
	if err != nil {
		return nil, err
	}

	authenticationRequest.AuthenticationParameterRAND = nasType.NewAuthenticationParameterRAND(nasMessage.AuthenticationRequestAuthenticationParameterRANDType)
	copy(tmpArray[:], rand[0:16])
	authenticationRequest.AuthenticationParameterRAND.SetRANDValue(tmpArray)

	autn, err := hex.DecodeString(ue.AuthenticationCtx.Autn)
	if err != nil {
		return nil, err
	}

	authenticationRequest.AuthenticationParameterAUTN = nasType.NewAuthenticationParameterAUTN(nasMessage.AuthenticationRequestAuthenticationParameterAUTNType)
	authenticationRequest.AuthenticationParameterAUTN.SetLen(uint8(len(autn)))
	copy(tmpArray[:], autn[0:16])
	authenticationRequest.AuthenticationParameterAUTN.SetAUTN(tmpArray)

	m.GmmMessage.AuthenticationRequest = authenticationRequest

	return m.PlainNasEncode()
}

func BuildServiceAccept(ue *context.AmfUe, pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool, errPduSessionID, errCause []uint8,
) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeServiceAccept)

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
	}

	serviceAccept := nasMessage.NewServiceAccept(0)
	serviceAccept.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	serviceAccept.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	serviceAccept.SetMessageType(nas.MsgTypeServiceAccept)
	if pDUSessionStatus != nil {
		serviceAccept.PDUSessionStatus = new(nasType.PDUSessionStatus)
		serviceAccept.PDUSessionStatus.SetIei(nasMessage.ServiceAcceptPDUSessionStatusType)
		serviceAccept.PDUSessionStatus.SetLen(2)
		serviceAccept.PDUSessionStatus.Buffer = nasConvert.PSIToBuf(*pDUSessionStatus)
	}
	if reactivationResult != nil {
		serviceAccept.PDUSessionReactivationResult = new(nasType.PDUSessionReactivationResult)
		serviceAccept.PDUSessionReactivationResult.SetIei(nasMessage.ServiceAcceptPDUSessionReactivationResultType)
		serviceAccept.PDUSessionReactivationResult.SetLen(2)
		serviceAccept.PDUSessionReactivationResult.Buffer = nasConvert.PSIToBuf(*reactivationResult)
	}
	if errPduSessionID != nil {
		serviceAccept.PDUSessionReactivationResultErrorCause = new(nasType.PDUSessionReactivationResultErrorCause)
		serviceAccept.PDUSessionReactivationResultErrorCause.SetIei(
			nasMessage.ServiceAcceptPDUSessionReactivationResultErrorCauseType)
		buf := nasConvert.PDUSessionReactivationResultErrorCauseToBuf(errPduSessionID, errCause)
		serviceAccept.PDUSessionReactivationResultErrorCause.SetLen(uint16(len(buf)))
		serviceAccept.PDUSessionReactivationResultErrorCause.Buffer = buf
	}
	m.GmmMessage.ServiceAccept = serviceAccept

	return nassecurity.Encode(ue, m)
}

func BuildAuthenticationReject(ue *context.AmfUe) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeAuthenticationReject)

	authenticationReject := nasMessage.NewAuthenticationReject(0)
	authenticationReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	authenticationReject.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	authenticationReject.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	authenticationReject.AuthenticationRejectMessageIdentity.SetMessageType(nas.MsgTypeAuthenticationReject)

	m.GmmMessage.AuthenticationReject = authenticationReject

	return m.PlainNasEncode()
}

// T3346 Timer and EAP are not Supported
func BuildServiceReject(pDUSessionStatus *[16]bool, cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeServiceReject)

	serviceReject := nasMessage.NewServiceReject(0)
	serviceReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	serviceReject.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	serviceReject.SetMessageType(nas.MsgTypeServiceReject)
	serviceReject.SetCauseValue(cause)
	if pDUSessionStatus != nil {
		serviceReject.PDUSessionStatus = new(nasType.PDUSessionStatus)
		serviceReject.PDUSessionStatus.SetIei(nasMessage.ServiceAcceptPDUSessionStatusType)
		serviceReject.PDUSessionStatus.SetLen(2)
		serviceReject.PDUSessionStatus.Buffer = nasConvert.PSIToBuf(*pDUSessionStatus)
	}

	m.GmmMessage.ServiceReject = serviceReject

	return m.PlainNasEncode()
}

// T3346 timer are not supported
func BuildRegistrationReject(ue *context.AmfUe, cause5GMM uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeRegistrationReject)

	registrationReject := nasMessage.NewRegistrationReject(0)
	registrationReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	registrationReject.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	registrationReject.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	registrationReject.RegistrationRejectMessageIdentity.SetMessageType(nas.MsgTypeRegistrationReject)
	registrationReject.Cause5GMM.SetCauseValue(cause5GMM)

	if ue.T3502Value != 0 {
		registrationReject.T3502Value = nasType.NewT3502Value(nasMessage.RegistrationRejectT3502ValueType)
		registrationReject.T3502Value.SetLen(1)
		t3502 := nasConvert.GPRSTimer2ToNas(ue.T3502Value)
		registrationReject.T3502Value.SetGPRSTimer2Value(t3502)
	}

	m.GmmMessage.RegistrationReject = registrationReject

	return m.PlainNasEncode()
}

// TS 24.501 8.2.25
func BuildSecurityModeCommand(ue *context.AmfUe) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeSecurityModeCommand)

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext,
	}

	securityModeCommand := nasMessage.NewSecurityModeCommand(0)
	securityModeCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	securityModeCommand.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	securityModeCommand.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	securityModeCommand.SecurityModeCommandMessageIdentity.SetMessageType(nas.MsgTypeSecurityModeCommand)

	securityModeCommand.SelectedNASSecurityAlgorithms.SetTypeOfCipheringAlgorithm(ue.CipheringAlg)
	securityModeCommand.SelectedNASSecurityAlgorithms.SetTypeOfIntegrityProtectionAlgorithm(ue.IntegrityAlg)

	securityModeCommand.SpareHalfOctetAndNgksi = util.SpareHalfOctetAndNgksiToNas(ue.NgKsi)

	securityModeCommand.ReplayedUESecurityCapabilities.SetLen(ue.UESecurityCapability.GetLen())
	securityModeCommand.ReplayedUESecurityCapabilities.Buffer = ue.UESecurityCapability.Buffer

	if ue.Pei != "" {
		securityModeCommand.IMEISVRequest = nasType.NewIMEISVRequest(nasMessage.SecurityModeCommandIMEISVRequestType)
		securityModeCommand.IMEISVRequest.SetIMEISVRequestValue(nasMessage.IMEISVNotRequested)
	} else {
		securityModeCommand.IMEISVRequest = nasType.NewIMEISVRequest(nasMessage.SecurityModeCommandIMEISVRequestType)
		securityModeCommand.IMEISVRequest.SetIMEISVRequestValue(nasMessage.IMEISVRequested)
	}

	securityModeCommand.Additional5GSecurityInformation = nasType.NewAdditional5GSecurityInformation(nasMessage.SecurityModeCommandAdditional5GSecurityInformationType)
	securityModeCommand.Additional5GSecurityInformation.SetLen(1)
	if ue.RetransmissionOfInitialNASMsg {
		securityModeCommand.Additional5GSecurityInformation.SetRINMR(1)
	} else {
		securityModeCommand.Additional5GSecurityInformation.SetRINMR(0)
	}

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSPeriodicRegistrationUpdating ||
		ue.RegistrationType5GS == nasMessage.RegistrationType5GSMobilityRegistrationUpdating {
		securityModeCommand.Additional5GSecurityInformation.SetHDP(1)
	} else {
		securityModeCommand.Additional5GSecurityInformation.SetHDP(0)
	}

	ue.SecurityContextAvailable = true
	m.GmmMessage.SecurityModeCommand = securityModeCommand
	payload, err := nassecurity.Encode(ue, m)
	if err != nil {
		ue.SecurityContextAvailable = false
		return nil, err
	} else {
		return payload, nil
	}
}

func BuildDeregistrationAccept() ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration)

	deregistrationAccept := nasMessage.NewDeregistrationAcceptUEOriginatingDeregistration(0)
	deregistrationAccept.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	deregistrationAccept.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	deregistrationAccept.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	deregistrationAccept.SetMessageType(nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration)

	m.GmmMessage.DeregistrationAcceptUEOriginatingDeregistration = deregistrationAccept

	return m.PlainNasEncode()
}

func BuildRegistrationAccept(
	ctx ctxt.Context,
	ue *context.AmfUe,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	supportedPLMN *context.PlmnSupportItem,
) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeRegistrationAccept)

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
	}

	registrationAccept := nasMessage.NewRegistrationAccept(0)
	registrationAccept.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	registrationAccept.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	registrationAccept.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	registrationAccept.RegistrationAcceptMessageIdentity.SetMessageType(nas.MsgTypeRegistrationAccept)

	registrationAccept.RegistrationResult5GS.SetLen(1)
	registrationResult := uint8(0)
	registrationResult |= nasMessage.AccessType3GPP
	registrationAccept.RegistrationResult5GS.SetRegistrationResultValue5GS(registrationResult)

	if ue.Guti != "" {
		gutiNas := nasConvert.GutiToNas(ue.Guti)
		registrationAccept.GUTI5G = &gutiNas
		registrationAccept.GUTI5G.SetIei(nasMessage.RegistrationAcceptGUTI5GType)
	}

	registrationAccept.EquivalentPlmns = nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)
	var buf []uint8
	plmnID, err := util.PlmnIDToNas(supportedPLMN.PlmnID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert PLMN ID to NAS: %s", err)
	}
	buf = append(buf, plmnID...)
	registrationAccept.EquivalentPlmns.SetLen(uint8(len(buf)))
	copy(registrationAccept.EquivalentPlmns.Octet[:], buf)

	if len(ue.RegistrationArea) > 0 {
		registrationAccept.TAIList = nasType.NewTAIList(nasMessage.RegistrationAcceptTAIListType)
		taiListNas, err := util.TaiListToNas(ue.RegistrationArea)
		if err != nil {
			return nil, fmt.Errorf("failed to convert TAI list to NAS: %s", err)
		}
		registrationAccept.TAIList.SetLen(uint8(len(taiListNas)))
		registrationAccept.TAIList.SetPartialTrackingAreaIdentityList(taiListNas)
	}

	if ue.AllowedNssai != nil {
		registrationAccept.AllowedNSSAI = nasType.NewAllowedNSSAI(nasMessage.RegistrationAcceptAllowedNSSAIType)

		snssai, err := util.SnssaiToNas(*ue.AllowedNssai)
		if err != nil {
			return nil, fmt.Errorf("failed to convert SNSSAI to NAS: %s", err)
		}

		var buf []uint8

		buf = append(buf, snssai...)
		registrationAccept.AllowedNSSAI.SetLen(uint8(len(buf)))
		registrationAccept.AllowedNSSAI.SetSNSSAIValue(buf)
	}

	// 5gs network feature support
	amfSelf := context.AMFSelf()
	if amfSelf.Get5gsNwFeatSuppEnable() {
		registrationAccept.NetworkFeatureSupport5GS = nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
		registrationAccept.NetworkFeatureSupport5GS.SetLen(2)
		registrationAccept.SetIMSVoPS3GPP(amfSelf.Get5gsNwFeatSuppImsVoPS())
		registrationAccept.SetEMC(amfSelf.Get5gsNwFeatSuppEmc())
		registrationAccept.SetEMF(amfSelf.Get5gsNwFeatSuppEmf())
		registrationAccept.SetIWKN26(amfSelf.Get5gsNwFeatSuppIwkN26())
		registrationAccept.SetMPSI(amfSelf.Get5gsNwFeatSuppMpsi())
		registrationAccept.SetEMCN(amfSelf.Get5gsNwFeatSuppEmcN3())
		registrationAccept.SetMCSI(amfSelf.Get5gsNwFeatSuppMcsi())
	}

	if pDUSessionStatus != nil {
		registrationAccept.PDUSessionStatus = nasType.NewPDUSessionStatus(nasMessage.RegistrationAcceptPDUSessionStatusType)
		registrationAccept.PDUSessionStatus.SetLen(2)
		registrationAccept.PDUSessionStatus.Buffer = nasConvert.PSIToBuf(*pDUSessionStatus)
	}

	if reactivationResult != nil {
		registrationAccept.PDUSessionReactivationResult = nasType.NewPDUSessionReactivationResult(nasMessage.RegistrationAcceptPDUSessionReactivationResultType)
		registrationAccept.PDUSessionReactivationResult.SetLen(2)
		registrationAccept.PDUSessionReactivationResult.Buffer = nasConvert.PSIToBuf(*reactivationResult)
	}

	if errPduSessionID != nil {
		registrationAccept.PDUSessionReactivationResultErrorCause = nasType.NewPDUSessionReactivationResultErrorCause(
			nasMessage.RegistrationAcceptPDUSessionReactivationResultErrorCauseType)
		buf := nasConvert.PDUSessionReactivationResultErrorCauseToBuf(errPduSessionID, errCause)
		registrationAccept.PDUSessionReactivationResultErrorCause.SetLen(uint16(len(buf)))
		registrationAccept.PDUSessionReactivationResultErrorCause.Buffer = buf
	}

	registrationAccept.T3512Value = nasType.NewT3512Value(nasMessage.RegistrationAcceptT3512ValueType)
	registrationAccept.T3512Value.SetLen(1)
	t3512 := nasConvert.GPRSTimer3ToNas(ue.T3512Value)
	registrationAccept.T3512Value.Octet = t3512

	// Temporary: commented this timer because UESIM is not supporting
	/*if ue.T3502Value != 0 {
		registrationAccept.T3502Value = nasType.NewT3502Value(nasMessage.RegistrationAcceptT3502ValueType)
		registrationAccept.T3502Value.SetLen(1)
		t3502 := nasConvert.GPRSTimer2ToNas(ue.T3502Value)
		registrationAccept.T3502Value.SetGPRSTimer2Value(t3502)
	}*/

	if ue.UESpecificDRX != nasMessage.DRXValueNotSpecified {
		registrationAccept.NegotiatedDRXParameters = nasType.NewNegotiatedDRXParameters(nasMessage.RegistrationAcceptNegotiatedDRXParametersType)
		registrationAccept.NegotiatedDRXParameters.SetLen(1)
		registrationAccept.NegotiatedDRXParameters.SetDRXValue(ue.UESpecificDRX)
	}

	m.GmmMessage.RegistrationAccept = registrationAccept

	return nassecurity.Encode(ue, m)
}

// TS 24.501 - 5.4.4 Generic UE configuration update procedure - 5.4.4.1 General
func BuildConfigurationUpdateCommand(ue *context.AmfUe, flags *context.ConfigurationUpdateCommandFlags) ([]byte, error, bool) {
	needTimer := false
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeConfigurationUpdateCommand)

	configurationUpdateCommand := nasMessage.NewConfigurationUpdateCommand(0)
	configurationUpdateCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	configurationUpdateCommand.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	configurationUpdateCommand.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	configurationUpdateCommand.SetMessageType(nas.MsgTypeConfigurationUpdateCommand)

	if flags.NeedNetworkSlicingIndication {
		configurationUpdateCommand.NetworkSlicingIndication = nasType.
			NewNetworkSlicingIndication(nasMessage.ConfigurationUpdateCommandNetworkSlicingIndicationType)
		configurationUpdateCommand.NetworkSlicingIndication.SetNSSCI(0x01)
	}

	if flags.NeedGUTI {
		if ue.Guti != "" {
			gutiNas, err := nasConvert.GutiToNasWithError(ue.Guti)
			if err != nil {
				return nil, fmt.Errorf("encode GUTI failed: %w", err), needTimer
			}
			configurationUpdateCommand.GUTI5G = &gutiNas
			configurationUpdateCommand.GUTI5G.SetIei(nasMessage.ConfigurationUpdateCommandGUTI5GType)
		} else {
			ue.GmmLog.Warn("Require 5G-GUTI, but got nothing.")
		}
	}

	if flags.NeedAllowedNSSAI {
		if ue.AllowedNssai != nil {
			configurationUpdateCommand.AllowedNSSAI = nasType.NewAllowedNSSAI(nasMessage.ConfigurationUpdateCommandAllowedNSSAIType)

			var buf []uint8

			allowedSnssaiNas, err := util.SnssaiToNas(*ue.AllowedNssai)
			if err != nil {
				return nil, fmt.Errorf("failed to convert allowed SNSSAI to NAS: %v", err), false
			}

			buf = append(buf, allowedSnssaiNas...)
			configurationUpdateCommand.AllowedNSSAI.SetLen(uint8(len(buf)))
			configurationUpdateCommand.AllowedNSSAI.SetSNSSAIValue(buf)
		} else {
			ue.GmmLog.Warn("Require Allowed NSSAI, but got nothing.")
		}
	}

	// Using this code requires additional refactoring, like adding the field ConfiguredNssai to
	// the amf_ue context.
	//
	// if flags.NeedConfiguredNSSAI {
	// 	if len(ue.ConfiguredNssai) > 0 {
	// 		configurationUpdateCommand.ConfiguredNSSAI = nasType.
	// 			NewConfiguredNSSAI(nasMessage.ConfigurationUpdateCommandConfiguredNSSAIType)

	// 		var buf []uint8
	// 		for _, snssai := range ue.ConfiguredNssai {
	// 			configuredSnssaiNas, err := util.SnssaiToNas(*snssai.ConfiguredSnssai)
	// 			if err != nil {
	// 				return nil, fmt.Errorf("failed to convert allowed SNSSAI to NAS: %v", err), false
	// 			}
	// 			buf = append(buf, configuredSnssaiNas...)
	// 		}
	// 		configurationUpdateCommand.ConfiguredNSSAI.SetLen(uint8(len(buf)))
	// 		configurationUpdateCommand.ConfiguredNSSAI.SetSNSSAIValue(buf)
	// 	} else {
	// 		ue.GmmLog.Warn("Require Configured NSSAI, but got nothing.")
	// 	}
	// }

	if flags.NeedRejectNSSAI {
		ue.GmmLog.Warn("Require Rejected NSSAI, but got nothing.")
	}

	if flags.NeedTaiList {
		if len(ue.RegistrationArea) > 0 {
			configurationUpdateCommand.TAIList = nasType.NewTAIList(nasMessage.ConfigurationUpdateCommandTAIListType)
			taiListNas, err := util.TaiListToNas(ue.RegistrationArea)
			if err != nil {
				return nil, fmt.Errorf("failed to convert TAI list to NAS: %v", err), false
			}
			configurationUpdateCommand.TAIList.SetLen(uint8(len(taiListNas)))
			configurationUpdateCommand.TAIList.SetPartialTrackingAreaIdentityList(taiListNas)
		} else {
			ue.GmmLog.Warn("Require TAI List, but got nothing.")
		}
	}

	if flags.NeedServiceAreaList {
		ue.GmmLog.Warn("Require Service Area List, but got nothing.")
	}

	if flags.NeedLadnInformation {
		ue.GmmLog.Warn("Require LADN Information, but got nothing.")
	}

	amfSelf := context.AMFSelf()

	if flags.NeedNITZ {
		// Full network name
		if amfSelf.NetworkName.Full != "" {
			fullNetworkName := nasConvert.FullNetworkNameToNas(amfSelf.NetworkName.Full)
			configurationUpdateCommand.FullNameForNetwork = &fullNetworkName
			configurationUpdateCommand.FullNameForNetwork.SetIei(nasMessage.ConfigurationUpdateCommandFullNameForNetworkType)
		} else {
			ue.GmmLog.Warn("Require Full Network Name, but got nothing.")
		}
		// Short network name
		if amfSelf.NetworkName.Short != "" {
			shortNetworkName := nasConvert.ShortNetworkNameToNas(amfSelf.NetworkName.Short)
			configurationUpdateCommand.ShortNameForNetwork = &shortNetworkName
			configurationUpdateCommand.ShortNameForNetwork.SetIei(nasMessage.ConfigurationUpdateCommandShortNameForNetworkType)
		} else {
			ue.GmmLog.Warn("Require Short Network Name, but got nothing.")
		}
		// Universal Time and Local Time Zone
		now := time.Now()
		universalTimeAndLocalTimeZone := nasConvert.EncodeUniversalTimeAndLocalTimeZoneToNas(now)
		universalTimeAndLocalTimeZone.SetIei(nasMessage.ConfigurationUpdateCommandUniversalTimeAndLocalTimeZoneType)
		configurationUpdateCommand.UniversalTimeAndLocalTimeZone = &universalTimeAndLocalTimeZone

		if ue.TimeZone != amfSelf.TimeZone {
			ue.TimeZone = amfSelf.TimeZone
			// Local Time Zone
			localTimeZone := nasConvert.EncodeLocalTimeZoneToNas(ue.TimeZone)
			localTimeZone.SetIei(nasMessage.ConfigurationUpdateCommandLocalTimeZoneType)
			configurationUpdateCommand.LocalTimeZone = nasType.
				NewLocalTimeZone(nasMessage.ConfigurationUpdateCommandLocalTimeZoneType)
			configurationUpdateCommand.LocalTimeZone = &localTimeZone
			// Daylight Saving Time
			daylightSavingTime := nasConvert.EncodeDaylightSavingTimeToNas(ue.TimeZone)
			daylightSavingTime.SetIei(nasMessage.ConfigurationUpdateCommandNetworkDaylightSavingTimeType)
			configurationUpdateCommand.NetworkDaylightSavingTime = nasType.
				NewNetworkDaylightSavingTime(nasMessage.ConfigurationUpdateCommandNetworkDaylightSavingTimeType)
			configurationUpdateCommand.NetworkDaylightSavingTime = &daylightSavingTime
		}
	}

	configurationUpdateCommand.ConfigurationUpdateIndication = nasType.
		NewConfigurationUpdateIndication(nasMessage.ConfigurationUpdateCommandConfigurationUpdateIndicationType)
	if configurationUpdateCommand.GUTI5G != nil ||
		configurationUpdateCommand.TAIList != nil ||
		configurationUpdateCommand.AllowedNSSAI != nil ||
		configurationUpdateCommand.LADNInformation != nil ||
		configurationUpdateCommand.ServiceAreaList != nil ||
		configurationUpdateCommand.MICOIndication != nil ||
		configurationUpdateCommand.ConfiguredNSSAI != nil ||
		configurationUpdateCommand.RejectedNSSAI != nil ||
		configurationUpdateCommand.NetworkSlicingIndication != nil ||
		configurationUpdateCommand.OperatordefinedAccessCategoryDefinitions != nil ||
		configurationUpdateCommand.SMSIndication != nil {
		// TS 24.501 - 5.4.4.2 Generic UE configuration update procedure initiated by the network
		// Acknowledgement shall be requested for all parameters except when only NITZ is included
		configurationUpdateCommand.ConfigurationUpdateIndication.SetACK(uint8(1))
		needTimer = true
	}
	if configurationUpdateCommand.MICOIndication != nil {
		// Allowed NSSAI and Configured NSSAI are optional to request to perform the registration procedure
		configurationUpdateCommand.ConfigurationUpdateIndication.SetRED(uint8(1))
	}

	// Check if the Configuration Update Command is vaild
	if configurationUpdateCommand.ConfigurationUpdateIndication.GetACK() == uint8(0) &&
		configurationUpdateCommand.ConfigurationUpdateIndication.GetRED() == uint8(0) &&
		(configurationUpdateCommand.FullNameForNetwork == nil &&
			configurationUpdateCommand.ShortNameForNetwork == nil &&
			configurationUpdateCommand.UniversalTimeAndLocalTimeZone == nil &&
			configurationUpdateCommand.LocalTimeZone == nil &&
			configurationUpdateCommand.NetworkDaylightSavingTime == nil) {
		return nil, fmt.Errorf("configuration update command is invalid"), false
	}

	m.GmmMessage.ConfigurationUpdateCommand = configurationUpdateCommand

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
	}

	b, err := nassecurity.Encode(ue, m)
	if err != nil {
		return nil, fmt.Errorf("could not encode NAS message: %v", err), false
	}
	return b, err, needTimer
}
