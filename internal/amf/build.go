// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

// BuildDLNASTransport assembles a DL NAS TRANSPORT message. additionalInfo
// carries the Additional information IE (TS 24.501 §9.11.2.1); it is required
// for LPP payloads, where it holds the LCS correlation identifier the UE hands
// to its location services application (TS 24.501 §5.4.5.3.2 case c).
func BuildDLNASTransport(ue *UeContext, payloadContainerType uint8, nasPdu []byte, pduSessionID uint8, cause *uint8, additionalInfo []byte) ([]byte, error) {
	plain, err := (&fgs.DLNASTransport{
		PayloadContainerType: payloadContainerType,
		PayloadContainer:     nasPdu,
		PDUSessionID:         pduSessionID,
		AdditionalInfo:       additionalInfo,
		Cause:                cause,
	}).Marshal()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
}

func BuildIdentityRequest(typeOfIdentity uint8) ([]byte, error) {
	return (&fgs.IdentityRequest{IdentityType: typeOfIdentity}).Marshal()
}

// ngksiToOctet packs a UE ngKSI into the half-octet on the wire: the NAS key set
// identifier in bits 1-3 and the type-of-security-context flag in bit 4
// (TS 24.501 §9.11.3.32).
func ngksiToOctet(k models.NgKsi) uint8 {
	var tsc uint8
	if k.Tsc == models.ScTypeMapped {
		tsc = 1
	}

	return tsc<<3 | uint8(k.Ksi)
}

func BuildAuthenticationRequest(ue *UeContext) ([]byte, error) {
	conn := ue.Conn()
	if conn == nil || conn.AuthenticationCtx == nil {
		return nil, fmt.Errorf("no authentication context available")
	}

	rand, err := hex.DecodeString(conn.AuthenticationCtx.Rand)
	if err != nil {
		return nil, err
	}

	autn, err := hex.DecodeString(conn.AuthenticationCtx.Autn)
	if err != nil {
		return nil, err
	}

	var randArr, autnArr [16]byte

	copy(randArr[:], rand)
	copy(autnArr[:], autn)

	m := &fgs.AuthenticationRequest{
		NgKSI: ngksiToOctet(ue.NgKsi()),
		ABBA:  ue.Abba(),
		RAND:  &randArr,
		AUTN:  &autnArr,
	}

	return m.Marshal()
}

func BuildServiceAccept(ue *UeContext, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) ([]byte, error) {
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
	return (&fgs.AuthenticationReject{}).Marshal()
}

// T3346 Timer and EAP are not Supported
func BuildServiceReject(cause uint8) ([]byte, error) {
	return (&fgs.ServiceReject{Cause: cause}).Marshal()
}

// T3346 timer are not supported
func BuildRegistrationReject(t3502Value int, cause5GMM uint8) ([]byte, error) {
	m := &fgs.RegistrationReject{Cause: cause5GMM}

	if t3502Value != 0 {
		octet, err := fgs.EncodeGPRSTimer2(time.Duration(t3502Value) * time.Second)
		if err != nil {
			return nil, err
		}

		m.T3502 = &octet
	}

	return m.Marshal()
}

func BuildSecurityModeCommand(ue *UeContext) ([]byte, error) {
	conn := ue.Conn()
	if conn == nil {
		return nil, fmt.Errorf("no active NAS connection")
	}

	ueSecCap := ue.UESecCap()
	if ueSecCap == nil {
		return nil, fmt.Errorf("UE security capability not available, cannot build SecurityModeCommand")
	}

	imeisv := fgs.IMEISVRequested
	if ue.Imei.IsSet() {
		imeisv = fgs.IMEISVNotRequested
	}

	var addInfo uint8

	if conn.RetransmissionOfInitialNASMsg {
		addInfo |= 1 << 1 // RINMR (bit 2)
	}

	if conn.RegistrationType5GS == nasMessage.RegistrationType5GSPeriodicRegistrationUpdating ||
		conn.RegistrationType5GS == nasMessage.RegistrationType5GSMobilityRegistrationUpdating {
		addInfo |= 1 // HDP (bit 1)
	}

	plain, err := (&fgs.SecurityModeCommand{
		CipheringAlgorithm:  ue.NEA(),
		IntegrityAlgorithm:  ue.NIA(),
		NgKSI:               ngksiToOctet(ue.NgKsi()),
		ReplayedUESecCap:    ueSecCap.Buffer[:ueSecCap.GetLen()],
		IMEISVRequest:       &imeisv,
		Additional5GSecInfo: &addInfo,
	}).Marshal()
	if err != nil {
		return nil, err
	}

	ue.MarkSecured()

	payload, err := ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedNewContext))
	if err != nil {
		ue.ClearSecured()

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
	amfInstance *AMF,
	ue *UeContext,
	guti etsi.GUTI5G,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	equivalentPlmnID models.PlmnID,
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

	if guti != etsi.InvalidGUTI5G {
		gutiNas := nasConvert.GutiToNas(guti.String())
		registrationAccept.GUTI5G = &gutiNas
		registrationAccept.GUTI5G.SetIei(nasMessage.RegistrationAcceptGUTI5GType)
	}

	registrationAccept.EquivalentPlmns = nasType.NewEquivalentPlmns(nasMessage.RegistrationAcceptEquivalentPlmnsType)

	var buf []uint8

	plmnID, err := util.PlmnIDToNas(equivalentPlmnID)
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

	if len(ue.AllowedNssai) > 0 {
		registrationAccept.AllowedNSSAI = nasType.NewAllowedNSSAI(nasMessage.RegistrationAcceptAllowedNSSAIType)

		var buf []uint8

		for _, s := range ue.AllowedNssai {
			snssai, err := util.SnssaiToNas(s)
			if err != nil {
				return nil, fmt.Errorf("failed to convert SNSSAI to NAS: %s", err)
			}

			buf = append(buf, snssai...)
		}

		registrationAccept.AllowedNSSAI.SetLen(uint8(len(buf)))
		registrationAccept.AllowedNSSAI.SetSNSSAIValue(buf)
	}

	nfs := amfInstance.NetworkFeatureSupport()
	if nfs.Enable {
		registrationAccept.NetworkFeatureSupport5GS = nasType.NewNetworkFeatureSupport5GS(nasMessage.RegistrationAcceptNetworkFeatureSupport5GSType)
		registrationAccept.NetworkFeatureSupport5GS.SetLen(2)
		registrationAccept.SetIMSVoPS3GPP(nfs.ImsVoPS)
		registrationAccept.SetEMC(nfs.Emc)
		registrationAccept.SetEMF(nfs.Emf)
		registrationAccept.SetIWKN26(nfs.IwkN26)
		registrationAccept.SetMPSI(nfs.Mpsi)
		registrationAccept.SetEMCN(nfs.EmcN3)
		registrationAccept.SetMCSI(nfs.Mcsi)
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
	t3512 := nasConvert.GPRSTimer3ToNas(int(amfInstance.T3512Value.Seconds()))
	registrationAccept.T3512Value.Octet = t3512

	if ue.DRXParameter != nasMessage.DRXValueNotSpecified {
		registrationAccept.NegotiatedDRXParameters = nasType.NewNegotiatedDRXParameters(nasMessage.RegistrationAcceptNegotiatedDRXParametersType)
		registrationAccept.NegotiatedDRXParameters.SetLen(1)
		registrationAccept.SetDRXValue(ue.DRXParameter)
	}

	m.RegistrationAccept = registrationAccept

	return ue.EncodeNASMessage(m)
}

// TS 24.501 Generic UE configuration update procedure.
// includeGUTI controls whether a new 5G-GUTI is included (e.g. during service request GUTI re-allocation).
func BuildConfigurationUpdateCommand(amfInstance *AMF, ue *UeContext, guti etsi.GUTI5G, spnFullName, spnShortName string, includeGUTI bool) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeConfigurationUpdateCommand)

	configurationUpdateCommand := nasMessage.NewConfigurationUpdateCommand(0)
	configurationUpdateCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	configurationUpdateCommand.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	configurationUpdateCommand.SetSpareHalfOctet(0)
	configurationUpdateCommand.SetMessageType(nas.MsgTypeConfigurationUpdateCommand)

	if includeGUTI {
		if guti == etsi.InvalidGUTI5G {
			return nil, fmt.Errorf("5G-GUTI is required")
		}

		gutiNas, err := nasConvert.GutiToNasWithError(guti.String())
		if err != nil {
			return nil, fmt.Errorf("encode GUTI failed: %w", err)
		}

		configurationUpdateCommand.GUTI5G = &gutiNas
		configurationUpdateCommand.GUTI5G.SetIei(nasMessage.ConfigurationUpdateCommandGUTI5GType)
	}

	if spnFullName != "" {
		fullNameForNetwork := encodeNetworkName(spnFullName)
		configurationUpdateCommand.FullNameForNetwork = &nasType.FullNameForNetwork{
			Iei:    nasMessage.ConfigurationUpdateCommandFullNameForNetworkType,
			Len:    uint8(len(fullNameForNetwork)),
			Buffer: fullNameForNetwork,
		}
	}

	if spnShortName != "" {
		shortNameForNetwork := encodeNetworkName(spnShortName)
		configurationUpdateCommand.ShortNameForNetwork = &nasType.ShortNameForNetwork{
			Iei:    nasMessage.ConfigurationUpdateCommandShortNameForNetworkType,
			Len:    uint8(len(shortNameForNetwork)),
			Buffer: shortNameForNetwork,
		}
	}

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

// encodeNetworkName encodes a network name string into the format defined by
// TS 24.008 (Network Name IE). It uses the GSM 7-bit default
// alphabet with no CI appended.
func encodeNetworkName(name string) []byte {
	chars := len(name)
	// GSM 7-bit packing: ceil(chars * 7 / 8) bytes for the text
	packedLen := (chars*7 + 7) / 8
	spareBits := uint8(packedLen*8 - chars*7)

	buf := make([]byte, 1+packedLen)
	// Byte 0: ext=1 (bit 7), coding scheme=0 (bits 6-4), addCI=0 (bit 3), spare bits (bits 2-0)
	buf[0] = 0x80 | (spareBits & 0x07)

	// Pack 7-bit characters into octets (TS 23.038)
	bitOffset := 0

	for i := range chars {
		c := name[i] & 0x7F
		bytePos := bitOffset / 8
		bitPos := bitOffset % 8

		buf[1+bytePos] |= c << uint(bitPos)
		if bitPos > 1 {
			buf[1+bytePos+1] |= c >> uint(8-bitPos)
		}

		bitOffset += 7
	}

	return buf
}

func BuildStatus5GMM(cause uint8) ([]byte, error) {
	return (&fgs.Status5GMM{Cause: cause}).Marshal()
}
