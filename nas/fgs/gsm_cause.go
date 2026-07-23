// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

// 5GSM cause values (TS 24.501 §9.11.4.2, table 9.11.4.2.1).
const (
	GSMCauseInsufficientResources                        uint8 = 0x1A
	GSMCauseMissingOrUnknownDNN                          uint8 = 0x1B
	GSMCauseUnknownPDUSessionType                        uint8 = 0x1C
	GSMCauseRequestRejectedUnspecified                   uint8 = 0x1F
	GSMCauseRegularDeactivation                          uint8 = 0x24
	GSMCauseReactivationRequested                        uint8 = 0x27
	GSMCauseInvalidPDUSessionIdentity                    uint8 = 0x2B
	GSMCausePTIMismatch                                  uint8 = 0x2F
	GSMCausePDUSessionTypeIPv4OnlyAllowed                uint8 = 0x32
	GSMCausePDUSessionTypeIPv6OnlyAllowed                uint8 = 0x33
	GSMCauseMissingOrUnknownDNNInASlice                  uint8 = 0x46
	GSMCauseInvalidPTIValue                              uint8 = 0x51
	GSMCauseMessageTypeNonExistentOrNotImplemented       uint8 = 0x61
	GSMCauseMessageTypeNotCompatibleWithTheProtocolState uint8 = 0x62
	GSMCauseProtocolErrorUnspecified                     uint8 = 0x6F
)
