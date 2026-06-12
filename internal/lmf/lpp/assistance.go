// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"fmt"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
)

// BuildAssistanceData constructs a ProvideAssistanceData message.
// For the MVP, assistance data is a placeholder; real ephemeris is Phase 3+.
func BuildAssistanceData(transactionID byte) ([]byte, error) {
	return EncodeProvideAssistanceData(transactionID, nil)
}

// BuildRequestLocationInfo constructs an LPP RequestLocationInformation message
// requesting a GNSS location estimate.
func BuildRequestLocationInfo(transactionID byte, method uint8) ([]byte, error) {
	_ = method // All methods use GNSS for MVP
	return EncodeRequestLocationInformation(transactionID)
}

// ParseLPPMessage decodes an APER-encoded LPP message and returns the
// appropriate model struct based on the message body type.
func ParseLPPMessage(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty LPP payload")
	}

	decoded, err := DecodeLPPMessage(data)
	if err != nil {
		return nil, fmt.Errorf("decode LPP message: %w", err)
	}

	switch decoded.BodyKind {
	case lpptype.LPPMessageBodyC1PresentProvideCapabilities:
		return decoded.ProvideCapabilities, nil
	case lpptype.LPPMessageBodyC1PresentProvideLocationInformation:
		return decoded.ProvideLocationInformation, nil
	case lpptype.LPPMessageBodyC1PresentRequestCapabilities:
		return decoded.RequestCapabilities, nil
	case lpptype.LPPMessageBodyC1PresentRequestLocationInformation:
		return decoded.RequestLocationInformation, nil
	case lpptype.LPPMessageBodyC1PresentProvideAssistanceData:
		return decoded.ProvideAssistanceData, nil
	case 0:
		return nil, fmt.Errorf("LPP message has no body")
	default:
		return nil, fmt.Errorf("unsupported LPP message body kind: %d", decoded.BodyKind)
	}
}
