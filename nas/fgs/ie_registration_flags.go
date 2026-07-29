// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "fmt"

// MICOIndication is the MICO indication information element value
// (TS 24.501 §9.11.3.31), a type-1 element whose value is the low nibble of its
// octet.
type MICOIndication struct {
	// RAAI is the registration area allocation indication (bit 1). The UE codes
	// it as zero; the network sets it to say the registration area it allocated
	// is all PLMN.
	RAAI bool
	// SPRTI is the strictly periodic registration timer indication (bit 2).
	SPRTI bool
}

// ConfigurationUpdateIndication is the configuration update indication
// information element value (TS 24.501 §9.11.3.18): whether the network wants
// the UE to acknowledge the command, and whether it wants the UE to register
// again.
type ConfigurationUpdateIndication struct {
	// ACK asks the UE to answer with a CONFIGURATION UPDATE COMPLETE (bit 1).
	ACK bool
	// RED asks the UE to perform a registration procedure (bit 2).
	RED bool
}

// ParseConfigurationUpdateIndication decodes a configuration update indication
// value nibble.
func ParseConfigurationUpdateIndication(v uint8) ConfigurationUpdateIndication {
	return ConfigurationUpdateIndication{ACK: v&0x01 != 0, RED: v&0x02 != 0}
}

// Nibble is the value half-octet this element occupies.
func (c ConfigurationUpdateIndication) Nibble() uint8 {
	return boolBit(c.ACK, 0) | boolBit(c.RED, 1)
}

func (c ConfigurationUpdateIndication) String() string {
	return fmt.Sprintf("ACK=%t RED=%t", c.ACK, c.RED)
}

// ParseMICOIndication decodes a MICO indication value nibble.
func ParseMICOIndication(v uint8) MICOIndication {
	return MICOIndication{RAAI: v&0x01 != 0, SPRTI: v&0x02 != 0}
}

// Nibble is the value half-octet this element occupies.
func (m MICOIndication) Nibble() uint8 {
	return boolBit(m.RAAI, 0) | boolBit(m.SPRTI, 1)
}

func (m MICOIndication) String() string {
	return fmt.Sprintf("RAAI=%t SPRTI=%t", m.RAAI, m.SPRTI)
}

// PreferredCIoTNetworkBehaviour is which CIoT optimization a UE prefers the
// network to use (TS 24.501 §9.11.3.9A, table 9.11.3.9A.1). The same coding
// names the 5GS and the EPS preference.
type PreferredCIoTNetworkBehaviour uint8

// Preferred CIoT network behaviours (TS 24.501 table 9.11.3.9A.1).
const (
	CIoTNoPreference  PreferredCIoTNetworkBehaviour = 0
	CIoTControlPlane  PreferredCIoTNetworkBehaviour = 1
	CIoTUserPlane     PreferredCIoTNetworkBehaviour = 2
	CIoTReservedValue PreferredCIoTNetworkBehaviour = 3
)

func (p PreferredCIoTNetworkBehaviour) String() string {
	return enumString(uint8(p), map[uint8]string{
		uint8(CIoTNoPreference):  "no additional information",
		uint8(CIoTControlPlane):  "control plane CIoT optimization",
		uint8(CIoTUserPlane):     "user plane CIoT optimization",
		uint8(CIoTReservedValue): "reserved",
	})
}

// UpdateType5GS is the 5GS update type information element value
// (TS 24.501 §9.11.3.9A).
type UpdateType5GS struct {
	SMSRequested bool                          // octet 3 bit 1
	NGRANRCU     bool                          // octet 3 bit 2, NG-RAN radio capability update
	PNBCIoT5GS   PreferredCIoTNetworkBehaviour // octet 3 bits 3-4
	PNBCIoTEPS   PreferredCIoTNetworkBehaviour // octet 3 bits 5-6
}

// ParseUpdateType5GS decodes a 5GS update type IE value.
func ParseUpdateType5GS(b []byte) (UpdateType5GS, error) {
	if len(b) != 1 {
		return UpdateType5GS{}, fmt.Errorf("nas/fgs: 5GS update type is %d octets, want 1", len(b))
	}

	return UpdateType5GS{
		SMSRequested: b[0]&0x01 != 0,
		NGRANRCU:     b[0]&0x02 != 0,
		PNBCIoT5GS:   PreferredCIoTNetworkBehaviour(b[0] >> 2 & 0x03),
		PNBCIoTEPS:   PreferredCIoTNetworkBehaviour(b[0] >> 4 & 0x03),
	}, nil
}

// AppendBinary encodes the 5GS update type IE value onto b.
func (u UpdateType5GS) AppendBinary(b []byte) ([]byte, error) {
	return append(b, boolBit(u.SMSRequested, 0)|boolBit(u.NGRANRCU, 1)|
		uint8(u.PNBCIoT5GS)&0x03<<2|uint8(u.PNBCIoTEPS)&0x03<<4), nil
}

// MarshalBinary encodes the 5GS update type IE value.
func (u UpdateType5GS) MarshalBinary() ([]byte, error) { return u.AppendBinary(nil) }

// DRXValue is the DRX cycle a UE requests (TS 24.501 §9.11.3.2A, table
// 9.11.3.2A.1). Values above 3 are reserved.
type DRXValue uint8

func (v DRXValue) String() string {
	return enumString(uint8(v), map[uint8]string{
		uint8(DRXValueNotSpecified):  "not specified",
		uint8(DRXCycleParameterT32):  "T = 32",
		uint8(DRXCycleParameterT64):  "T = 64",
		uint8(DRXCycleParameterT128): "T = 128",
		uint8(DRXCycleParameterT256): "T = 256",
	})
}

// DRXParameter is the 5GS DRX parameters information element value
// (TS 24.501 §9.11.3.2A): the DRX cycle the UE requests, in bits 1-4 of its one
// content octet.
type DRXParameter struct {
	Value DRXValue
}

// ParseDRXParameter decodes a 5GS DRX parameters IE value.
func ParseDRXParameter(b []byte) (DRXParameter, error) {
	if len(b) != 1 {
		return DRXParameter{}, fmt.Errorf("nas/fgs: 5GS DRX parameters is %d octets, want 1", len(b))
	}

	return DRXParameter{Value: DRXValue(b[0] & 0x0F)}, nil
}

// AppendBinary encodes the 5GS DRX parameters IE value onto b.
func (d DRXParameter) AppendBinary(b []byte) ([]byte, error) {
	return append(b, uint8(d.Value)&0x0F), nil
}

// MarshalBinary encodes the 5GS DRX parameters IE value.
func (d DRXParameter) MarshalBinary() ([]byte, error) { return d.AppendBinary(nil) }
