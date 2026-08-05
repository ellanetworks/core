// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enb

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
)

type InitialUEMessageOpts struct {
	RanUENGAPID           int64
	NasPDU                []byte
	Guti5g                []byte
	Mcc                   string
	Mnc                   string
	Tac                   string
	EnbID                 string
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

	if opts.EnbID == "" {
		return nil, fmt.Errorf("ENB ID is required to build InitialUEMessage")
	}

	if opts.NasPDU == nil {
		return nil, fmt.Errorf("NAS PDU is required to build InitialUEMessage")
	}

	if opts.RanUENGAPID == 0 {
		return nil, fmt.Errorf("RAN UE NGAP ID is required to build InitialUEMessage")
	}

	plmn, err := gnb.PLMNIdentity(opts.Mcc, opts.Mnc)
	if err != nil {
		return nil, err
	}

	tac, err := gnb.TACValue(opts.Tac)
	if err != nil {
		return nil, err
	}

	node, err := NgENBNodeID(opts.Mcc, opts.Mnc, opts.EnbID)
	if err != nil {
		return nil, err
	}

	cellID, err := node.EUTRACellIdentity(0)
	if err != nil {
		return nil, fmt.Errorf("could not build E-UTRA cell identity: %w", err)
	}

	msg := &ngap.InitialUEMessage{
		RANUENGAPID: ngap.RANUENGAPID(opts.RanUENGAPID),
		NASPDU:      ngap.NASPDU(opts.NasPDU),
		UserLocationInformation: ngap.UserLocationInformation{
			Kind:         ngap.UserLocationEUTRA,
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

// NgENBNodeID builds this simulator's Global RAN Node ID, whose ng-eNB-ID width
// the library needs to place the node in a cell identity.
func NgENBNodeID(mcc, mnc, enbID string) (ngap.GlobalRANNodeID, error) {
	plmn, err := gnb.PLMNIdentity(mcc, mnc)
	if err != nil {
		return ngap.GlobalRANNodeID{}, err
	}

	b, err := gnb.GetGnbIdInBytes(enbID)
	if err != nil {
		return ngap.GlobalRANNodeID{}, fmt.Errorf("could not decode ng-eNB id: %w", err)
	}

	var v uint64
	for _, o := range b {
		v = v<<8 | uint64(o)
	}

	// TS 38.413 §9.3.1.8: the Macro ng-eNB ID is "the 20 leftmost bits of the
	// E-UTRA Cell Identity". BuildNGSetupRequest reads the configured hex string
	// the same way — the top ngENBIDBits of its octets — so the node this
	// announces and the node the cell identity below belongs to are the same one.
	if 8*len(b) < ngENBIDBits {
		return ngap.GlobalRANNodeID{}, fmt.Errorf("ng-eNB id %q is %d octets, too few for a %d-bit node id", enbID, len(b), ngENBIDBits)
	}

	return ngap.GlobalRANNodeID{
		Kind: ngap.RANNodeIDMacroNgENB, PLMNIdentity: plmn,
		Value: uint32(v >> uint(8*len(b)-ngENBIDBits)), Bits: ngENBIDBits,
	}, nil
}
