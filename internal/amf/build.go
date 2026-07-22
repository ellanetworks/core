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
	"github.com/free5gc/nas/nasMessage"
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
	m := &fgs.ServiceAccept{}

	if pDUSessionStatus != nil {
		m.PDUSessionStatus = fgs.PSIToBytes(*pDUSessionStatus)
	}

	if reactivationResult != nil {
		m.PDUSessionReactivationResult = fgs.PSIToBytes(*reactivationResult)
	}

	if errPduSessionID != nil {
		m.ReactivationResultErrorCause = reactivationErrCauseToBytes(errPduSessionID, errCause)
	}

	plain, err := m.Marshal()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
}

// reactivationErrCauseToBytes interleaves PDU session identities with their 5GSM
// cause into the reactivation-result error cause IE value (TS 24.501 §9.11.3.43),
// returning an empty (non-nil) value on a length mismatch, as the IE is still
// emitted when the caller supplies error PDU session identities.
func reactivationErrCauseToBytes(ids, causes []uint8) []byte {
	buf := []byte{}

	if len(ids) == len(causes) {
		for i := range ids {
			buf = append(buf, ids[i], causes[i])
		}
	}

	return buf
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
		octet, err := fgs.EncodeGPRSTimer(time.Duration(t3502Value) * time.Second)
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
	return (&fgs.DeregistrationAcceptUEOriginating{}).Marshal()
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
	equivalentPlmn, err := util.PlmnIDToNas(equivalentPlmnID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert PLMN ID to NAS: %s", err)
	}

	t3512 := fgs.EncodeGPRSTimer3(amfInstance.T3512Value)

	m := &fgs.RegistrationAccept{
		RegistrationResult: fgs.AccessType3GPP,
		EquivalentPlmns:    equivalentPlmn,
		T3512Value:         &t3512,
	}

	if guti != etsi.InvalidGUTI5G {
		gutiNas, err := guti.MobileIdentity()
		if err != nil {
			return nil, fmt.Errorf("failed to encode GUTI: %s", err)
		}

		m.GUTI = gutiNas
	}

	if len(ue.RegistrationArea) > 0 {
		taiListNas, err := util.TaiListToNas(ue.RegistrationArea)
		if err != nil {
			return nil, fmt.Errorf("failed to convert TAI list to NAS: %s", err)
		}

		m.TAIList = taiListNas
	}

	if len(ue.AllowedNssai) > 0 {
		var buf []uint8

		for _, s := range ue.AllowedNssai {
			snssai, err := util.SnssaiToNas(s)
			if err != nil {
				return nil, fmt.Errorf("failed to convert SNSSAI to NAS: %s", err)
			}

			buf = append(buf, snssai...)
		}

		m.AllowedNSSAI = buf
	}

	if nfs := amfInstance.NetworkFeatureSupport(); nfs.Enable {
		octet0 := (nfs.Mpsi&1)<<7 | (nfs.IwkN26&1)<<6 | (nfs.Emf&3)<<4 | (nfs.Emc&3)<<2 | (nfs.ImsVoPS & 1)
		octet1 := (nfs.Mcsi&1)<<1 | (nfs.EmcN3 & 1)
		m.NetworkFeatureSupport = []byte{octet0, octet1}
	}

	if pDUSessionStatus != nil {
		m.PDUSessionStatus = fgs.PSIToBytes(*pDUSessionStatus)
	}

	if reactivationResult != nil {
		m.PDUSessionReactivationResult = fgs.PSIToBytes(*reactivationResult)
	}

	if errPduSessionID != nil {
		m.ReactivationResultErrorCause = reactivationErrCauseToBytes(errPduSessionID, errCause)
	}

	if ue.DRXParameter != nasMessage.DRXValueNotSpecified {
		drx := ue.DRXParameter
		m.NegotiatedDRX = &drx
	}

	plain, err := m.Marshal()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
}

// TS 24.501 Generic UE configuration update procedure.
// includeGUTI controls whether a new 5G-GUTI is included (e.g. during service request GUTI re-allocation).
func BuildConfigurationUpdateCommand(amfInstance *AMF, ue *UeContext, guti etsi.GUTI5G, spnFullName, spnShortName string, includeGUTI bool) ([]byte, error) {
	ack := uint8(1) // configuration update indication: ACK requested

	m := &fgs.ConfigurationUpdateCommand{ConfigurationUpdateIndication: &ack}

	if includeGUTI {
		if guti == etsi.InvalidGUTI5G {
			return nil, fmt.Errorf("5G-GUTI is required")
		}

		gutiNas, err := guti.MobileIdentity()
		if err != nil {
			return nil, fmt.Errorf("encode GUTI failed: %w", err)
		}

		m.GUTI = gutiNas
	}

	if spnFullName != "" {
		m.FullNameForNetwork = encodeNetworkName(spnFullName)
	}

	if spnShortName != "" {
		m.ShortNameForNetwork = encodeNetworkName(spnShortName)
	}

	plain, err := m.Marshal()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
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
	return (&fgs.GMMStatus{Cause: cause}).Marshal()
}
