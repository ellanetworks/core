// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// Container size bounds (TS 38.413, NGAP-Constants).
const (
	maxProtocolIEs        = 65535
	maxProtocolExtensions = 65535
)

var profile = &apimpl.Profile{
	Name:                  "ngap",
	MaxProtocolIEs:        maxProtocolIEs,
	MaxProtocolExtensions: maxProtocolExtensions,
	OpaqueProcedures:      []uint8{uint8(ProcPrivateMessage)},
	ProcedureName:         func(pc uint8) string { return ProcedureCode(pc).String() },
	IEName:                func(id uint16) string { return ProtocolIEID(id).String() },
}

type ieField = apimpl.FieldValue[ProtocolIEID]

type SingleContainer = ap.SingleContainer[ProtocolIEID]

type rawIE = ap.Field[ProtocolIEID]

// A ProtocolIE-Field the message type does not model; Value is open-type bytes.
type RawIE = ap.Field[ProtocolIEID]

func decodeIE(r *per.Reader, u per.Unmarshaler) error {
	return u.UnmarshalPER(r, per.Aligned)
}

func restOctets(r *per.Reader) []byte {
	b, _ := r.ReadOctets(r.Bits() / 8)

	return b
}

// Keeps every field, including ids the caller does not model.
//
//nolint:unparam
func decodeIEContainer(r *per.Reader, enc per.Encoding) ([]rawIE, error) {
	return apimpl.DecodeIEContainer[ProtocolIEID](profile, r, enc)
}

func perIEDecode(b []byte, u per.Unmarshaler) error {
	return apimpl.DecodeValue(b, u)
}

func encodeExtensionContainer(w *per.Writer, enc per.Encoding, fields []ieField) error {
	return apimpl.EncodeExtensionContainer(profile, w, enc, fields)
}

func decodeExtensionContainer[M any](r *per.Reader, enc per.Encoding, table []ieSpec[M], m *M, meta *extensionMeta) error {
	return apimpl.DecodeExtensionContainer(profile, r, enc, table, m, meta)
}
