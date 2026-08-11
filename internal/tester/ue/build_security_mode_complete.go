// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

type SecurityModeCompleteOpts struct {
	UESecurity       *UESecurity
	IMEISV           string
	PDUSessionStatus *[16]bool
}

func BuildSecurityModeComplete(opts *SecurityModeCompleteOpts) ([]byte, error) {
	regReqOpts := &RegistrationRequestOpts{
		RegistrationType:  uint8(fgs.RegistrationTypeInitial),
		RequestedNSSAI:    nil,
		UplinkDataStatus:  nil,
		IncludeCapability: true,
		UESecurity:        opts.UESecurity,
		PDUSessionStatus:  opts.PDUSessionStatus,
	}

	if opts.UESecurity.S1UENetworkCapability != nil {
		s1, err := opts.UESecurity.S1UENetworkCapability.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("encode S1 UE network capability: %w", err)
		}

		regReqOpts.S1UENetworkCapability = s1
	}

	registrationRequest, err := BuildRegistrationRequest(regReqOpts)
	if err != nil {
		return nil, fmt.Errorf("error encoding %s IMSI UE  NAS Registration Request message: %v", opts.UESecurity.Supi, err)
	}

	imeisv := fgs.PEIIdentity(fgs.PEI{Type: fgs.IdentityIMEISV, Digits: opts.IMEISV})
	if !imeisv.PEI.Valid() {
		return nil, fmt.Errorf("IMEISV must be 16 digits, got %d", len(opts.IMEISV))
	}

	m := &fgs.SecurityModeComplete{
		IMEISV:              &imeisv,
		NASMessageContainer: registrationRequest,
	}

	pdu, err := m.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("error encoding %s IMSI UE  NAS Security Mode Complete message: %v", opts.UESecurity.Supi, err)
	}

	return pdu, nil
}

// marshalIMEISV encodes a 16-digit IMEISV as an IMEISV 5GS mobile identity value
// (type 5, TS 24.501 §9.11.3.4): the first octet packs identity digit 1 and the
// type; each following octet packs two BCD digits, and the unused final high
// nibble is filled with 0xF.
