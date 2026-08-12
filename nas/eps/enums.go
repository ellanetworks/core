// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "fmt"

// AttachType is the EPS attach type of an ATTACH REQUEST (TS 24.301 §9.9.3.11).
type AttachType uint8

var attachTypeNames = map[uint8]string{
	uint8(AttachTypeEPS):          "EPS attach",
	uint8(AttachTypeCombined):     "Combined EPS/IMSI attach",
	uint8(AttachTypeEPSRLOS):      "EPS RLOS attach",
	uint8(AttachTypeEPSEmergency): "EPS emergency attach",
	uint8(AttachTypeReserved):     "Reserved",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (t AttachType) Name() string { return attachTypeNames[uint8(t)] }

func (t AttachType) String() string { return enumString(uint8(t), attachTypeNames) }

// RequestType is the request type of a PDN CONNECTIVITY REQUEST
// (TS 24.301 §9.9.4.14 → TS 24.008 §10.5.6.17). It is the EPS counterpart of
// fgs.RequestType, whose values TS 24.501 §9.11.3.47 assigns differently.
type RequestType uint8

// Request type values (TS 24.008 §10.5.6.17, table 10.5.156a). All other values
// are reserved.
const (
	RequestTypeInitialRequest                    RequestType = 1
	RequestTypeHandover                          RequestType = 2
	RequestTypeRLOS                              RequestType = 3
	RequestTypeEmergency                         RequestType = 4
	RequestTypeHandoverOfEmergencyBearerServices RequestType = 6
)

var requestTypeNames = map[uint8]string{
	uint8(RequestTypeInitialRequest):                    "Initial request",
	uint8(RequestTypeHandover):                          "Handover",
	uint8(RequestTypeRLOS):                              "RLOS",
	uint8(RequestTypeEmergency):                         "Emergency",
	uint8(RequestTypeHandoverOfEmergencyBearerServices): "Handover of emergency bearer services",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (t RequestType) Name() string { return requestTypeNames[uint8(t)] }

func (t RequestType) String() string { return enumString(uint8(t), requestTypeNames) }

// AttachResult is the EPS attach result of an ATTACH ACCEPT
// (TS 24.301 §9.9.3.10).
type AttachResult uint8

var attachResultNames = map[uint8]string{
	uint8(AttachResultEPS):      "EPS only",
	uint8(AttachResultCombined): "Combined EPS/IMSI attach",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (r AttachResult) Name() string { return attachResultNames[uint8(r)] }

func (r AttachResult) String() string { return enumString(uint8(r), attachResultNames) }

// DetachType is the detach type of a UE-originating DETACH REQUEST
// (TS 24.301 §9.9.3.7, table 9.9.3.7.1).
//
// The element's three bits mean different things in each direction, so the two
// are separate types: naming a network value with a UE constant would report the
// wrong procedure.
type DetachType uint8

var detachTypeNames = map[uint8]string{
	uint8(DetachTypeEPS):      "EPS detach",
	uint8(DetachTypeIMSI):     "IMSI detach",
	uint8(DetachTypeCombined): "Combined EPS/IMSI detach",
}

func (t DetachType) String() string { return enumString(uint8(t), detachTypeNames) }

// DetachTypeNetwork is the detach type of a network-originating DETACH REQUEST
// (TS 24.301 §9.9.3.7, table 9.9.3.7.1).
type DetachTypeNetwork uint8

var detachTypeNetworkNames = map[uint8]string{
	uint8(DetachTypeReattachRequired):    "Re-attach required",
	uint8(DetachTypeReattachNotRequired): "Re-attach not required",
	uint8(DetachTypeNetworkIMSI):         "IMSI detach",
}

func (t DetachTypeNetwork) String() string { return enumString(uint8(t), detachTypeNetworkNames) }

// EPSUpdateType is the EPS update type of a TRACKING AREA UPDATE REQUEST
// (TS 24.301 §9.9.3.14).
type EPSUpdateType uint8

var epsUpdateTypeNames = map[uint8]string{
	uint8(EPSUpdateTypeTA):               "TA updating",
	uint8(EPSUpdateTypeCombinedTALA):     "Combined TA/LA updating",
	uint8(EPSUpdateTypeCombinedTALAIMSI): "Combined TA/LA updating with IMSI attach",
	uint8(EPSUpdateTypePeriodic):         "Periodic updating",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (t EPSUpdateType) Name() string { return epsUpdateTypeNames[uint8(t)] }

func (t EPSUpdateType) String() string { return enumString(uint8(t), epsUpdateTypeNames) }

// EPSUpdateResult is the EPS update result of a TRACKING AREA UPDATE ACCEPT
// (TS 24.301 §9.9.3.13).
type EPSUpdateResult uint8

var epsUpdateResultNames = map[uint8]string{
	uint8(EPSUpdateResultTA):                   "TA updated",
	uint8(EPSUpdateResultCombined):             "Combined TA/LA updated",
	uint8(EPSUpdateResultTAISRActivated):       "TA updated and ISR activated",
	uint8(EPSUpdateResultCombinedISRActivated): "Combined TA/LA updated and ISR activated",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (r EPSUpdateResult) Name() string { return epsUpdateResultNames[uint8(r)] }

func (r EPSUpdateResult) String() string { return enumString(uint8(r), epsUpdateResultNames) }

// PDNType is the PDN type of a PDN connectivity request or PDN address
// (TS 24.301 §9.9.4.10). It is the EPS counterpart of the 5GS PDU session type.
type PDNType uint8

var pdnTypeNames = map[uint8]string{
	uint8(PDNTypeIPv4):     "IPv4",
	uint8(PDNTypeIPv6):     "IPv6",
	uint8(PDNTypeIPv4v6):   "IPv4v6",
	uint8(PDNTypeNonIP):    "non IP",
	uint8(PDNTypeEthernet): "Ethernet",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (t PDNType) Name() string { return pdnTypeNames[uint8(t)] }

func (t PDNType) String() string { return enumString(uint8(t), pdnTypeNames) }

func enumString(v uint8, names map[uint8]string) string {
	if name, ok := names[v]; ok {
		return name
	}

	return fmt.Sprintf("unknown (%d)", v)
}
