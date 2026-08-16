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
}

func BuildRegistrationRequest(opts *RegistrationRequestOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("RegistrationRequestOpts is nil")
	}

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

	status, haveStatus, err := psiBitmap(opts.PDUSessionStatus)
	if err != nil {
		return nil, fmt.Errorf("encode PDU session status: %w", err)
	}

	uplink, haveUplink, err := psiBitmap(opts.UplinkDataStatus)
	if err != nil {
		return nil, fmt.Errorf("encode uplink data status: %w", err)
	}

	if !haveStatus && !haveUplink {
		return m.MarshalBinary()
	}

	inner := *m

	if haveStatus {
		inner.PDUSessionStatus = &status
	}

	if haveUplink {
		inner.UplinkDataStatus = &uplink
	}

	innerBytes, err := inner.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode inner REGISTRATION REQUEST: %w", err)
	}

	sc, err := securityContext(opts.UESecurity.IntegrityAlg, opts.UESecurity.CipheringAlg,
		opts.UESecurity.KnasInt, opts.UESecurity.KnasEnc)
	if err != nil {
		return nil, err
	}

	container, err := sc.Cipher(innerBytes, opts.UESecurity.ULCount, nas.Bearer3GPP, nas.DirectionUplink)
	if err != nil {
		return nil, fmt.Errorf("error encrypting NAS message: %w", err)
	}

	m.NASMessageContainer = container

	return m.MarshalBinary()
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
