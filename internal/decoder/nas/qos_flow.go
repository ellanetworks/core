package nas

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
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

func qfdOpcodeToEnum(op byte) utils.EnumField[uint8] {
	switch op & QFDOpCodeBitmask {
	case 0x20:
		return utils.MakeEnum(op&QFDOpCodeBitmask, "Create", false)
	case 0x40:
		return utils.MakeEnum(op&QFDOpCodeBitmask, "Modify", false)
	case 0x60:
		return utils.MakeEnum(op&QFDOpCodeBitmask, "Delete", false)
	default:
		return utils.MakeEnum(op&QFDOpCodeBitmask, "", true)
	}
}

func qfdParamIDToEnum(id uint8) utils.EnumField[uint8] {
	switch id {
	case QFDParamID5QI:
		return utils.MakeEnum(id, "5QI", false)
	case QFDParamIDGfbrUl:
		return utils.MakeEnum(id, "GFBR UL", false)
	case QFDParamIDGfbrDl:
		return utils.MakeEnum(id, "GFBR DL", false)
	case QFDParamIDMfbrUl:
		return utils.MakeEnum(id, "MFBR UL", false)
	case QFDParamIDMfbrDl:
		return utils.MakeEnum(id, "MFBR DL", false)
	case QFDParamIDAvgWnd:
		return utils.MakeEnum(id, "Averaging Window", false)
	case QFDParamIDEpsBId:
		return utils.MakeEnum(id, "EPS Bearer ID", false)
	default:
		return utils.MakeEnum(id, fmt.Sprintf("Unknown(0x%02X)", id), true)
	}
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

// ---------- decoded structs ----------

type QosFlowParameter struct {
	ParamID  utils.EnumField[uint8] `json:"identifier"`
	ParamLen uint8                  `json:"length"`

	// Decoded variants (only one or few will be set depending on ParamID):
	FiveQI      *uint8  `json:"five_qi,omitempty"`
	GfbrUlKbps  *uint64 `json:"gfbr_ul_kbps,omitempty"`
	GfbrDlKbps  *uint64 `json:"gfbr_dl_kbps,omitempty"`
	MfbrUlKbps  *uint64 `json:"mfbr_ul_kbps,omitempty"`
	MfbrDlKbps  *uint64 `json:"mfbr_dl_kbps,omitempty"`
	AvgWindowMs *uint16 `json:"averaging_window_ms,omitempty"`
	EpsBearerID *uint8  `json:"eps_bearer_id,omitempty"`
}

type QoSFlowDescription struct {
	ParamList  []QosFlowParameter     `json:"param_list"`
	Qfi        uint8                  `json:"qfi"`
	OpCode     utils.EnumField[uint8] `json:"op_code"`
	EBit       bool                   `json:"e_bit"`
	ParamCount uint8                  `json:"param_count"`
	QFDLen     uint8                  `json:"qfd_len"`
}

func ParseAuthorizedQosFlowDescriptions(content []byte) ([]QoSFlowDescription, error) {
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
		d.OpCode = qfdOpcodeToEnum(op)
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
				ParamID:  qfdParamIDToEnum(pid),
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
				} else {
					// unknown unit → keep Raw, mark len ok but unit unknown is implicit from missing decoded field
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

			default:
				// Unknown parameter → leave Raw populated
			}

			// QFDLen accounting: +2 (ID+Len) + content
			d.QFDLen += 2 + plen
			d.ParamList = append(d.ParamList, param)
		}

		descs = append(descs, d)
	}
	return descs, nil
}
