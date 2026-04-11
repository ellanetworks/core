// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type RRCEstablishmentCause aper.Enumerated

type UserLocationKind uint8

const (
	UserLocationKindUnknown UserLocationKind = iota
	UserLocationKindNR
	UserLocationKindEUTRA
	UserLocationKindN3IWF
)

// UserLocationInformation wraps the free5gc CHOICE. When Kind is not
// Unknown, raw and the variant pointer matching Kind are both non-nil.
// Raw is a transitional accessor for callers not yet migrated to typed
// fields.
type UserLocationInformation struct {
	kind UserLocationKind
	raw  *ngapType.UserLocationInformation
}

func (u UserLocationInformation) Kind() UserLocationKind                 { return u.kind }
func (u UserLocationInformation) Raw() *ngapType.UserLocationInformation { return u.raw }

type FiveGSTMSI struct {
	AMFSetID   aper.BitString
	AMFPointer aper.BitString
	FiveGTMSI  []byte
}

// InitialUEMessage is a decoded NGAP InitialUEMessage (3GPP TS 38.413
// §9.2.5.1). Non-pointer fields are mandatory and populated when the
// accompanying *DecodeReport is non-fatal; pointer fields are optional.
type InitialUEMessage struct {
	RANUENGAPID             int64
	NASPDU                  []byte
	UserLocationInformation UserLocationInformation
	RRCEstablishmentCause   RRCEstablishmentCause
	FiveGSTMSI              *FiveGSTMSI
	UEContextRequest        bool
}
