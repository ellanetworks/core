// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type RegistrationRequestOpts struct {
	RegistrationType  uint8
	RequestedNSSAI    fgs.NSSAI
	UplinkDataStatus  []byte
	IncludeCapability bool
	UESecurity        *UESecurity
	PDUSessionStatus  *[16]bool
}

func BuildRegistrationRequest(opts *RegistrationRequestOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("RegistrationRequestOpts is nil")
	}

	mobileIdentity := opts.UESecurity.Suci
	if opts.UESecurity.Guti != nil {
		mobileIdentity = *opts.UESecurity.Guti
	}

	m := &fgs.RegistrationRequest{
		RegistrationType: fgs.RegistrationType(opts.RegistrationType),
		FOR:              true,
		NgKSI:            nas.KeySetIdentifier{Value: uint8(opts.UESecurity.NgKsi.Ksi)},
		MobileIdentity:   mobileIdentity,
	}

	if opts.IncludeCapability {
		m.GMMCapability = &fgs.GMMCapability{RestrictEC: true, LPP: true, HOAttach: true, S1Mode: true}
	}

	m.UESecurityCapability = &opts.UESecurity.UeSecurityCapability

	if opts.RequestedNSSAI != nil {
		m.RequestedNSSAI = opts.RequestedNSSAI
	}

	pduFlag := uint16(0)

	if opts.PDUSessionStatus != nil {
		for i, pduSession := range opts.PDUSessionStatus {
			pduFlag += boolToUint16(pduSession) << i
		}
	}

	if pduFlag == 0 {
		return m.MarshalBinary()
	}

	// The UE's active PDU sessions ride in a ciphered NAS message container so the
	// AMF can recover them before the security context is established (TS 24.501
	// §5.5.1.2.2): the plain REGISTRATION REQUEST carrying the status IEs is
	// ciphered and wrapped, and the outer message drops the status IEs.
	statusBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(statusBuf, pduFlag)

	status, err := fgs.ParsePSIBitmap(statusBuf)
	if err != nil {
		return nil, fmt.Errorf("encode PDU session status: %w", err)
	}

	inner := *m
	inner.UplinkDataStatus = &status
	inner.PDUSessionStatus = &status

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

func boolToUint16(b bool) uint16 {
	if b {
		return 1
	}

	return 0
}
