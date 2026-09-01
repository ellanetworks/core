// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type ServiceRequestOpts struct {
	ServiceType      uint8
	AMFSetID         uint16
	AMFPointer       uint8
	TMSI5G           [4]uint8
	PDUSessionStatus *[16]bool
	UESecurity       *UESecurity
}

func BuildServiceRequest(opts *ServiceRequestOpts) ([]byte, error) {
	pduFlag := uint16(0)
	for i, pduSession := range opts.PDUSessionStatus {
		pduFlag += boolToUint16(pduSession) << i
	}

	statusBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(statusBuf, pduFlag)

	stmsi := fgs.STMSIIdentity(fgs.STMSI{AMFSetID: opts.AMFSetID, AMFPointer: opts.AMFPointer, TMSI: opts.TMSI5G})

	status, err := fgs.ParsePSIBitmap(statusBuf)
	if err != nil {
		return nil, fmt.Errorf("encode PDU session status: %w", err)
	}

	m := &fgs.ServiceRequest{
		ServiceType:      fgs.ServiceType(opts.ServiceType),
		NgKSI:            nas.KeySetIdentifier{Value: uint8(opts.UESecurity.NgKsi.Ksi)},
		MobileIdentity:   stmsi,
		PDUSessionStatus: &status,
	}

	if fgs.ServiceType(opts.ServiceType) == fgs.ServiceTypeData {
		m.UplinkDataStatus = &status
	}

	// The SERVICE REQUEST replays its own IEs inside a ciphered NAS message
	// container so the AMF can recover them before validating the short MAC
	// (TS 24.501 §5.6.1.2, §4.4.6).
	innerBytes, err := m.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode inner SERVICE REQUEST: %w", err)
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

	outer := cleartextServiceRequestIEs(m)
	outer.NASMessageContainer = container

	return outer.MarshalBinary()
}

// cleartextServiceRequestIEs keeps only the IEs TS 24.501 §4.4.6 lets a SERVICE
// REQUEST carry outside the NAS message container. The PDU session status and
// uplink data status are not among them: they travel ciphered in the container
// alone, which is what the handsets on the reference network send.
func cleartextServiceRequestIEs(m *fgs.ServiceRequest) *fgs.ServiceRequest {
	return &fgs.ServiceRequest{
		ServiceType:    m.ServiceType,
		NgKSI:          m.NgKSI,
		MobileIdentity: m.MobileIdentity,
	}
}
