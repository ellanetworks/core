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
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// BuildDLNASTransport assembles a DL NAS TRANSPORT message. additionalInfo
// carries the Additional information IE (TS 24.501 §9.11.2.1); it is required
// for LPP payloads, where it holds the LCS correlation identifier the UE hands
// to its location services application (TS 24.501 §5.4.5.3.2 case c).
func BuildDLNASTransport(ue *UeContext, payloadContainerType fgs.PayloadContainerType, nasPdu []byte, pduSessionID *fgs.PDUSessionID, cause *fgs.GMMCause, additionalInfo []byte) ([]byte, error) {
	plain, err := (&fgs.DLNASTransport{
		PayloadContainerType: payloadContainerType,
		PayloadContainer:     nasPdu,
		PDUSessionID:         pduSessionID,
		AdditionalInfo:       additionalInfo,
		Cause:                cause,
	}).MarshalBinary()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
}

func BuildIdentityRequest(typeOfIdentity fgs.MobileIdentityType) ([]byte, error) {
	return (&fgs.IdentityRequest{IdentityType: typeOfIdentity}).MarshalBinary()
}

// ngKsi maps a UE ngKSI to the NAS key set identifier the wire carries
// (TS 24.501 §9.11.3.32).
func ngKsi(k models.NgKsi) nas.KeySetIdentifier {
	return nas.KeySetIdentifier{Value: uint8(k.Ksi), Mapped: k.Tsc == models.ScTypeMapped}
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

	ngksi := ue.NgKsi()

	m := &fgs.AuthenticationRequest{
		NgKSI: ngKsi(ngksi),
		ABBA:  ue.Abba(),
		RAND:  &randArr,
		AUTN:  &autnArr,
	}

	return m.MarshalBinary()
}

func BuildServiceAccept(ue *UeContext, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) ([]byte, error) {
	m := &fgs.ServiceAccept{}

	if pDUSessionStatus != nil {
		m.PDUSessionStatus = &fgs.PSIBitmap{PSI: *pDUSessionStatus}
	}

	if reactivationResult != nil {
		m.PDUSessionReactivationResult = &fgs.PSIBitmap{PSI: *reactivationResult}
	}

	if errPduSessionID != nil {
		m.ReactivationResultErrorCause = reactivationErrCause(errPduSessionID, errCause)
	}

	plain, err := m.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
}

// reactivationErrCause pairs each PDU session identity with its 5GSM cause into
// the reactivation-result error cause IE value (TS 24.501 §9.11.3.43). A length
// mismatch is a caller error with no correct encoding, so it returns nil and the
// message omits the element.
func reactivationErrCause(ids, causes []uint8) fgs.ReactivationResultErrorCause {
	if len(ids) != len(causes) {
		return nil
	}

	out := make(fgs.ReactivationResultErrorCause, 0, len(ids))
	for i := range ids {
		out = append(out, fgs.ReactivationError{
			PDUSessionID: fgs.PDUSessionID(ids[i]),
			Cause:        fgs.GSMCause(causes[i]),
		})
	}

	return out
}

func BuildAuthenticationReject() ([]byte, error) {
	return (&fgs.AuthenticationReject{}).MarshalBinary()
}

// T3346 Timer and EAP are not Supported
func BuildServiceReject(cause fgs.GMMCause) ([]byte, error) {
	return (&fgs.ServiceReject{Cause: cause}).MarshalBinary()
}

// T3346 timer are not supported
func BuildRegistrationReject(t3502Value int, cause5GMM fgs.GMMCause) ([]byte, error) {
	m := &fgs.RegistrationReject{Cause: cause5GMM}

	if t3502Value != 0 {
		timer, err := nas.GPRSTimer2FromDuration(time.Duration(t3502Value) * time.Second)
		if err != nil {
			return nil, err
		}

		m.T3502 = &timer
	}

	return m.MarshalBinary()
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

	imeisv := fgs.IMEISVNotRequested
	if !ue.Imei.IsSet() {
		imeisv = fgs.IMEISVRequested
	}

	addInfo := fgs.AdditionalSecurityInformation{
		RINMR: conn.RetransmissionOfInitialNASMsg,
		HDP: conn.RegistrationType5GS == fgs.RegistrationTypePeriodicUpdating ||
			conn.RegistrationType5GS == fgs.RegistrationTypeMobilityUpdating,
	}

	ngksi := ue.NgKsi()

	smc := &fgs.SecurityModeCommand{
		CipheringAlgorithm:            ue.NEA(),
		IntegrityAlgorithm:            ue.NIA(),
		NgKSI:                         ngKsi(ngksi),
		ReplayedUESecurityCapability:  *ueSecCap,
		IMEISVRequested:               &imeisv,
		AdditionalSecurityInformation: &addInfo,
	}

	addEPSNASSecurityAlgorithms(ue, smc)

	plain, err := smc.MarshalBinary()
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

func BuildEPSNASAlgorithmsSecurityModeCommand(ue *UeContext) ([]byte, error) {
	ueSecCap := ue.UESecCap()
	if ueSecCap == nil {
		return nil, fmt.Errorf("UE security capability not available, cannot build SecurityModeCommand")
	}

	smc := &fgs.SecurityModeCommand{
		CipheringAlgorithm:           ue.NEA(),
		IntegrityAlgorithm:           ue.NIA(),
		NgKSI:                        ngKsi(ue.NgKsi()),
		ReplayedUESecurityCapability: *ueSecCap,
	}

	addEPSNASSecurityAlgorithms(ue, smc)

	if smc.SelectedEPSNASSecurityAlgorithms == nil {
		return nil, fmt.Errorf("no EPS NAS security algorithms selected, cannot build SecurityModeCommand")
	}

	plain, err := smc.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtected))
}

func addEPSNASSecurityAlgorithms(ue *UeContext, smc *fgs.SecurityModeCommand) {
	offered, ok := ue.offeredEPSNASAlgorithms()
	if !ok {
		return
	}

	replayed, ok := ue.EPSSecurityCapability()
	if !ok {
		return
	}

	encoded, err := replayed.MarshalBinary()
	if err != nil {
		return
	}

	smc.SelectedEPSNASSecurityAlgorithms = &fgs.SelectedEPSNASSecurityAlgorithms{
		Ciphering: offered.Ciphering,
		Integrity: offered.Integrity,
	}
	smc.ReplayedS1UESecurityCapability = encoded
}

func BuildDeregistrationAccept() ([]byte, error) {
	return (&fgs.DeregistrationAcceptUEOriginating{}).MarshalBinary()
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
	equivalentPLMNs := nas.PLMNList{{MCC: equivalentPlmnID.Mcc, MNC: equivalentPlmnID.Mnc}}

	t3512, err := nas.GPRSTimer3FromDuration(amfInstance.T3512Value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode T3512: %w", err)
	}

	m := &fgs.RegistrationAccept{
		RegistrationResult: fgs.RegistrationResult3GPP,
		EquivalentPLMNs:    equivalentPLMNs,
		T3512:              &t3512,
	}

	if guti != etsi.InvalidGUTI5G {
		gutiNas, err := guti.MobileIdentity()
		if err != nil {
			return nil, fmt.Errorf("failed to encode GUTI: %s", err)
		}

		m.GUTI = &gutiNas
	}

	if len(ue.RegistrationArea) > 0 {
		taiListNas, err := util.TaiListToNas(ue.RegistrationArea)
		if err != nil {
			return nil, fmt.Errorf("failed to convert TAI list to NAS: %s", err)
		}

		m.TAIList = &taiListNas
	}

	for _, s := range ue.AllowedNssai {
		snssai, err := util.SnssaiToNas(s)
		if err != nil {
			return nil, fmt.Errorf("failed to convert SNSSAI to NAS: %s", err)
		}

		m.AllowedNSSAI = append(m.AllowedNSSAI, snssai)
	}

	if nfs := amfInstance.NetworkFeatureSupport(); nfs.Enable {
		m.NetworkFeatureSupport = &fgs.NetworkFeatureSupport{
			IMSVoPS3GPP: nfs.ImsVoPS != 0,
			EMC:         nfs.Emc,
			EMF:         nfs.Emf,
			IWKN26:      ue.SupportsS1Mode() && !amfInstance.N26Enabled,
			MPSI:        nfs.Mpsi != 0,
			HasOctet4:   true,
			EMCN3:       nfs.EmcN3 != 0,
			MCSI:        nfs.Mcsi != 0,
		}
	}

	if pDUSessionStatus != nil {
		m.PDUSessionStatus = &fgs.PSIBitmap{PSI: *pDUSessionStatus}
	}

	if reactivationResult != nil {
		m.PDUSessionReactivationResult = &fgs.PSIBitmap{PSI: *reactivationResult}
	}

	if errPduSessionID != nil {
		m.ReactivationResultErrorCause = reactivationErrCause(errPduSessionID, errCause)
	}

	if ue.DRXParameter != fgs.DRXValueNotSpecified {
		m.NegotiatedDRX = &fgs.DRXParameter{Value: ue.DRXParameter}
	}

	plain, err := m.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
}

// TS 24.501 Generic UE configuration update procedure.
// includeGUTI controls whether a new 5G-GUTI is included (e.g. during service request GUTI re-allocation).
func BuildConfigurationUpdateCommand(amfInstance *AMF, ue *UeContext, guti etsi.GUTI5G, spnFullName, spnShortName string, includeGUTI bool) ([]byte, error) {
	ack := fgs.ConfigurationUpdateIndication{ACK: true}

	m := &fgs.ConfigurationUpdateCommand{ConfigurationUpdateIndication: &ack}

	if includeGUTI {
		if guti == etsi.InvalidGUTI5G {
			return nil, fmt.Errorf("5G-GUTI is required")
		}

		gutiNas, err := guti.MobileIdentity()
		if err != nil {
			return nil, fmt.Errorf("encode GUTI failed: %w", err)
		}

		m.GUTI = &gutiNas
	}

	if spnFullName != "" {
		m.FullNameForNetwork = new(nas.NewNetworkName(spnFullName))
	}

	if spnShortName != "" {
		m.ShortNameForNetwork = new(nas.NewNetworkName(spnShortName))
	}

	plain, err := m.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return ue.EncodeNASMessagePlain(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
}

func BuildStatus5GMM(cause fgs.GMMCause) ([]byte, error) {
	return (&fgs.GMMStatus{Cause: cause}).MarshalBinary()
}
