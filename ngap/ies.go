// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// Container size bounds (TS 38.413, NGAP-Constants).
const (
	maxProtocolIEs        = 65535
	maxProtocolExtensions = 65535
)

var profile = &ap.Profile{
	Name:                  "ngap",
	MaxProtocolIEs:        maxProtocolIEs,
	MaxProtocolExtensions: maxProtocolExtensions,
	OpaqueProcedures:      []uint8{uint8(ProcPrivateMessage)},
	ProcedureName:         func(pc uint8) string { return ProcedureCode(pc).String() },
	IEName:                func(id uint16) string { return ProtocolIEID(id).String() },
}

type ieField = ap.FieldValue[ProtocolIEID]

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
	return ap.DecodeIEContainer[ProtocolIEID](profile, r, enc)
}

func perIEDecode(b []byte, u per.Unmarshaler) error {
	return u.UnmarshalPER(per.NewReader(b), per.Aligned)
}
