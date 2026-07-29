// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "encoding"

// Every information element in this package encodes through AppendBinary and
// MarshalBinary, the shapes of encoding.BinaryAppender and
// encoding.BinaryMarshaler.
var (
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = PLMN{}
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = PLMNList{}
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = ProtocolConfigurationOptions{}
	_ interface {
		encoding.BinaryAppender
		encoding.BinaryMarshaler
	} = EPSBearerContextStatus{}
)
