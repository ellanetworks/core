// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type RegistrationRequestOpts struct {
	RegistrationType       uint8
	RequestedNSSAI         fgs.NSSAI
	IncludeCapability      bool
	UESecurity             *UESecurity
	PDUSessionStatus       *[16]bool
	UplinkDataStatus       *[16]bool
	S1UENetworkCapability  []byte
	UEStatus               *fgs.UEStatus
	EPSNASMessageContainer []byte
	AdditionalGUTI         *fgs.MobileIdentity
	MobileIdentity         *fgs.MobileIdentity

	// ProtectAsInitialNASMessage sends the message the way a UE with a valid 5G
	// NAS security context sends an initial NAS message: cleartext IEs on the
	// wire, everything else in a ciphered NAS message container (TS 24.501
	// §4.4.6 b). Leave it clear to build the complete message, which is what
	// goes in the container of SECURITY MODE COMPLETE, and what a UE with no
	// security context to protect with sends as-is.
	ProtectAsInitialNASMessage bool
}

func BuildRegistrationRequest(opts *RegistrationRequestOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("RegistrationRequestOpts is nil")
	}

	m, err := registrationRequestMessage(opts)
	if err != nil {
		return nil, err
	}

	if !opts.ProtectAsInitialNASMessage {
		return m.MarshalBinary()
	}

	return protectInitialRegistrationRequest(m, opts.UESecurity)
}

// registrationRequestMessage builds the complete REGISTRATION REQUEST: every IE
// the options ask for, cleartext and non-cleartext alike.
func registrationRequestMessage(opts *RegistrationRequestOpts) (*fgs.RegistrationRequest, error) {
	mobileIdentity := opts.UESecurity.Suci
	if opts.UESecurity.Guti != nil {
		mobileIdentity = *opts.UESecurity.Guti
	}

	if opts.MobileIdentity != nil {
		mobileIdentity = *opts.MobileIdentity
	}

	m := &fgs.RegistrationRequest{
		RegistrationType: fgs.RegistrationType(opts.RegistrationType),
		FOR:              true,
		NgKSI: nas.KeySetIdentifier{
			Value:  uint8(opts.UESecurity.NgKsi.Ksi),
			Mapped: opts.UESecurity.NgKsi.Tsc == models.ScTypeMapped,
		},
		MobileIdentity:         mobileIdentity,
		UEStatus:               opts.UEStatus,
		EPSNASMessageContainer: opts.EPSNASMessageContainer,
		AdditionalGUTI:         opts.AdditionalGUTI,
	}

	if opts.IncludeCapability {
		m.GMMCapability = &fgs.GMMCapability{RestrictEC: true, LPP: true, HOAttach: true, S1Mode: true}
	}

	m.UESecurityCapability = &opts.UESecurity.UeSecurityCapability
	m.S1UENetworkCapability = opts.S1UENetworkCapability

	if opts.RequestedNSSAI != nil {
		m.RequestedNSSAI = opts.RequestedNSSAI
	}

	// TS 24.501 §5.5.1.2.2
	status, haveStatus, err := psiBitmap(opts.PDUSessionStatus)
	if err != nil {
		return nil, fmt.Errorf("encode PDU session status: %w", err)
	}

	if haveStatus {
		m.PDUSessionStatus = &status
	}

	uplink, haveUplink, err := psiBitmap(opts.UplinkDataStatus)
	if err != nil {
		return nil, fmt.Errorf("encode uplink data status: %w", err)
	}

	if haveUplink {
		m.UplinkDataStatus = &uplink
	}

	return m, nil
}

// protectInitialRegistrationRequest splits the complete message the way TS 24.501
// §4.4.6 b)1) requires of a UE that holds a valid 5G NAS security context: the
// cleartext IEs travel in the clear, and the entire message travels beside them in
// a NAS message container whose value part is ciphered.
//
// A real UE sends no non-cleartext IE outside the container, so neither does this
// one. Leaving a copy in the clear would let the AMF read the 5GMM capability, the
// PDU session status and the rest without ever decrypting the container, and a core
// that only ever worked because of that would fail against real handsets.
func protectInitialRegistrationRequest(complete *fgs.RegistrationRequest, sec *UESecurity) ([]byte, error) {
	inner, err := complete.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode the complete REGISTRATION REQUEST: %w", err)
	}

	sc, err := securityContext(sec.IntegrityAlg, sec.CipheringAlg, sec.KnasInt, sec.KnasEnc)
	if err != nil {
		return nil, err
	}

	container, err := sc.Cipher(inner, sec.ULCount, nas.Bearer3GPP, nas.DirectionUplink)
	if err != nil {
		return nil, fmt.Errorf("error encrypting NAS message: %w", err)
	}

	outer := cleartextRegistrationIEs(complete)
	outer.NASMessageContainer = container

	return outer.MarshalBinary()
}

// cleartextRegistrationIEs returns the IEs TS 24.501 §4.4.6 lists as the cleartext
// IEs of a REGISTRATION REQUEST. The registration type, ngKSI and mobile identity
// are part of the message rather than optional IEs, so they are always carried.
func cleartextRegistrationIEs(m *fgs.RegistrationRequest) *fgs.RegistrationRequest {
	return &fgs.RegistrationRequest{
		RegistrationType:       m.RegistrationType,
		FOR:                    m.FOR,
		NgKSI:                  m.NgKSI,
		MobileIdentity:         m.MobileIdentity,
		UESecurityCapability:   m.UESecurityCapability,
		AdditionalGUTI:         m.AdditionalGUTI,
		UEStatus:               m.UEStatus,
		EPSNASMessageContainer: m.EPSNASMessageContainer,
	}
}

func psiBitmap(sessions *[16]bool) (fgs.PSIBitmap, bool, error) {
	var none fgs.PSIBitmap

	if sessions == nil {
		return none, false, nil
	}

	flag := uint16(0)

	for i, set := range sessions {
		flag += boolToUint16(set) << i
	}

	if flag == 0 {
		return none, false, nil
	}

	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, flag)

	bitmap, err := fgs.ParsePSIBitmap(buf)
	if err != nil {
		return none, false, err
	}

	return bitmap, true, nil
}

func boolToUint16(b bool) uint16 {
	if b {
		return 1
	}

	return 0
}
