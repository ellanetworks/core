// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// ConfigurationUpdateCommand is the CONFIGURATION UPDATE COMMAND message
// (TS 24.501 §8.2.19): an optional configuration update indication, 5G-GUTI, and
// network names. The network names are supplied as their already-encoded IE
// value (TS 24.008 §10.5.3.5a).
type ConfigurationUpdateCommand struct {
	ConfigurationUpdateIndication *ConfigurationUpdateIndication // optional (IEI 0xD)
	GUTI                          *MobileIdentity                // optional (IEI 0x77): 5G-GUTI
	FullNameForNetwork            *nas.NetworkName               // optional (IEI 0x43)
	ShortNameForNetwork           *nas.NetworkName               // optional (IEI 0x45)
	LocalTimeZone                 *nas.TimeZone                  // optional (IEI 0x46)
	UniversalTime                 *nas.TimeZoneAndTime           // optional (IEI 0x47)
	DaylightSavingTime            *nas.DaylightSavingTime        // optional (IEI 0x49)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain CONFIGURATION UPDATE COMMAND message.
// The encoding is appended to b.
func (m *ConfigurationUpdateCommand) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgConfigurationUpdateCommand)

	if m.ConfigurationUpdateIndication != nil {
		o.TV1(ieiConfigUpdateInd, m.ConfigurationUpdateIndication.Nibble())
	}

	if m.GUTI != nil {
		raw, err := m.GUTI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiGUTI5G, raw)
	}

	if m.FullNameForNetwork != nil {
		raw, err := m.FullNameForNetwork.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiFullNameForNet, raw)
	}

	if m.ShortNameForNetwork != nil {
		raw, err := m.ShortNameForNetwork.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiShortNameForNet, raw)
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

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *ConfigurationUpdateCommand) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// configurationUpdateCommandIEs is the full-octet optional-IE table of the
// CONFIGURATION UPDATE COMMAND (TS 24.501 §8.2.19). The configuration update
// indication is a type-1 IE, delimited generically by the walker.
var configurationUpdateCommandIEs = []nas.OptionalIE{
	{IEI: ieiGUTI5G, Format: nas.IETLVE, Name: "5G-GUTI"},
	{IEI: ieiFullNameForNet, Format: nas.IETLV, Name: "Full name for network"},
	{IEI: ieiShortNameForNet, Format: nas.IETLV, Name: "Short name for network"},
	{IEI: ieiLocalTimeZone, Format: nas.IETV3, Len: 1, Name: "Local time zone"},
	{IEI: ieiUniversalTimeAndLocalTimeZone, Format: nas.IETV3, Len: 7, Name: "Universal time and local time zone"},
	{IEI: ieiNetworkDaylightSavingTime, Format: nas.IETLV, Name: "Network daylight saving time"},
}

// ParseConfigurationUpdateCommand decodes a plain CONFIGURATION UPDATE COMMAND.
func ParseConfigurationUpdateCommand(b []byte) (*ConfigurationUpdateCommand, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgConfigurationUpdateCommand); err != nil {
		return nil, err
	}

	out := &ConfigurationUpdateCommand{}

	_unrec, err := walkOptionalIEs(r, configurationUpdateCommandIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiConfigUpdateInd:
			if len(value) == 0 {
				return false, nil
			}

			ind := ParseConfigurationUpdateIndication(value[0])
			out.ConfigurationUpdateIndication = &ind
		case ieiGUTI5G:
			parsed, err := ParseMobileIdentity(value)
			if err != nil {
				return false, err
			}

			out.GUTI = &parsed
		case ieiFullNameForNet:
			name, err := nas.ParseNetworkName(value)
			if err != nil {
				return false, err
			}

			out.FullNameForNetwork = &name
		case ieiShortNameForNet:
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

// ConfigurationUpdateComplete is the CONFIGURATION UPDATE COMPLETE message
// (TS 24.501 §8.2.20): the UE's acknowledgement, carrying no information elements.
type ConfigurationUpdateComplete struct {
	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. TS 24.501
	// §8.2.20 defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain CONFIGURATION UPDATE COMPLETE message.
// The encoding is appended to b.
func (m *ConfigurationUpdateComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgConfigurationUpdateComplete)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *ConfigurationUpdateComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseConfigurationUpdateComplete decodes the message.
func ParseConfigurationUpdateComplete(b []byte) (*ConfigurationUpdateComplete, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgConfigurationUpdateComplete); err != nil {
		return nil, err
	}

	out := &ConfigurationUpdateComplete{}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
