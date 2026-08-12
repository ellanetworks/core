// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "fmt"

// ServiceType is the service type of a SERVICE REQUEST (TS 24.501 §9.11.3.50).
type ServiceType uint8

var serviceTypeNames = map[uint8]string{
	uint8(ServiceTypeSignalling):                "Signalling",
	uint8(ServiceTypeData):                      "Data",
	uint8(ServiceTypeMobileTerminatedServices):  "Mobile terminated services",
	uint8(ServiceTypeEmergencyServices):         "Emergency services",
	uint8(ServiceTypeEmergencyServicesFallback): "Emergency services fallback",
	uint8(ServiceTypeHighPriorityAccess):        "High priority access",
	uint8(ServiceTypeElevatedSignalling):        "Elevated signalling",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.501 assigns.
func (t ServiceType) Name() string { return serviceTypeNames[uint8(t)] }

func (t ServiceType) String() string { return enumString(uint8(t), serviceTypeNames) }

// RegistrationType is the 5GS registration type (TS 24.501 §9.11.3.7).
type RegistrationType uint8

var registrationTypeNames = map[uint8]string{
	uint8(RegistrationTypeInitial):                         "Initial registration",
	uint8(RegistrationTypeMobilityUpdating):                "Mobility registration updating",
	uint8(RegistrationTypePeriodicUpdating):                "Periodic registration updating",
	uint8(RegistrationTypeEmergency):                       "Emergency registration",
	uint8(RegistrationTypeSNPNOnboarding):                  "SNPN onboarding registration",
	uint8(RegistrationTypeDisasterRoamingMobilityUpdating): "Disaster roaming mobility registration updating",
	uint8(RegistrationTypeDisasterRoamingInitial):          "Disaster roaming initial registration",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.501 assigns.
func (t RegistrationType) Name() string { return registrationTypeNames[uint8(t)] }

func (t RegistrationType) String() string { return enumString(uint8(t), registrationTypeNames) }

// AccessType is the access type of a de-registration (TS 24.501 §9.11.3.20).
type AccessType uint8

var accessTypeNames = map[uint8]string{
	uint8(AccessType3GPP):    "3GPP access",
	uint8(AccessTypeNon3GPP): "Non-3GPP access",
	uint8(AccessTypeBoth):    "3GPP access and non-3GPP access",
}

func (t AccessType) String() string { return enumString(uint8(t), accessTypeNames) }

// PayloadContainerType is the payload container type of a NAS transport message
// (TS 24.501 §9.11.3.40).
type PayloadContainerType uint8

var payloadContainerTypeNames = map[uint8]string{
	uint8(PayloadContainerTypeN1SMInfo):          "N1 SM information",
	uint8(PayloadContainerTypeSMS):               "SMS",
	uint8(PayloadContainerTypeLPP):               "LPP message container",
	uint8(PayloadContainerTypeSOR):               "SOR transparent container",
	uint8(PayloadContainerTypeUEPolicy):          "UE policy container",
	uint8(PayloadContainerTypeUEParameterUpdate): "UE parameters update transparent container",
	uint8(PayloadContainerTypeMultiplePayload):   "Multiple payloads",
}

func (t PayloadContainerType) String() string {
	return enumString(uint8(t), payloadContainerTypeNames)
}

// RequestType is the request type of an UL NAS TRANSPORT (TS 24.501 §9.11.3.47).
type RequestType uint8

var requestTypeNames = map[uint8]string{
	uint8(RequestTypeInitialRequest):              "Initial request",
	uint8(RequestTypeExistingPDUSession):          "Existing PDU session",
	uint8(RequestTypeInitialEmergencyRequest):     "Initial emergency request",
	uint8(RequestTypeExistingEmergencyPDUSession): "Existing emergency PDU session",
	uint8(RequestTypeModificationRequest):         "Modification request",
	uint8(RequestTypeReserved):                    "Reserved",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.501 assigns.
func (t RequestType) Name() string { return requestTypeNames[uint8(t)] }

func (t RequestType) String() string { return enumString(uint8(t), requestTypeNames) }

// SessionAMBRUnit is the unit of a Session-AMBR bit rate (TS 24.501 §9.11.4.14).
type SessionAMBRUnit uint8

var sessionAMBRUnitNames = map[uint8]string{
	uint8(SessionAMBRUnitNotUsed): "Not used",
	uint8(SessionAMBRUnit1Kbps):   "1 kbps",
	uint8(SessionAMBRUnit1Mbps):   "1 Mbps",
	uint8(SessionAMBRUnit1Gbps):   "1 Gbps",
	uint8(SessionAMBRUnit1Tbps):   "1 Tbps",
	uint8(SessionAMBRUnit1Pbps):   "1 Pbps",
}

func (u SessionAMBRUnit) String() string { return enumString(uint8(u), sessionAMBRUnitNames) }

// PDUSessionType is the type of a PDU session (TS 24.501 §9.11.4.11).
type PDUSessionType uint8

var pduSessionTypeNames = map[uint8]string{
	uint8(PDUSessionTypeIPv4):         "IPv4",
	uint8(PDUSessionTypeIPv6):         "IPv6",
	uint8(PDUSessionTypeIPv4v6):       "IPv4v6",
	uint8(PDUSessionTypeUnstructured): "Unstructured",
	uint8(PDUSessionTypeEthernet):     "Ethernet",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.501 assigns.
func (t PDUSessionType) Name() string { return pduSessionTypeNames[uint8(t)] }

func (t PDUSessionType) String() string { return enumString(uint8(t), pduSessionTypeNames) }

// SSCMode is the session and service continuity mode of a PDU session
// (TS 24.501 §9.11.4.16).
type SSCMode uint8

// SSC mode values (TS 24.501 §9.11.4.16).
const (
	SSCMode1 SSCMode = 1
	SSCMode2 SSCMode = 2
	SSCMode3 SSCMode = 3
)

var sscModeNames = map[uint8]string{
	uint8(SSCMode1): "SSC mode 1",
	uint8(SSCMode2): "SSC mode 2",
	uint8(SSCMode3): "SSC mode 3",
}

func (m SSCMode) String() string { return enumString(uint8(m), sscModeNames) }

// RegistrationResult is the 5GS registration result value of a REGISTRATION
// ACCEPT (TS 24.501 §9.11.3.6): which accesses the UE is registered over.
type RegistrationResult uint8

// 5GS registration result values (TS 24.501 §9.11.3.6, bits 1-3).
const (
	RegistrationResult3GPP           RegistrationResult = 0x01
	RegistrationResultNon3GPP        RegistrationResult = 0x02
	RegistrationResult3GPPAndNon3GPP RegistrationResult = 0x03
)

var registrationResultNames = map[uint8]string{
	uint8(RegistrationResult3GPP):           "3GPP access",
	uint8(RegistrationResultNon3GPP):        "Non-3GPP access",
	uint8(RegistrationResult3GPPAndNon3GPP): "3GPP and non-3GPP access",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.501 assigns.
func (r RegistrationResult) Name() string { return registrationResultNames[uint8(r)] }

func (r RegistrationResult) String() string { return enumString(uint8(r), registrationResultNames) }

func enumString(v uint8, names map[uint8]string) string {
	if name, ok := names[v]; ok {
		return name
	}

	return fmt.Sprintf("unknown (%d)", v)
}
