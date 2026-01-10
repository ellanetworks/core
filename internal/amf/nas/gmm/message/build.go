// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	"encoding/hex"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

func BuildDLNASTransport(ue *amfContext.AmfUe, payloadContainerType uint8, nasPdu []byte, pduSessionID uint8, cause *uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDLNASTransport)

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
	}

	dLNASTransport := nasMessage.NewDLNASTransport(0)
	dLNASTransport.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	dLNASTransport.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	dLNASTransport.SetMessageType(nas.MsgTypeDLNASTransport)
	dLNASTransport.SetPayloadContainerType(payloadContainerType)
	dLNASTransport.PayloadContainer.SetLen(uint16(len(nasPdu)))
	dLNASTransport.SetPayloadContainerContents(nasPdu)

	if pduSessionID != 0 {
		dLNASTransport.PduSessionID2Value = new(nasType.PduSessionID2Value)
		dLNASTransport.PduSessionID2Value.SetIei(nasMessage.DLNASTransportPduSessionID2ValueType)
		dLNASTransport.SetPduSessionID2Value(pduSessionID)
	}

	if cause != nil {
		dLNASTransport.Cause5GMM = new(nasType.Cause5GMM)
		dLNASTransport.Cause5GMM.SetIei(nasMessage.DLNASTransportCause5GMMType)
		dLNASTransport.SetCauseValue(*cause)
	}

	m.DLNASTransport = dLNASTransport

	return ue.EncodeNASMessage(m)
}

func BuildIdentityRequest(typeOfIdentity uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeIdentityRequest)

	identityRequest := nasMessage.NewIdentityRequest(0)
	identityRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	identityRequest.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	identityRequest.SetSpareHalfOctet(0)
	identityRequest.SetMessageType(nas.MsgTypeIdentityRequest)
	identityRequest.SetTypeOfIdentity(typeOfIdentity)

	m.IdentityRequest = identityRequest

	return m.PlainNasEncode()
}

func BuildAuthenticationRequest(ue *amfContext.AmfUe) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeAuthenticationRequest)

	authenticationRequest := nasMessage.NewAuthenticationRequest(0)
	authenticationRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	authenticationRequest.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	authenticationRequest.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	authenticationRequest.SetMessageType(nas.MsgTypeAuthenticationRequest)
	authenticationRequest.SpareHalfOctetAndNgksi = util.SpareHalfOctetAndNgksiToNas(ue.NgKsi)
	authenticationRequest.ABBA.SetLen(uint8(len(ue.ABBA)))
	authenticationRequest.SetABBAContents(ue.ABBA)

	var tmpArray [16]byte

	rand, err := hex.DecodeString(ue.AuthenticationCtx.Rand)
	if err != nil {
		return nil, err
	}

	authenticationRequest.AuthenticationParameterRAND = nasType.NewAuthenticationParameterRAND(nasMessage.AuthenticationRequestAuthenticationParameterRANDType)

	copy(tmpArray[:], rand[0:16])
	authenticationRequest.SetRANDValue(tmpArray)

	autn, err := hex.DecodeString(ue.AuthenticationCtx.Autn)
	if err != nil {
		return nil, err
	}

	authenticationRequest.AuthenticationParameterAUTN = nasType.NewAuthenticationParameterAUTN(nasMessage.AuthenticationRequestAuthenticationParameterAUTNType)
	authenticationRequest.AuthenticationParameterAUTN.SetLen(uint8(len(autn)))
	copy(tmpArray[:], autn[0:16])
	authenticationRequest.SetAUTN(tmpArray)

	m.AuthenticationRequest = authenticationRequest

	return m.PlainNasEncode()
}

func BuildServiceAccept(ue *amfContext.AmfUe, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) ([]byte, error) {
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

	m.ServiceAccept = serviceAccept

	return ue.EncodeNASMessage(m)
}

func BuildAuthenticationReject() ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeAuthenticationReject)

	authenticationReject := nasMessage.NewAuthenticationReject(0)
	authenticationReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	authenticationReject.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	authenticationReject.SetSpareHalfOctet(0)
	authenticationReject.SetMessageType(nas.MsgTypeAuthenticationReject)

	m.AuthenticationReject = authenticationReject

	return m.PlainNasEncode()
}

// T3346 Timer and EAP are not Supported
func BuildServiceReject(cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeServiceReject)

	serviceReject := nasMessage.NewServiceReject(0)
	serviceReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	serviceReject.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	serviceReject.SetMessageType(nas.MsgTypeServiceReject)
	serviceReject.SetCauseValue(cause)

	m.ServiceReject = serviceReject

	return m.PlainNasEncode()
}

// T3346 timer are not supported
func BuildRegistrationReject(t3502Value int, cause5GMM uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeRegistrationReject)

	registrationReject := nasMessage.NewRegistrationReject(0)
	registrationReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	registrationReject.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	registrationReject.SetSpareHalfOctet(0)
	registrationReject.SetMessageType(nas.MsgTypeRegistrationReject)
	registrationReject.SetCauseValue(cause5GMM)

	if t3502Value != 0 {
		registrationReject.T3502Value = nasType.NewT3502Value(nasMessage.RegistrationRejectT3502ValueType)
		registrationReject.T3502Value.SetLen(1)

		t3502 := nasConvert.GPRSTimer2ToNas(t3502Value)
		registrationReject.T3502Value.SetGPRSTimer2Value(t3502)
	}

	m.RegistrationReject = registrationReject

	return m.PlainNasEncode()
}

// TS 24.501 8.2.25
func BuildSecurityModeCommand(ue *amfContext.AmfUe) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeSecurityModeCommand)

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext,
	}

	securityModeCommand := nasMessage.NewSecurityModeCommand(0)
	securityModeCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	securityModeCommand.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	securityModeCommand.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	securityModeCommand.SetMessageType(nas.MsgTypeSecurityModeCommand)

	securityModeCommand.SelectedNASSecurityAlgorithms.SetTypeOfCipheringAlgorithm(ue.CipheringAlg)
	securityModeCommand.SelectedNASSecurityAlgorithms.SetTypeOfIntegrityProtectionAlgorithm(ue.IntegrityAlg)

	securityModeCommand.SpareHalfOctetAndNgksi = util.SpareHalfOctetAndNgksiToNas(ue.NgKsi)

	securityModeCommand.ReplayedUESecurityCapabilities.SetLen(ue.UESecurityCapability.GetLen())
	securityModeCommand.ReplayedUESecurityCapabilities.Buffer = ue.UESecurityCapability.Buffer

	if ue.Pei != "" {
		securityModeCommand.IMEISVRequest = nasType.NewIMEISVRequest(nasMessage.SecurityModeCommandIMEISVRequestType)
		securityModeCommand.SetIMEISVRequestValue(nasMessage.IMEISVNotRequested)
	} else {
		securityModeCommand.IMEISVRequest = nasType.NewIMEISVRequest(nasMessage.SecurityModeCommandIMEISVRequestType)
		securityModeCommand.SetIMEISVRequestValue(nasMessage.IMEISVRequested)
	}

	securityModeCommand.Additional5GSecurityInformation = nasType.NewAdditional5GSecurityInformation(nasMessage.SecurityModeCommandAdditional5GSecurityInformationType)
	securityModeCommand.Additional5GSecurityInformation.SetLen(1)

	if ue.RetransmissionOfInitialNASMsg {
		securityModeCommand.SetRINMR(1)
	} else {
		securityModeCommand.SetRINMR(0)
	}

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSPeriodicRegistrationUpdating || ue.RegistrationType5GS == nasMessage.RegistrationType5GSMobilityRegistrationUpdating {
		securityModeCommand.SetHDP(1)
	} else {
		securityModeCommand.SetHDP(0)
	}

	ue.SecurityContextAvailable = true
	m.SecurityModeCommand = securityModeCommand

	payload, err := ue.EncodeNASMessage(m)
	if err != nil {
		ue.SecurityContextAvailable = false
		return nil, err
	}

	return payload, nil
}

func BuildDeregistrationAccept() ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration)

	deregistrationAccept := nasMessage.NewDeregistrationAcceptUEOriginatingDeregistration(0)
	deregistrationAccept.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	deregistrationAccept.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	deregistrationAccept.SetSpareHalfOctet(0)
	deregistrationAccept.SetMessageType(nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration)

	m.DeregistrationAcceptUEOriginatingDeregistration = deregistrationAccept

	return m.PlainNasEncode()
}

func BuildRegistrationAccept(
	amf *amfContext.AMF,
	ue *amfContext.AmfUe,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	supportedPLMN *models.PlmnSupportItem,
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
	registrationAccept.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	registrationAccept.SetSpareHalfOctet(0)
	registrationAccept.SetMessageType(nas.MsgTypeRegistrationAccept)

	registrationAccept.RegistrationResult5GS.SetLen(1)

	registrationResult := uint8(0)
	registrationResult |= nasMessage.AccessType3GPP
	registrationAccept.SetRegistrationResultValue5GS(registrationResult)

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
		registrationAccept.SetPartialTrackingAreaIdentityList(taiListNas)
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
	if amf.Get5gsNwFeatSuppEnable() {
		registrationAccept.NetworkFeatureSupport5GS = nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
		registrationAccept.NetworkFeatureSupport5GS.SetLen(2)
		registrationAccept.SetIMSVoPS3GPP(amf.Get5gsNwFeatSuppImsVoPS())
		registrationAccept.SetEMC(amf.Get5gsNwFeatSuppEmc())
		registrationAccept.SetEMF(amf.Get5gsNwFeatSuppEmf())
		registrationAccept.SetIWKN26(amf.Get5gsNwFeatSuppIwkN26())
		registrationAccept.SetMPSI(amf.Get5gsNwFeatSuppMpsi())
		registrationAccept.SetEMCN(amf.Get5gsNwFeatSuppEmcN3())
		registrationAccept.SetMCSI(amf.Get5gsNwFeatSuppMcsi())
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
		registrationAccept.SetDRXValue(ue.UESpecificDRX)
	}

	m.RegistrationAccept = registrationAccept

	return ue.EncodeNASMessage(m)
}

// TS 24.501 - 5.4.4 Generic UE configuration update procedure - 5.4.4.1 General
func BuildConfigurationUpdateCommand(ue *amfContext.AmfUe) ([]byte, error) {
	if ue.Guti == "" {
		return nil, fmt.Errorf("5G-GUTI is required")
	}

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeConfigurationUpdateCommand)

	configurationUpdateCommand := nasMessage.NewConfigurationUpdateCommand(0)
	configurationUpdateCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	configurationUpdateCommand.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	configurationUpdateCommand.SetSpareHalfOctet(0)
	configurationUpdateCommand.SetMessageType(nas.MsgTypeConfigurationUpdateCommand)

	gutiNas, err := nasConvert.GutiToNasWithError(ue.Guti)
	if err != nil {
		return nil, fmt.Errorf("encode GUTI failed: %w", err)
	}

	configurationUpdateCommand.GUTI5G = &gutiNas
	configurationUpdateCommand.GUTI5G.SetIei(nasMessage.ConfigurationUpdateCommandGUTI5GType)

	configurationUpdateCommand.ConfigurationUpdateIndication = nasType.NewConfigurationUpdateIndication(nasMessage.ConfigurationUpdateCommandConfigurationUpdateIndicationType)

	configurationUpdateCommand.SetACK(uint8(1))

	m.ConfigurationUpdateCommand = configurationUpdateCommand

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
	}

	b, err := ue.EncodeNASMessage(m)
	if err != nil {
		return nil, fmt.Errorf("could not encode NAS message: %v", err)
	}

	return b, nil
}
