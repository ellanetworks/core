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
	FollowOnRequest        bool
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
	InitialNASMessage      bool
}

func BuildRegistrationRequest(opts *RegistrationRequestOpts) ([]byte, error) {
	wire, _, err := buildRegistrationRequest(opts)

	return wire, err
}

func buildRegistrationRequest(opts *RegistrationRequestOpts) (wire, complete []byte, err error) {
	if opts == nil {
		return nil, nil, fmt.Errorf("RegistrationRequestOpts is nil")
	}

	m, err := registrationRequestMessage(opts)
	if err != nil {
		return nil, nil, err
	}

	complete, err = m.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	if !opts.InitialNASMessage || !hasNonCleartextIEs(m) {
		return complete, complete, nil
	}

	if opts.UESecurity.NgKsi.Ksi == ngKSINoKey {
		wire, err = cleartextRegistrationIEs(m).MarshalBinary()

		return wire, complete, err
	}

	wire, err = protectInitialRegistrationRequest(m, opts.UESecurity)

	return wire, complete, err
}

func hasNonCleartextIEs(m *fgs.RegistrationRequest) bool {
	return m.GMMCapability != nil ||
		m.S1UENetworkCapability != nil ||
		m.RequestedNSSAI != nil ||
		m.PDUSessionStatus != nil ||
		m.UplinkDataStatus != nil
}

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
		FOR:              opts.FollowOnRequest,
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
