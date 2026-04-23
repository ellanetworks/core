package utils

import (
	"encoding/binary"
	"fmt"
)

// ---- QFD constants (TS 24.501 §9.11.4.12) ----
const (
	QFDParamID5QI    uint8 = 0x01
	QFDParamIDGfbrUl uint8 = 0x02
	QFDParamIDGfbrDl uint8 = 0x03
	QFDParamIDMfbrUl uint8 = 0x04
	QFDParamIDMfbrDl uint8 = 0x05
	QFDParamIDAvgWnd uint8 = 0x06
	QFDParamIDEpsBId uint8 = 0x07
)

const (
	QFDQfiBitmask    uint8 = 0x3f // bits 6..1
	QFDOpCodeBitmask uint8 = 0xe0 // bits 8..6
	QFDEbit          uint8 = 0x40 // bit 6 in the NumOfParam octet
)

const QFDFixLen uint8 = 0x03

// Unit codes used in rate params
const (
	QFRateUnit1Kbps uint8 = 0x01
	QFRateUnit1Mbps uint8 = 0x06
	QFRateUnit1Gbps uint8 = 0x0B
)

type QosFlowParameter struct {
	ParamID  uint8
	ParamLen uint8

	FiveQI      *uint8
	GfbrUlKbps  *uint64
	GfbrDlKbps  *uint64
	MfbrUlKbps  *uint64
	MfbrDlKbps  *uint64
	AvgWindowMs *uint16
	EpsBearerID *uint8
}

type QoSFlowDescription struct {
	ParamList  []QosFlowParameter
	Qfi        uint8
	OpCode     uint8
	EBit       bool
	ParamCount uint8
	QFDLen     uint8
}

func ParseAuthorizedQosFlowDescriptions(content []byte) ([]QoSFlowDescription, error) { // nolint:gocognit
	var descs []QoSFlowDescription

	i := 0

	for i < len(content) {
		if len(content[i:]) < 3 {
			return nil, fmt.Errorf("qfd: truncated header at off=%d (have %d, need 3)", i, len(content[i:]))
		}

		var d QoSFlowDescription

		d.QFDLen = QFDFixLen

		// QFI (mask to bits 6..1)
		d.Qfi = content[i] & QFDQfiBitmask
		i++

		// OpCode
		op := content[i]
		d.OpCode = op
		i++

		// NumOfParam: E-bit + count(6 bits)
		num := content[i]
		i++
		d.EBit = (num & QFDEbit) != 0
		d.ParamCount = num & 0x3F

		// Parameters
		d.ParamList = make([]QosFlowParameter, 0, int(d.ParamCount))
		for p := 0; p < int(d.ParamCount); p++ {
			if len(content[i:]) < 2 {
				return nil, fmt.Errorf("qfd: truncated parameter header at off=%d", i)
			}

			pid := content[i]
			plen := content[i+1]
			i += 2

			if len(content[i:]) < int(plen) {
				return nil, fmt.Errorf("qfd: truncated parameter content at off=%d want=%d have=%d", i, plen, len(content[i:]))
			}

			raw := make([]byte, plen)
			copy(raw, content[i:i+int(plen)])
			i += int(plen)

			param := QosFlowParameter{
				ParamID:  pid,
				ParamLen: plen,
			}

			switch pid {
			case QFDParamID5QI:
				// length must be 1
				if plen != 1 {
					break
				}

				v := raw[0]
				param.FiveQI = &v

			case QFDParamIDGfbrUl, QFDParamIDGfbrDl, QFDParamIDMfbrUl, QFDParamIDMfbrDl:
				// length must be 3: [unit][MSB][LSB]
				if plen != 3 {
					break
				}

				unit := raw[0]
				val := binary.BigEndian.Uint16(raw[1:3])

				if kbps, ok := toKbps(unit, val); ok {
					switch pid {
					case QFDParamIDGfbrUl:
						param.GfbrUlKbps = &kbps
					case QFDParamIDGfbrDl:
						param.GfbrDlKbps = &kbps
					case QFDParamIDMfbrUl:
						param.MfbrUlKbps = &kbps
					case QFDParamIDMfbrDl:
						param.MfbrDlKbps = &kbps
					}
				}

			case QFDParamIDAvgWnd:
				// spec uses 2 bytes (ms). If your PCF uses different, adjust here.
				if plen != 2 {
					break
				}

				ms := binary.BigEndian.Uint16(raw)
				param.AvgWindowMs = &ms

			case QFDParamIDEpsBId:
				// typically 1 byte (EBI 0..15 in EPS context)
				if plen != 1 {
					break
				}

				ebi := raw[0]
				param.EpsBearerID = &ebi
			}

			// QFDLen accounting: +2 (ID+Len) + content
			d.QFDLen += 2 + plen
			d.ParamList = append(d.ParamList, param)
		}

		descs = append(descs, d)
	}

	return descs, nil
}

// convert (unit, value16) → kbps
func toKbps(unit uint8, v uint16) (kbps uint64, ok bool) {
	switch unit {
	case QFRateUnit1Kbps:
		return uint64(v), true
	case QFRateUnit1Mbps:
		return uint64(v) * 1000, true
	case QFRateUnit1Gbps:
		return uint64(v) * 1000 * 1000, true
	default:
		return 0, false
	}
}
