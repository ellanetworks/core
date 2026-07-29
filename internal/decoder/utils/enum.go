// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

// EnumField is the diagnostic decoders' display value for a protocol enum,
// message type, or identifier: the raw numeric value plus a human label and an
// unknown flag, marshaled to JSON for the UI. Value is int64 — the widest domain
// any decoder needs (ASN.1 INTEGER is signed and may be wide; NAS enums are
// bytes). The type is deliberately not generic: the value is only ever rendered,
// never read back typed, so a per-decoder integer width served no purpose and
// only let the 4G/5G NAS and ASN.1 decoders drift apart.
type EnumField struct {
	Type    string `json:"type"` // always "enum"
	Value   int64  `json:"value"`
	Label   string `json:"label"`
	Unknown bool   `json:"unknown"`
}

// MakeEnum builds an EnumField from any integer value, widening it to int64.
func MakeEnum[T ~int | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64](v T, label string, unknown bool) EnumField {
	return EnumField{Type: "enum", Value: int64(v), Label: label, Unknown: unknown}
}
