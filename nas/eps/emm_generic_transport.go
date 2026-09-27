// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// GenericMessageContainerType is the type of the application message a generic
// NAS transport message carries (TS 24.301 §9.9.3.42).
type GenericMessageContainerType uint8

// Generic message container type values (TS 24.301 table 9.9.3.42.1).
const (
	GenericMessageContainerTypeLPP              GenericMessageContainerType = 0x01
	GenericMessageContainerTypeLocationServices GenericMessageContainerType = 0x02
)

var genericMessageContainerTypeNames = map[uint8]string{
	uint8(GenericMessageContainerTypeLPP):              "LTE Positioning Protocol (LPP) message container",
	uint8(GenericMessageContainerTypeLocationServices): "Location services message container",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (t GenericMessageContainerType) Name() string {
	return genericMessageContainerTypeNames[uint8(t)]
}

func (t GenericMessageContainerType) String() string {
	return enumString(uint8(t), genericMessageContainerTypeNames)
}

// DownlinkGenericNASTransport is the DOWNLINK GENERIC NAS TRANSPORT message
// (TS 24.301 §8.2.31).
type DownlinkGenericNASTransport struct {
	ContainerType         GenericMessageContainerType
	Container             []byte
	AdditionalInformation []byte

	Unrecognized []nas.RawIE
}

// UplinkGenericNASTransport is the UPLINK GENERIC NAS TRANSPORT message
// (TS 24.301 §8.2.32).
type UplinkGenericNASTransport struct {
	ContainerType         GenericMessageContainerType
	Container             []byte
	AdditionalInformation []byte

	Unrecognized []nas.RawIE
}

var genericNASTransportIEs = []nas.OptionalIE{
	{IEI: ieiAdditionalInformation, Format: nas.IETLV, Name: "Additional information"},
}

// AppendBinary encodes the plain DOWNLINK GENERIC NAS TRANSPORT message.
// The encoding is appended to b.
func (m *DownlinkGenericNASTransport) AppendBinary(b []byte) ([]byte, error) {
	return appendGenericNASTransport(b, MsgDownlinkGenericNASTransport, m.ContainerType, m.Container, m.AdditionalInformation, m.Unrecognized)
}

// MarshalBinary encodes the message.
func (m *DownlinkGenericNASTransport) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDownlinkGenericNASTransport decodes the message.
func ParseDownlinkGenericNASTransport(b []byte) (*DownlinkGenericNASTransport, error) {
	fields, err := parseGenericNASTransport(b, MsgDownlinkGenericNASTransport)
	if fields == nil {
		return nil, err
	}

	return &DownlinkGenericNASTransport{
		ContainerType:         fields.containerType,
		Container:             fields.container,
		AdditionalInformation: fields.additionalInformation,
		Unrecognized:          fields.unrecognized,
	}, err
}

// AppendBinary encodes the plain UPLINK GENERIC NAS TRANSPORT message.
// The encoding is appended to b.
func (m *UplinkGenericNASTransport) AppendBinary(b []byte) ([]byte, error) {
	return appendGenericNASTransport(b, MsgUplinkGenericNASTransport, m.ContainerType, m.Container, m.AdditionalInformation, m.Unrecognized)
}

// MarshalBinary encodes the message.
func (m *UplinkGenericNASTransport) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseUplinkGenericNASTransport decodes the message.
func ParseUplinkGenericNASTransport(b []byte) (*UplinkGenericNASTransport, error) {
	fields, err := parseGenericNASTransport(b, MsgUplinkGenericNASTransport)
	if fields == nil {
		return nil, err
	}

	return &UplinkGenericNASTransport{
		ContainerType:         fields.containerType,
		Container:             fields.container,
		AdditionalInformation: fields.additionalInformation,
		Unrecognized:          fields.unrecognized,
	}, err
}

type genericNASTransportFields struct {
	containerType         GenericMessageContainerType
	container             []byte
	additionalInformation []byte
	unrecognized          []nas.RawIE
}

func appendGenericNASTransport(b []byte, mt MessageType, containerType GenericMessageContainerType, container, additionalInformation []byte, unrecognized []nas.RawIE) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, mt)
	w.U8(uint8(containerType))
	w.LVE(container)

	if len(additionalInformation) > 0 {
		o.TLV(ieiAdditionalInformation, additionalInformation)
	}

	o.Raw(unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

func parseGenericNASTransport(b []byte, mt MessageType) (*genericNASTransportFields, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, mt); err != nil {
		return nil, err
	}

	containerType, err := r.U8()
	if err != nil {
		return nil, err
	}

	container, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &genericNASTransportFields{
		containerType: GenericMessageContainerType(containerType),
		container:     container,
	}

	_unrec, err := walkOptionalIEs(r, genericNASTransportIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiAdditionalInformation {
			return false, nil
		}

		out.additionalInformation = value

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.unrecognized = _unrec

	return out, err
}
