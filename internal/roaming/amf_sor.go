// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/nas"
)

const (
	sorIEI            = 0x73
	sorHeaderListInd  = 0x02
	sorHeaderListType = 0x04
	sorHeaderAck      = 0x08
	sorDataTypeAck    = 0x01
	sorMaxSteering    = 16
	sorMacLen         = 16
	sorCounterLen     = 2
)

var ErrSoRContainer = errors.New("roaming: malformed steering of roaming information")

type SoR struct {
	Container        []byte
	AckRequested     bool
	ProvisioningTime wire.DateTime
}

func SoRFromHome(info *wire.SorInfo) (*SoR, error) {
	if info == nil {
		return nil, nil
	}

	if len(info.SorTransparentContainer) > 0 {
		contents, err := sorContents(info.SorTransparentContainer)
		if err != nil {
			return nil, err
		}

		return &SoR{Container: contents, AckRequested: contents[0]&sorHeaderAck != 0, ProvisioningTime: info.ProvisioningTime}, nil
	}

	mac, err := hex.DecodeString(info.SorMacIausf)
	if err != nil || len(mac) != sorMacLen {
		return nil, fmt.Errorf("%w: SoR-MAC-IAUSF %q", ErrSoRContainer, info.SorMacIausf)
	}

	counter, err := hex.DecodeString(info.Countersor)
	if err != nil || len(counter) != sorCounterLen {
		return nil, fmt.Errorf("%w: CounterSoR %q", ErrSoRContainer, info.Countersor)
	}

	ack := info.AckInd != nil && *info.AckInd

	var header byte
	if ack {
		header |= sorHeaderAck
	}

	var list []byte

	switch c := info.SteeringContainer; {
	case c == nil:
		header |= sorHeaderListType
	case c.SecuredPacket != nil:
		header |= sorHeaderListInd
		list = c.SecuredPacket
	default:
		header |= sorHeaderListInd | sorHeaderListType

		list, err = steeringList(c.SteeringInfo)
		if err != nil {
			return nil, err
		}
	}

	out := make([]byte, 0, 1+sorMacLen+sorCounterLen+len(list))
	out = append(out, header)
	out = append(out, mac...)
	out = append(out, counter...)
	out = append(out, list...)

	return &SoR{Container: out, AckRequested: ack, ProvisioningTime: info.ProvisioningTime}, nil
}

func sorContents(ie []byte) ([]byte, error) {
	if len(ie) < 3 || ie[0] != sorIEI {
		return nil, fmt.Errorf("%w: the SOR transparent container does not start with IEI 0x73", ErrSoRContainer)
	}

	n := int(ie[1])<<8 | int(ie[2])
	if n < 1 || n != len(ie)-3 {
		return nil, fmt.Errorf("%w: SOR transparent container length %d with %d octets", ErrSoRContainer, n, len(ie)-3)
	}

	if ie[3]&sorDataTypeAck != 0 {
		return nil, fmt.Errorf("%w: the home sent an acknowledgement container", ErrSoRContainer)
	}

	return ie[3:], nil
}

func steeringList(infos []wire.SteeringInfo) ([]byte, error) {
	if len(infos) > sorMaxSteering {
		return nil, fmt.Errorf("%w: %d PLMNs in the steering list, at most %d", ErrSoRContainer, len(infos), sorMaxSteering)
	}

	out := make([]byte, 0, 5*len(infos))

	for _, s := range infos {
		if s.PlmnID == nil {
			return nil, fmt.Errorf("%w: steering entry without PLMN", ErrSoRContainer)
		}

		plmn, err := nas.PLMN{MCC: s.PlmnID.Mcc, MNC: s.PlmnID.Mnc}.Octets()
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSoRContainer, err)
		}

		tech := accessTechnologies(s.AccessTechList)

		out = append(out, plmn[:]...)
		out = append(out, tech[:]...)
	}

	return out, nil
}

func accessTechnologies(list []wire.AccessTech) [2]byte {
	var b [2]byte

	for _, t := range list {
		switch t {
		case wire.AccessTechUTRAN:
			b[0] |= 0x80
		case wire.AccessTechEUTRANWBS1AndNBS1:
			b[0] |= 0x40
		case wire.AccessTechEUTRANNBS1Only:
			b[0] |= 0x40 | 0x10
		case wire.AccessTechEUTRANWBS1Only:
			b[0] |= 0x40 | 0x20
		case wire.AccessTechNR:
			b[0] |= 0x08
		case wire.AccessTechSatelliteNGRAN:
			b[0] |= 0x04
		case wire.AccessTechSatelliteEUTRANWBS1:
			b[0] |= 0x02
		case wire.AccessTechSatelliteEUTRANNBS1:
			b[0] |= 0x01
		case wire.AccessTechGSMAndECGSMIoT:
			b[1] |= 0x80
		case wire.AccessTechGSMWithoutECGSMIoT:
			b[1] |= 0x80 | 0x04
		case wire.AccessTechECGSMIoTOnly:
			b[1] |= 0x80 | 0x08
		case wire.AccessTechGSMCompact:
			b[1] |= 0x40
		case wire.AccessTechCDMAHRPD:
			b[1] |= 0x20
		case wire.AccessTechCDMA1xRTT:
			b[1] |= 0x10
		}
	}

	return b
}

func SoRAcknowledgement(container []byte, provisioned wire.DateTime) (wire.AcknowledgeInfo, error) {
	if len(container) != 1+sorMacLen || container[0]&sorDataTypeAck == 0 {
		return wire.AcknowledgeInfo{}, fmt.Errorf("%w: not an acknowledgement container", ErrSoRContainer)
	}

	return wire.AcknowledgeInfo{SorMacIue: hex.EncodeToString(container[1:]), ProvisioningTime: provisioned}, nil
}
