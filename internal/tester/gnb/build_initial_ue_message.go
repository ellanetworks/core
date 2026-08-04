// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type InitialUEMessageOpts struct {
	RanUENGAPID           int64
	NasPDU                []byte
	Guti5g                []byte
	Mcc                   string
	Mnc                   string
	Tac                   string
	GnbID                 string
	RRCEstablishmentCause ngap.RRCEstablishmentCause
}

func BuildInitialUEMessage(opts *InitialUEMessageOpts) ([]byte, error) {
	if opts.Mcc == "" {
		return nil, fmt.Errorf("MCC is required to build InitialUEMessage")
	}

	if opts.Mnc == "" {
		return nil, fmt.Errorf("MNC is required to build InitialUEMessage")
	}

	if opts.Tac == "" {
		return nil, fmt.Errorf("TAC is required to build InitialUEMessage")
	}

	if opts.GnbID == "" {
		return nil, fmt.Errorf("GNB ID is required to build InitialUEMessage")
	}

	if opts.NasPDU == nil {
		return nil, fmt.Errorf("NAS PDU is required to build InitialUEMessage")
	}

	if opts.RanUENGAPID == 0 {
		return nil, fmt.Errorf("RAN UE NGAP ID is required to build InitialUEMessage")
	}

	plmn, err := PLMNIdentity(opts.Mcc, opts.Mnc)
	if err != nil {
		return nil, err
	}

	tac, err := TACValue(opts.Tac)
	if err != nil {
		return nil, err
	}

	node, err := GNBNodeID(opts.Mcc, opts.Mnc, opts.GnbID)
	if err != nil {
		return nil, err
	}

	cellID, err := node.NRCellIdentity(0)
	if err != nil {
		return nil, fmt.Errorf("could not build NR cell identity: %w", err)
	}

	msg := &ngap.InitialUEMessage{
		RANUENGAPID: ngap.RANUENGAPID(opts.RanUENGAPID),
		NASPDU:      ngap.NASPDU(opts.NasPDU),
		UserLocationInformation: ngap.UserLocationInformation{
			Kind:         ngap.UserLocationNR,
			PLMNIdentity: plmn,
			CellIdentity: cellID,
			TAI:          ngap.TAI{PLMNIdentity: plmn, TAC: tac},
		},
		RRCEstablishmentCause: ngap.Ptr(opts.RRCEstablishmentCause),
		UEContextRequest:      ngap.Ptr(ngap.UEContextRequested),
	}

	// The AMF Set ID (10 bits) and AMF Pointer (6 bits) sit in octets 6-7 of the
	// 5G-GUTI value, and the 5G-TMSI in octets 8-11 (TS 24.501 §9.11.3.4).
	if len(opts.Guti5g) >= 11 {
		msg.FiveGSTMSI = &ngap.FiveGSTMSI{
			AMFSetID:   ngap.AMFSetID(uint16(opts.Guti5g[5])<<2 | uint16(opts.Guti5g[6])>>6),
			AMFPointer: ngap.AMFPointer(opts.Guti5g[6] & 0x3f),
			FiveGTMSI:  ngap.FiveGTMSI(binary.BigEndian.Uint32(opts.Guti5g[7:11])),
		}
	}

	return msg.Marshal()
}

// PLMNIdentity encodes MCC/MNC as the three TBCD octets of a PLMN Identity.
func PLMNIdentity(mcc, mnc string) (ngap.PLMNIdentity, error) {
	b, err := GetMccAndMncInOctets(mcc, mnc)
	if err != nil {
		return ngap.PLMNIdentity{}, fmt.Errorf("failed to get plmnID: %w", err)
	}

	if len(b) != 3 {
		return ngap.PLMNIdentity{}, fmt.Errorf("PLMN identity is %d octets, want 3", len(b))
	}

	return ngap.PLMNIdentity{b[0], b[1], b[2]}, nil
}

// TACValue decodes the three-octet NR tracking area code.
func TACValue(s string) (ngap.TAC, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return 0, fmt.Errorf("could not decode tac to bytes: %w", err)
	}

	if len(b) != 3 {
		return 0, fmt.Errorf("TAC %q is %d octets, want 3", s, len(b))
	}

	return ngap.TAC(uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])), nil
}

// GNBNodeID builds this simulator's Global RAN Node ID, whose gNB-ID width the
// library needs to place the node in a cell identity.
func GNBNodeID(mcc, mnc, gnbID string) (ngap.GlobalRANNodeID, error) {
	plmn, err := PLMNIdentity(mcc, mnc)
	if err != nil {
		return ngap.GlobalRANNodeID{}, err
	}

	b, err := GetGnbIdInBytes(gnbID)
	if err != nil {
		return ngap.GlobalRANNodeID{}, fmt.Errorf("could not decode gNB id: %w", err)
	}

	var v uint64
	for _, o := range b {
		v = v<<8 | uint64(o)
	}

	return ngap.GlobalRANNodeID{
		Kind: ngap.RANNodeIDGNB, PLMNIdentity: plmn, Value: uint32(v), Bits: gnbIDBits,
	}, nil
}
