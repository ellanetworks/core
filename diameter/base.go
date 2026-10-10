// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import "fmt"

const PPID uint32 = 46

const RelayApplicationID uint32 = 0xffffffff

const (
	CommandCapabilitiesExchange uint32 = 257
	CommandReAuth               uint32 = 258
	CommandAbortSession         uint32 = 274
	CommandSessionTermination   uint32 = 275
	CommandDeviceWatchdog       uint32 = 280
	CommandDisconnectPeer       uint32 = 282
)

const (
	AVPUserName                    uint32 = 1
	AVPFramedIPAddress             uint32 = 8
	AVPClass                       uint32 = 25
	AVPSessionTimeout              uint32 = 27
	AVPCalledStationID             uint32 = 30
	AVPFramedIPv6Prefix            uint32 = 97
	AVPHostIPAddress               uint32 = 257
	AVPAuthApplicationID           uint32 = 258
	AVPAcctApplicationID           uint32 = 259
	AVPVendorSpecificApplicationID uint32 = 260
	AVPRedirectHostUsage           uint32 = 261
	AVPRedirectMaxCacheTime        uint32 = 262
	AVPSessionID                   uint32 = 263
	AVPOriginHost                  uint32 = 264
	AVPSupportedVendorID           uint32 = 265
	AVPVendorID                    uint32 = 266
	AVPResultCode                  uint32 = 268
	AVPProductName                 uint32 = 269
	AVPSessionBinding              uint32 = 270
	AVPSessionServerFailover       uint32 = 271
	AVPDisconnectCause             uint32 = 273
	AVPAuthGracePeriod             uint32 = 276
	AVPAuthSessionState            uint32 = 277
	AVPOriginStateID               uint32 = 278
	AVPRouteRecord                 uint32 = 282
	AVPFailedAVP                   uint32 = 279
	AVPDestinationRealm            uint32 = 283
	AVPProxyInfo                   uint32 = 284
	AVPReAuthRequestType           uint32 = 285
	AVPRedirectHost                uint32 = 292
	AVPAuthorizationLifetime       uint32 = 291
	AVPInbandSecurityID            uint32 = 299
	AVPDRMP                        uint32 = 301
	AVPDestinationHost             uint32 = 293
	AVPTerminationCause            uint32 = 295
	AVPOriginRealm                 uint32 = 296
	AVPExperimentalResult          uint32 = 297
	AVPExperimentalResultCode      uint32 = 298
	AVPSubscriptionID              uint32 = 443
	AVPFinalUnitAction             uint32 = 449
	AVPSubscriptionIDData          uint32 = 444
	AVPSubscriptionIDType          uint32 = 450
)

const (
	ResultSuccess                uint32 = 2001
	ResultCommandUnsupported     uint32 = 3001
	ResultUnableToDeliver        uint32 = 3002
	ResultRealmNotServed         uint32 = 3003
	ResultTooBusy                uint32 = 3004
	ResultLoopDetected           uint32 = 3005
	ResultRedirectIndication     uint32 = 3006
	ResultApplicationUnsupported uint32 = 3007
	ResultInvalidHdrBits         uint32 = 3008
	ResultInvalidAVPBits         uint32 = 3009
	ResultUnknownPeer            uint32 = 3010
	ResultAVPUnsupported         uint32 = 5001
	ResultUnknownSessionID       uint32 = 5002
	ResultAuthorizationRejected  uint32 = 5003
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

var resultNames = map[uint32]string{
	ResultSuccess:                "DIAMETER_SUCCESS",
	ResultCommandUnsupported:     "DIAMETER_COMMAND_UNSUPPORTED",
	ResultUnableToDeliver:        "DIAMETER_UNABLE_TO_DELIVER",
	ResultRealmNotServed:         "DIAMETER_REALM_NOT_SERVED",
	ResultTooBusy:                "DIAMETER_TOO_BUSY",
	ResultLoopDetected:           "DIAMETER_LOOP_DETECTED",
	ResultRedirectIndication:     "DIAMETER_REDIRECT_INDICATION",
	ResultApplicationUnsupported: "DIAMETER_APPLICATION_UNSUPPORTED",
	ResultInvalidHdrBits:         "DIAMETER_INVALID_HDR_BITS",
	ResultInvalidAVPBits:         "DIAMETER_INVALID_AVP_BITS",
	ResultUnknownPeer:            "DIAMETER_UNKNOWN_PEER",
	ResultAVPUnsupported:         "DIAMETER_AVP_UNSUPPORTED",
	ResultUnknownSessionID:       "DIAMETER_UNKNOWN_SESSION_ID",
	ResultAuthorizationRejected:  "DIAMETER_AUTHORIZATION_REJECTED",
	ResultInvalidAVPValue:        "DIAMETER_INVALID_AVP_VALUE",
	ResultMissingAVP:             "DIAMETER_MISSING_AVP",
	ResultAVPOccursTooManyTimes:  "DIAMETER_AVP_OCCURS_TOO_MANY_TIMES",
	ResultNoCommonApplication:    "DIAMETER_NO_COMMON_APPLICATION",
	ResultUnsupportedVersion:     "DIAMETER_UNSUPPORTED_VERSION",
	ResultUnableToComply:         "DIAMETER_UNABLE_TO_COMPLY",
	ResultInvalidAVPLength:       "DIAMETER_INVALID_AVP_LENGTH",
	ResultInvalidMessageLength:   "DIAMETER_INVALID_MESSAGE_LENGTH",
	ResultNoCommonSecurity:       "DIAMETER_NO_COMMON_SECURITY",
}

func ResultName(code uint32) string { return resultNames[code] }

const InbandSecurityNone uint32 = 0

const (
	AuthSessionStateMaintained        uint32 = 0
	AuthSessionStateNoStateMaintained uint32 = 1
)

const (
	DisconnectCauseRebooting            uint32 = 0
	DisconnectCauseBusy                 uint32 = 1
	DisconnectCauseDoNotWantToTalkToYou uint32 = 2
)

var disconnectCauseNames = map[uint32]string{
	DisconnectCauseRebooting:            "REBOOTING",
	DisconnectCauseBusy:                 "BUSY",
	DisconnectCauseDoNotWantToTalkToYou: "DO_NOT_WANT_TO_TALK_TO_YOU",
}

func DisconnectCauseName(cause uint32) string {
	if name, ok := disconnectCauseNames[cause]; ok {
		return name
	}

	return fmt.Sprintf("disconnect cause %d", cause)
}
