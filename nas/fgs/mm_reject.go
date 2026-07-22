// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// Status5GMM is the 5GMM STATUS message (TS 24.501 §8.2.29): a mandatory 5GMM
// cause reporting an error detected on received 5GMM signalling.
type Status5GMM struct {
	Cause uint8
}

// Marshal encodes the plain 5GMM STATUS message.
func (m *Status5GMM) Marshal() ([]byte, error) {
	var w common.Writer

	writeMMHeader(&w, Msg5GMMStatus)
	w.U8(m.Cause)

	return w.Bytes(), nil
}

// ParseStatus5GMM decodes the message.
func ParseStatus5GMM(b []byte) (*Status5GMM, error) {
	return parseMMCause(b, Msg5GMMStatus)
}

// SecurityModeReject is the SECURITY MODE REJECT message (TS 24.501 §8.2.27): a
// mandatory 5GMM cause.
type SecurityModeReject struct {
	Cause uint8
}

// ParseSecurityModeReject decodes the message.
func ParseSecurityModeReject(b []byte) (*SecurityModeReject, error) {
	s, err := parseMMCause(b, MsgSecurityModeReject)
	if err != nil {
		return nil, err
	}

	return &SecurityModeReject{Cause: s.Cause}, nil
}

// ServiceReject is the SERVICE REJECT message (TS 24.501 §8.2.18): a mandatory
// 5GMM cause. The optional IEs Ella does not set are omitted.
type ServiceReject struct {
	Cause uint8
}

// Marshal encodes the plain SERVICE REJECT message.
func (m *ServiceReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeMMHeader(&w, MsgServiceReject)
	w.U8(m.Cause)

	return w.Bytes(), nil
}

// RegistrationReject is the REGISTRATION REJECT message (TS 24.501 §8.2.9): a
// mandatory 5GMM cause and an optional T3502 value (GPRS timer 2).
type RegistrationReject struct {
	Cause uint8
	T3502 *uint8 // optional (IEI 0x16)
}

// Marshal encodes the plain REGISTRATION REJECT message.
func (m *RegistrationReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeMMHeader(&w, MsgRegistrationReject)
	w.U8(m.Cause)

	if m.T3502 != nil {
		writeTLV(&w, ieiT3502Value, []byte{*m.T3502})
	}

	return w.Bytes(), nil
}

// parseMMCause decodes a 5GMM message whose body is a single mandatory 5GMM
// cause octet.
func parseMMCause(b []byte, want MessageType) (*Status5GMM, error) {
	r := common.NewReader(b)

	if err := readMMHeader(r, want); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &Status5GMM{Cause: cause}, nil
}
