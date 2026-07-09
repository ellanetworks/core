// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// GUTIReallocationCommand is the GUTI REALLOCATION COMMAND message
// (TS 24.301 §8.2.16). The assigned GUTI is the only mandatory information
// element (an LV); the optional TAI list and other IEs are not emitted, as a
// standalone reallocation leaves the registration area unchanged.
type GUTIReallocationCommand struct {
	GUTI EPSMobileIdentity
}

// Marshal encodes the plain GUTI REALLOCATION COMMAND message.
func (m *GUTIReallocationCommand) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgGUTIReallocationCommand)

	v, err := m.GUTI.encode()
	if err != nil {
		return nil, err
	}

	if err := w.LV(v); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// ParseGUTIReallocationCommand decodes a plain GUTI REALLOCATION COMMAND message.
func ParseGUTIReallocationCommand(b []byte) (*GUTIReallocationCommand, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgGUTIReallocationCommand); err != nil {
		return nil, err
	}

	v, err := r.LV()
	if err != nil {
		return nil, err
	}

	id, err := decodeEPSMobileIdentity(v)
	if err != nil {
		return nil, err
	}

	return &GUTIReallocationCommand{GUTI: id}, nil
}

// GUTIReallocationComplete is the GUTI REALLOCATION COMPLETE message
// (TS 24.301 §8.2.17), which carries no information elements.
type GUTIReallocationComplete struct{}

// Marshal encodes the plain GUTI REALLOCATION COMPLETE message.
func (m *GUTIReallocationComplete) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgGUTIReallocationComplete)

	return w.Bytes(), nil
}

// ParseGUTIReallocationComplete decodes a plain GUTI REALLOCATION COMPLETE message.
func ParseGUTIReallocationComplete(b []byte) (*GUTIReallocationComplete, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgGUTIReallocationComplete); err != nil {
		return nil, err
	}

	return &GUTIReallocationComplete{}, nil
}
