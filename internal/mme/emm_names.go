// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/nas/eps"
)

// emmMessageTypeName renders an EMM message type for logging (TS 24.301).
// It covers types the eps codec does not yet model (e.g. tracking area
// updating) so dropped messages are identifiable in logs.
func emmMessageTypeName(mt eps.MessageType) string {
	switch uint8(mt) {
	case 0x41:
		return "AttachRequest"
	case 0x42:
		return "AttachAccept"
	case 0x43:
		return "AttachComplete"
	case 0x44:
		return "AttachReject"
	case 0x45:
		return "DetachRequest"
	case 0x46:
		return "DetachAccept"
	case 0x48:
		return "TrackingAreaUpdateRequest"
	case 0x49:
		return "TrackingAreaUpdateAccept"
	case 0x4a:
		return "TrackingAreaUpdateComplete"
	case 0x4b:
		return "TrackingAreaUpdateReject"
	case 0x4c:
		return "ExtendedServiceRequest"
	case 0x4d:
		return "ControlPlaneServiceRequest"
	case 0x4e:
		return "ServiceReject"
	case 0x50:
		return "GUTIReallocationCommand"
	case 0x51:
		return "GUTIReallocationComplete"
	case 0x52:
		return "AuthenticationRequest"
	case 0x53:
		return "AuthenticationResponse"
	case 0x54:
		return "AuthenticationReject"
	case 0x55:
		return "IdentityRequest"
	case 0x56:
		return "IdentityResponse"
	case 0x5c:
		return "AuthenticationFailure"
	case 0x5d:
		return "SecurityModeCommand"
	case 0x5e:
		return "SecurityModeComplete"
	case 0x5f:
		return "SecurityModeReject"
	case 0x60:
		return "EMMStatus"
	case 0x61:
		return "EMMInformation"
	default:
		return fmt.Sprintf("0x%02x", uint8(mt))
	}
}
