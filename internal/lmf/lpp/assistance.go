// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"fmt"
)

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

	return decoded.Payload()
}
