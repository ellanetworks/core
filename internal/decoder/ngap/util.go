// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

const ntpToUnixOffset = 2208988800 // seconds between 1900-01-01 and 1970-01-01

type IE struct {
	ID          utils.EnumField `json:"id"`
	Criticality utils.EnumField `json:"criticality"`
	Value       any             `json:"value,omitempty"`
	ValueType   string          `json:"value_type,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

func inferValueType(v any) string {
	if v == nil {
		return ""
	}

	switch v.(type) {
	case int64, int32, int, uint64, uint32:
		return "integer"
	case string:
		return "string"
	case []byte:
		return "bytes"
	case NASPDU:
		return "nas_pdu"
	case NRPPaPDU:
		return "nrppa_pdu"
	case rawIEValue:
		return "unmodeled"
	case utils.EnumField:
		return "enum"
	default:
		if reflect.TypeOf(v).Kind() == reflect.Slice {
			return "array"
		}

		return "object"
	}
}

func setIEValueTypes(ies []IE) {
	for i := range ies {
		ies[i].ValueType = inferValueType(ies[i].Value)
	}
}

func criticalityToEnum(c ngap.Criticality) utils.EnumField {
	return utils.NamedEnum(uint8(c), c.Name())
}

func timeStampToRFC3339(timeStampNgap []byte) (string, error) {
	if len(timeStampNgap) != 4 {
		return "", fmt.Errorf("invalid NGAP timestamp length: got %d, want 4", len(timeStampNgap))
	}

	ntpSeconds := binary.BigEndian.Uint32(timeStampNgap)
	unixSeconds := int64(ntpSeconds) - ntpToUnixOffset
	t := time.Unix(unixSeconds, 0).UTC()

	return t.Format(time.RFC3339), nil
}

func ieEnum(id ngap.ProtocolIEID) utils.EnumField {
	name, ok := ngap.ProtocolIEIDName(id)

	return utils.MakeEnum(uint16(id), name, !ok)
}

// transportLayerAddressToString formats a NGAP TransportLayerAddress byte slice
// as a human-readable string per 3GPP TS 38.414 Section 5.1:
//   - 4 bytes  → IPv4 dotted-decimal
//   - 16 bytes → IPv6 colon-notation
//   - 20 bytes → "IPv4+IPv6" dual-stack
func transportLayerAddressToString(addr []byte) string {
	switch len(addr) {
	case 4:
		return fmt.Sprintf("%d.%d.%d.%d", addr[0], addr[1], addr[2], addr[3])
	case 16:
		return net.IP(addr).String()
	case 20:
		return fmt.Sprintf("%s+%s", net.IP(addr[0:4]).String(), net.IP(addr[4:20]).String())
	default:
		return ""
	}
}

func procedureCodeToEnum(code ngap.ProcedureCode) utils.EnumField {
	name, ok := ngap.ProcedureCodeName(code)

	return utils.MakeEnum(int64(code), name, !ok)
}

// CriticalityDiagnostics is the decoded CriticalityDiagnostics IE (TS 38.413
// §9.3.1.3): which procedure/message triggered the diagnostic and the offending
// IEs. Absent sub-fields are omitted.
type CriticalityDiagnostics struct {
	ProcedureCode        *utils.EnumField           `json:"procedure_code,omitempty"`
	TriggeringMessage    *utils.EnumField           `json:"triggering_message,omitempty"`
	ProcedureCriticality *utils.EnumField           `json:"procedure_criticality,omitempty"`
	IEs                  []CriticalityDiagnosticsIE `json:"ies,omitempty"`
}

type Guami struct {
	PLMNID      PLMNID `json:"plmn_id"`
	AMFRegionID string `json:"amf_region_id"`
	AMFSetID    string `json:"amf_set_id"`
	AMFPointer  string `json:"amf_pointer"`
}

// CriticalityDiagnosticsIE reports one offending IE (TS 38.413 §9.3.1.3).
type CriticalityDiagnosticsIE struct {
	IEID        utils.EnumField `json:"ie_id"`
	Criticality utils.EnumField `json:"criticality"`
	TypeOfError utils.EnumField `json:"type_of_error"`
}

type PLMN struct {
	PLMNID           PLMNID   `json:"plmn_id"`
	SliceSupportList []SNSSAI `json:"slice_support_list,omitempty"`
}

// ranNodeIDHex renders a RAN node identifier as the hex digits its bit length
// covers, matching how the AMF stores it.

type PLMNID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type SNSSAI struct {
	SST int32   `json:"sst"`
	SD  *string `json:"sd,omitempty"`
}

func triggeringMessageToEnum(t ngap.TriggeringMessage) utils.EnumField {
	return utils.NamedEnum(uint8(t), t.Name())
}

func typeOfErrorToEnum(t ngap.TypeOfError) utils.EnumField {
	return utils.NamedEnum(uint8(t), t.Name())
}
