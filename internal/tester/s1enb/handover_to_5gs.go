// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/s1ap"
)

const (
	fcKeNB    = "11"
	fcNextHop = "12"
)

type HandoverToFiveGSOpts struct {
	ENBUEID int64
	MMEUEID int64

	TargetGNBID     uint32
	TargetGNBIDBits int
	TargetTAC       uint32

	Cause *s1ap.Cause
}

func (e *ENB) SendHandoverRequiredToFiveGS(opts *HandoverToFiveGSOpts) error {
	if opts == nil {
		return fmt.Errorf("s1enb: HandoverToFiveGSOpts is nil")
	}

	cause := opts.Cause
	if cause == nil {
		cause = s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 16})
	}

	req := &s1ap.HandoverRequired{
		MMEUES1APID:  s1ap.MMEUES1APID(opts.MMEUEID),
		ENBUES1APID:  s1ap.ENBUES1APID(opts.ENBUEID),
		HandoverType: s1ap.HandoverTypeEPSToFiveGS,
		Cause:        cause,
		TargetID: s1ap.TargetID{TargetNgRanNodeID: &s1ap.TargetNgRanNodeID{
			GlobalRANNodeID: s1ap.GlobalRANNodeID{GNB: &s1ap.GNB{GlobalGNBID: s1ap.GlobalGNBID{
				PLMNIdentity: e.plmn,
				GNBID:        s1ap.GNBID{Value: opts.TargetGNBID, Bits: opts.TargetGNBIDBits},
			}}},
			SelectedTAI: s1ap.FiveGSTAI{PLMNIdentity: e.plmn, TAC: s1ap.FiveGSTAC(opts.TargetTAC)},
		}},
		SourceToTarget: s1ap.TransparentContainer{0x00},
	}

	b, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build Handover Required to 5GS: %w", err)
	}

	return e.SendMessage(b, true)
}

func (e *ENB) WaitForHandoverPreparationFailure(enbUEID int64, timeout time.Duration) (*s1ap.HandoverPreparationFailure, error) {
	f, err := e.WaitForMessage(enbUEID, Unsuccessful, s1ap.ProcHandoverPreparation, timeout)
	if err != nil {
		return nil, err
	}

	fail, err := s1ap.ParseHandoverPreparationFailure(f.Value)
	if err != nil {
		return nil, fmt.Errorf("s1enb: parse Handover Preparation Failure: %w", err)
	}

	return fail, nil
}

type EPSKeyMaterial struct {
	KASME [32]byte
	NH    [32]byte
	NCC   uint8
}

func (ue *UE) SecurityContextForHandoverToFiveGS(ncc uint8) (EPSKeyMaterial, error) {
	var out EPSKeyMaterial

	if len(ue.kasme) != len(out.KASME) {
		return out, fmt.Errorf("s1enb: K_ASME is %d octets, want %d", len(ue.kasme), len(out.KASME))
	}

	if ncc == 0 {
		return out, fmt.Errorf("s1enb: NCC 0 names no next hop to derive K'AMF from")
	}

	copy(out.KASME[:], ue.kasme)
	out.NCC = ncc

	kenb, err := ue.deriveKeNB()
	if err != nil {
		return out, err
	}

	sync := kenb[:]

	for range ncc {
		nh, err := deriveNextHop(ue.kasme, sync)
		if err != nil {
			return out, err
		}

		out.NH = nh
		sync = out.NH[:]
	}

	return out, nil
}

func (ue *UE) deriveKeNB() ([32]byte, error) {
	var kenb [32]byte

	p0 := []byte{
		byte(ue.kenbCount >> 24), byte(ue.kenbCount >> 16),
		byte(ue.kenbCount >> 8), byte(ue.kenbCount),
	}

	out, err := ueauth.GetKDFValue(ue.kasme, fcKeNB, p0, ueauth.KDFLen(p0))
	if err != nil {
		return kenb, fmt.Errorf("s1enb: derive K_eNB: %w", err)
	}

	if len(out) != len(kenb) {
		return kenb, fmt.Errorf("s1enb: K_eNB is %d octets, want %d", len(out), len(kenb))
	}

	copy(kenb[:], out)

	return kenb, nil
}

func deriveNextHop(kasme, sync []byte) ([32]byte, error) {
	var nh [32]byte

	out, err := ueauth.GetKDFValue(kasme, fcNextHop, sync, ueauth.KDFLen(sync))
	if err != nil {
		return nh, fmt.Errorf("s1enb: derive NH: %w", err)
	}

	if len(out) != len(nh) {
		return nh, fmt.Errorf("s1enb: NH is %d octets, want %d", len(out), len(nh))
	}

	copy(nh[:], out)

	return nh, nil
}
