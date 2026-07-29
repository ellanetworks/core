// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

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
	} = (*AttachRequest)(nil)
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = (*SecurityProtectedMessage)(nil)
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = MobileIdentity{}
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = EPSQoS{}
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = APNAMBR{}
)

// marshalMessage encodes a whole NAS message, which — unlike an information
// element — must fit what a NAS container can carry (nas.MaxPDULen).
func marshalMessage(m interface{ AppendBinary([]byte) ([]byte, error) }) ([]byte, error) {
	b, err := m.AppendBinary(nil)
	if err != nil {
		return nil, err
	}

	if err := nas.CheckPDULen(len(b)); err != nil {
		return nil, err
	}

	return b, nil
}
