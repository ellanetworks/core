// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

// FuzzDecodeNoPanic asserts the envelope and container decoders never panic on
// arbitrary input. Where a PDU decodes, its value is also parsed as a
// ProtocolIE-Container, exercising the full Phase 2 decode path.
func FuzzDecodeNoPanic(f *testing.F) {
	f.Add([]byte{0x00, 0x11, 0x00, 0x01, 0xab})
	f.Add([]byte{0x00, 0x11, 0x00, 0x02, 0x00, 0x00})
	f.Add([]byte{0x20, 0x0a, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = unmarshalPERValue[Cause](data)
		_, _ = unmarshalPERValue[CriticalityDiagnostics](data)

		pdu, err := Unmarshal(data)
		if err != nil {
			return
		}

		_, _ = decodeIEContainer(per.NewReader(pdu.value()), per.Aligned)

		// Every parser in the registry sees the payload, so a newly added
		// message cannot silently escape the no-panic guarantee.
		for _, mp := range messageParsers {
			_ = mp.Parse(pdu.value())
		}
	})
}
