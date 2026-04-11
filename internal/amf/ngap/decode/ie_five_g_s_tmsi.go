// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// decodeFiveGSTMSI enforces the field widths from 3GPP TS 38.413
// §9.3.1.13 (AMFSetID: 10 bits, AMFPointer: 6 bits, 5G-TMSI: 4 octets)
// and copies bytes out so the result is independent of the PDU buffer.
func decodeFiveGSTMSI(in *ngapType.FiveGSTMSI) (FiveGSTMSI, bool) {
	if in == nil {
		return FiveGSTMSI{}, false
	}

	if in.AMFSetID.Value.BitLength != 10 {
		return FiveGSTMSI{}, false
	}

	if in.AMFPointer.Value.BitLength != 6 {
		return FiveGSTMSI{}, false
	}

	if len(in.FiveGTMSI.Value) != 4 {
		return FiveGSTMSI{}, false
	}

	out := FiveGSTMSI{
		AMFSetID: aper.BitString{
			Bytes:     append([]byte(nil), in.AMFSetID.Value.Bytes...),
			BitLength: in.AMFSetID.Value.BitLength,
		},
		AMFPointer: aper.BitString{
			Bytes:     append([]byte(nil), in.AMFPointer.Value.Bytes...),
			BitLength: in.AMFPointer.Value.BitLength,
		},
		FiveGTMSI: append([]byte(nil), in.FiveGTMSI.Value...),
	}

	return out, true
}
