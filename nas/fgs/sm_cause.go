// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

// 5GSM cause values (TS 24.501 §9.11.4.2, table 9.11.4.2.1).
const (
	Cause5GSMInsufficientResources                        uint8 = 0x1A
	Cause5GSMMissingOrUnknownDNN                          uint8 = 0x1B
	Cause5GSMUnknownPDUSessionType                        uint8 = 0x1C
	Cause5GSMRequestRejectedUnspecified                   uint8 = 0x1F
	Cause5GSMRegularDeactivation                          uint8 = 0x24
	Cause5GSMReactivationRequested                        uint8 = 0x27
	Cause5GSMInvalidPDUSessionIdentity                    uint8 = 0x2B
	Cause5GSMPTIMismatch                                  uint8 = 0x2F
	Cause5GSMPDUSessionTypeIPv4OnlyAllowed                uint8 = 0x32
	Cause5GSMPDUSessionTypeIPv6OnlyAllowed                uint8 = 0x33
	Cause5GSMMissingOrUnknownDNNInASlice                  uint8 = 0x46
	Cause5GSMInvalidPTIValue                              uint8 = 0x51
	Cause5GSMMessageTypeNonExistentOrNotImplemented       uint8 = 0x61
	Cause5GSMMessageTypeNotCompatibleWithTheProtocolState uint8 = 0x62
	Cause5GSMProtocolErrorUnspecified                     uint8 = 0x6F
)
