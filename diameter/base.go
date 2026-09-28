// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

const PPID uint32 = 46

const RelayApplicationID uint32 = 0xffffffff

const (
	CommandCapabilitiesExchange uint32 = 257
	CommandDeviceWatchdog       uint32 = 280
	CommandDisconnectPeer       uint32 = 282
)

const (
	AVPUserName                    uint32 = 1
	AVPHostIPAddress               uint32 = 257
	AVPAuthApplicationID           uint32 = 258
	AVPAcctApplicationID           uint32 = 259
	AVPVendorSpecificApplicationID uint32 = 260
	AVPSessionID                   uint32 = 263
	AVPOriginHost                  uint32 = 264
	AVPSupportedVendorID           uint32 = 265
	AVPVendorID                    uint32 = 266
	AVPResultCode                  uint32 = 268
	AVPProductName                 uint32 = 269
	AVPDisconnectCause             uint32 = 273
	AVPAuthSessionState            uint32 = 277
	AVPOriginStateID               uint32 = 278
	AVPRouteRecord                 uint32 = 282
	AVPFailedAVP                   uint32 = 279
	AVPDestinationRealm            uint32 = 283
	AVPProxyInfo                   uint32 = 284
	AVPInbandSecurityID            uint32 = 299
	AVPDRMP                        uint32 = 301
	AVPDestinationHost             uint32 = 293
	AVPOriginRealm                 uint32 = 296
	AVPExperimentalResult          uint32 = 297
	AVPExperimentalResultCode      uint32 = 298
)

const (
	ResultSuccess                uint32 = 2001
	ResultCommandUnsupported     uint32 = 3001
	ResultUnableToDeliver        uint32 = 3002
	ResultRealmNotServed         uint32 = 3003
	ResultTooBusy                uint32 = 3004
	ResultApplicationUnsupported uint32 = 3007
	ResultInvalidHdrBits         uint32 = 3008
	ResultInvalidAVPBits         uint32 = 3009
	ResultUnknownPeer            uint32 = 3010
	ResultAVPUnsupported         uint32 = 5001
	ResultInvalidAVPValue        uint32 = 5004
	ResultMissingAVP             uint32 = 5005
	ResultAVPOccursTooManyTimes  uint32 = 5009
	ResultNoCommonApplication    uint32 = 5010
	ResultUnsupportedVersion     uint32 = 5011
	ResultUnableToComply         uint32 = 5012
	ResultInvalidAVPLength       uint32 = 5014
	ResultInvalidMessageLength   uint32 = 5015
	ResultNoCommonSecurity       uint32 = 5017
)

const InbandSecurityNone uint32 = 0

const AuthSessionStateNoStateMaintained uint32 = 1

const (
	DisconnectCauseRebooting            uint32 = 0
	DisconnectCauseBusy                 uint32 = 1
	DisconnectCauseDoNotWantToTalkToYou uint32 = 2
)
