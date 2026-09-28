// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

const (
	minNASMessageContainerLen = 2
	maxNASMessageContainerLen = 251
)

// DownlinkNASTransport is the DOWNLINK NAS TRANSPORT message (TS 24.301 §8.2.12).
type DownlinkNASTransport struct {
	NASMessageContainer []byte

	Unrecognized []nas.RawIE
}

// UplinkNASTransport is the UPLINK NAS TRANSPORT message (TS 24.301 §8.2.30).
type UplinkNASTransport struct {
	NASMessageContainer []byte

	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain DOWNLINK NAS TRANSPORT message onto b.
func (m *DownlinkNASTransport) AppendBinary(b []byte) ([]byte, error) {
	return appendNASTransport(b, MsgDownlinkNASTransport, m.NASMessageContainer, m.Unrecognized)
}

// MarshalBinary encodes the message.
func (m *DownlinkNASTransport) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDownlinkNASTransport decodes the message.
func ParseDownlinkNASTransport(b []byte) (*DownlinkNASTransport, error) {
	container, unrec, err := parseNASTransport(b, MsgDownlinkNASTransport)
	if container == nil {
		return nil, err
	}

	return &DownlinkNASTransport{NASMessageContainer: container, Unrecognized: unrec}, err
}

// AppendBinary encodes the plain UPLINK NAS TRANSPORT message onto b.
func (m *UplinkNASTransport) AppendBinary(b []byte) ([]byte, error) {
	return appendNASTransport(b, MsgUplinkNASTransport, m.NASMessageContainer, m.Unrecognized)
}

// MarshalBinary encodes the message.
func (m *UplinkNASTransport) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseUplinkNASTransport decodes the message.
func ParseUplinkNASTransport(b []byte) (*UplinkNASTransport, error) {
	container, unrec, err := parseNASTransport(b, MsgUplinkNASTransport)
	if container == nil {
		return nil, err
	}

	return &UplinkNASTransport{NASMessageContainer: container, Unrecognized: unrec}, err
}

func checkNASMessageContainer(c []byte) error {
	if len(c) < minNASMessageContainerLen || len(c) > maxNASMessageContainerLen {
		return fmt.Errorf("nas/eps: NAS message container is %d octets, want %d to %d",
			len(c), minNASMessageContainerLen, maxNASMessageContainerLen)
	}

	return nil
}

func appendNASTransport(b []byte, mt MessageType, container []byte, unrecognized []nas.RawIE) ([]byte, error) {
	if err := checkNASMessageContainer(container); err != nil {
		return b, err
	}

	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, mt)
	w.LV(container)

	o.Raw(unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

func parseNASTransport(b []byte, mt MessageType) ([]byte, []nas.RawIE, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, mt); err != nil {
		return nil, nil, err
	}

	container, err := r.LV()
	if err != nil {
		return nil, nil, err
	}

	if err := checkNASMessageContainer(container); err != nil {
		return nil, nil, err
	}

	unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, nil, err
	}

	return container, unrec, err
}
