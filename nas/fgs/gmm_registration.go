// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// RegistrationAccept is the REGISTRATION ACCEPT message (TS 24.501 §8.2.7). The
// mandatory 5GS registration result is followed by optional IEs, several of them
// (GUTI, equivalent PLMNs, TAI list, allowed NSSAI, network feature support)
// supplied as their already-encoded IE value.
type RegistrationAccept struct {
	RegistrationResult           uint8  // 5GS registration result value (bits 1-3)
	GUTI                         []byte // optional (IEI 0x77): 5GS mobile identity value
	EquivalentPlmns              []byte // optional (IEI 0x4A)
	TAIList                      []byte // optional (IEI 0x54)
	AllowedNSSAI                 []byte // optional (IEI 0x15)
	NetworkFeatureSupport        []byte // optional (IEI 0x21)
	PDUSessionStatus             []byte // optional (IEI 0x50)
	PDUSessionReactivationResult []byte // optional (IEI 0x26)
	ReactivationResultErrorCause []byte // optional (IEI 0x72), TLV-E
	T3512Value                   *uint8 // optional (IEI 0x5E), GPRS timer 3 octet
	NegotiatedDRX                *uint8 // optional (IEI 0x51), DRX value
}

// Marshal encodes the plain REGISTRATION ACCEPT message.
func (m *RegistrationAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgRegistrationAccept)

	// 5GS registration result (mandatory, LV, 1-octet value).
	if err := w.LV([]byte{m.RegistrationResult & 0x07}); err != nil {
		return nil, err
	}

	if m.GUTI != nil {
		writeTLVE(&w, ieiGUTI5G, m.GUTI)
	}

	if m.EquivalentPlmns != nil {
		writeTLV(&w, ieiEquivalentPlmns, m.EquivalentPlmns)
	}

	if m.TAIList != nil {
		writeTLV(&w, ieiTAIList, m.TAIList)
	}

	if m.AllowedNSSAI != nil {
		writeTLV(&w, ieiAllowedNSSAI, m.AllowedNSSAI)
	}

	if m.NetworkFeatureSupport != nil {
		writeTLV(&w, ieiNetworkFeature, m.NetworkFeatureSupport)
	}

	if m.PDUSessionStatus != nil {
		writeTLV(&w, ieiPDUSessionStatus, m.PDUSessionStatus)
	}

	if m.PDUSessionReactivationResult != nil {
		writeTLV(&w, ieiPDUReactResult, m.PDUSessionReactivationResult)
	}

	if m.ReactivationResultErrorCause != nil {
		writeTLVE(&w, ieiPDUReactErrCause, m.ReactivationResultErrorCause)
	}

	if m.T3512Value != nil {
		writeTLV(&w, ieiT3512Value, []byte{*m.T3512Value})
	}

	if m.NegotiatedDRX != nil {
		writeTLV(&w, ieiNegotiatedDRX, []byte{*m.NegotiatedDRX})
	}

	return w.Bytes(), nil
}
