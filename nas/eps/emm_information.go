// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// EMMInformation is the EMM INFORMATION message (TS 24.301), sent by the
// MME to provide the network name to the UE. The procedure is optional in the
// network; the MME sends it integrity-protected and ciphered after
// attach. Only the network-name IEs are carried.
type EMMInformation struct {
	FullNameForNetwork  *nas.NetworkName        // optional (IEI 0x43)
	ShortNameForNetwork *nas.NetworkName        // optional (IEI 0x45)
	LocalTimeZone       *nas.TimeZone           // optional (IEI 0x46)
	UniversalTime       *nas.TimeZoneAndTime    // optional (IEI 0x47)
	DaylightSavingTime  *nas.DaylightSavingTime // optional (IEI 0x49)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// emmInformationIEs is the optional-IE table of the EMM INFORMATION message
// (TS 24.301 §8.2.13, table 8.2.13.1).
var emmInformationIEs = []nas.OptionalIE{
	{IEI: ieiFullNameForNetwork, Format: nas.IETLV, Name: "Full name for network"},
	{IEI: ieiShortNameForNetwork, Format: nas.IETLV, Name: "Short name for network"},
	{IEI: ieiLocalTimeZone, Format: nas.IETV3, Len: 1, Name: "Local time zone"},
	{IEI: ieiUniversalTimeAndLocalTimeZone, Format: nas.IETV3, Len: 7, Name: "Universal time and local time zone"},
	{IEI: ieiNetworkDaylightSavingTime, Format: nas.IETLV, Name: "Network daylight saving time"},
}

// AppendBinary encodes the plain EMM INFORMATION message.
// The encoding is appended to b.
func (m *EMMInformation) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgEMMInformation)

	if m.FullNameForNetwork != nil {
		raw, err := m.FullNameForNetwork.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiFullNameForNetwork, raw)
	}

	if m.ShortNameForNetwork != nil {
		raw, err := m.ShortNameForNetwork.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiShortNameForNetwork, raw)
	}

	if m.LocalTimeZone != nil {
		raw, err := m.LocalTimeZone.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TV3(ieiLocalTimeZone, raw)
	}

	if m.UniversalTime != nil {
		raw, err := m.UniversalTime.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TV3(ieiUniversalTimeAndLocalTimeZone, raw)
	}

	if m.DaylightSavingTime != nil {
		raw, err := m.DaylightSavingTime.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiNetworkDaylightSavingTime, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *EMMInformation) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseEMMInformation decodes the message.
func ParseEMMInformation(b []byte) (*EMMInformation, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgEMMInformation); err != nil {
		return nil, err
	}

	out := &EMMInformation{}

	_unrec, err := walkOptionalIEs(r, emmInformationIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiFullNameForNetwork:
			name, err := nas.ParseNetworkName(value)
			if err != nil {
				return false, err
			}

			out.FullNameForNetwork = &name
		case ieiShortNameForNetwork:
			name, err := nas.ParseNetworkName(value)
			if err != nil {
				return false, err
			}

			out.ShortNameForNetwork = &name
		case ieiLocalTimeZone:
			parsed, err := nas.ParseTimeZone(value)
			if err != nil {
				return false, err
			}

			out.LocalTimeZone = &parsed
		case ieiUniversalTimeAndLocalTimeZone:
			parsed, err := nas.ParseTimeZoneAndTime(value)
			if err != nil {
				return false, err
			}

			out.UniversalTime = &parsed
		case ieiNetworkDaylightSavingTime:
			parsed, err := nas.ParseDaylightSavingTime(value)
			if err != nil {
				return false, err
			}

			out.DaylightSavingTime = &parsed
		default:
			return false, nil
		}

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
