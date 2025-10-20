package nas

import "fmt"

type QosFlowParameter struct {
	ParamContent []byte `json:"param_content"`
	ParamID      uint8  `json:"param_id"`
	ParamLen     uint8  `json:"param_len"`
}

type QoSFlowDescription struct {
	ParamList  []QosFlowParameter `json:"param_list"`
	Qfi        uint8              `json:"qfi"`
	OpCode     string             `json:"op_code"`     // "Create" | "Modify" | "Delete" | "Unknown(0x..)"
	EBit       bool               `json:"e_bit"`       // extracted from NumOfParam byte
	ParamCount uint8              `json:"param_count"` // lower 6 bits only
	QFDLen     uint8              `json:"qfd_len"`
}

const (
	QFDFixLen uint8 = 0x03
)

const (
	QFDQfiBitmask    uint8 = 0x3f // bits 6..1
	QFDOpCodeBitmask uint8 = 0xe0 // bits 8..6
	QFDEbit          uint8 = 0x40 // bit 6 (0x40) in the NumOfParam octet
)

// helper: map opcode to label
func qfdOpcodeToString(op byte) string {
	switch op & QFDOpCodeBitmask {
	case 0x20:
		return "Create"
	case 0x40:
		return "Modify"
	case 0x60:
		return "Delete"
	default:
		return fmt.Sprintf("Unknown(0x%02X)", op&QFDOpCodeBitmask)
	}
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

		// OpCode -> string
		op := content[i]
		d.OpCode = qfdOpcodeToString(op)
		i++

		// NumOfParam byte: E-bit + count
		num := content[i]
		i++

		d.EBit = (num & QFDEbit) != 0
		paramCount := num & 0x3F  // <= 63
		d.ParamCount = paramCount // <-- show "3" instead of raw 0x43 (=67)

		// Params
		d.ParamList = make([]QosFlowParameter, 0, int(paramCount))
		for p := 0; p < int(paramCount); p++ {
			if len(content[i:]) < 2 {
				return nil, fmt.Errorf("qfd: truncated parameter header at off=%d", i)
			}
			paramID := content[i]
			paramLen := content[i+1]
			i += 2

			if len(content[i:]) < int(paramLen) {
				return nil, fmt.Errorf("qfd: truncated parameter content at off=%d want=%d have=%d",
					i, paramLen, len(content[i:]))
			}
			val := make([]byte, paramLen)
			copy(val, content[i:i+int(paramLen)])
			i += int(paramLen)

			d.ParamList = append(d.ParamList, QosFlowParameter{
				ParamID:      paramID,
				ParamLen:     paramLen,
				ParamContent: val,
			})

			// QFDLen accounting: +2 (ID+Len) + content
			d.QFDLen += 2 + paramLen
		}

		descs = append(descs, d)
	}
	return descs, nil
}
