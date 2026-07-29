// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding"

	"github.com/ellanetworks/core/nas"
)

// Every message and information element in this package encodes through
// AppendBinary and MarshalBinary, the shapes of encoding.BinaryAppender and
// encoding.BinaryMarshaler.
var (
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = (*RegistrationRequest)(nil)
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = (*SecurityProtectedMessage)(nil)
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = SNSSAI{}
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = QoSRules{}
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = QoSFlowDescriptions{}
)

// messageResult finishes a message encoding. It refuses an encoding no NAS
// container could carry, measured on what this message appended rather than on
// the caller's whole buffer: AppendBinary composes onto a buffer that may
// already hold something else, and the limit belongs to the message
// (nas.MaxPDULen). The caller's buffer comes back unchanged on failure, as on
// every other encode error.
func messageResult(w *nas.Writer, b []byte) ([]byte, error) {
	out, err := w.Result(b)
	if err != nil {
		return b, err
	}

	if err := nas.CheckPDULen(len(out) - len(b)); err != nil {
		return b, err
	}

	return out, nil
}

// marshalMessage encodes a whole NAS message. The length a NAS container can
// carry is enforced by messageResult, on the appending primitive this calls, so
// both entry points refuse the same messages.
func marshalMessage(m interface{ AppendBinary([]byte) ([]byte, error) }) ([]byte, error) {
	b, err := m.AppendBinary(nil)
	if err != nil {
		return nil, err
	}

	return b, nil
}
