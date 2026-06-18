// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap/aper"
)

// FuzzDecodeNoPanic asserts the envelope and container decoders never panic on
// arbitrary input. Where a PDU decodes, its value is also parsed as a
// ProtocolIE-Container, exercising the full Phase 2 decode path.
func FuzzDecodeNoPanic(f *testing.F) {
	f.Add([]byte{0x00, 0x11, 0x00, 0x01, 0xab})
	f.Add([]byte{0x00, 0x11, 0x00, 0x02, 0x00, 0x00})
	f.Add([]byte{0x20, 0x0a, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = decodeCause(aper.NewReader(data))
		_, _ = decodeCriticalityDiagnostics(aper.NewReader(data))

		pdu, err := Unmarshal(data)
		if err != nil {
			return
		}

		_, _ = decodeIEContainer(aper.NewReader(pdu.value()))
		_, _ = ParseS1SetupRequest(pdu.value())
		_, _ = ParseS1SetupResponse(pdu.value())
		_, _ = ParseS1SetupFailure(pdu.value())
		_, _ = ParseInitialUEMessage(pdu.value())
		_, _ = ParseUplinkNASTransport(pdu.value())
		_, _ = ParseDownlinkNASTransport(pdu.value())
		_, _ = ParseInitialContextSetupRequest(pdu.value())
		_, _ = ParseInitialContextSetupResponse(pdu.value())
		_, _ = ParseInitialContextSetupFailure(pdu.value())
		_, _ = ParseUEContextReleaseCommand(pdu.value())
		_, _ = ParseUEContextReleaseComplete(pdu.value())
		_, _ = ParseUEContextReleaseRequest(pdu.value())
		_, _ = ParseErrorIndication(pdu.value())
		_, _ = ParsePaging(pdu.value())
	})
}
