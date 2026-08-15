// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package validate

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
)

type ExpectedPDUSessionInformation struct {
	FiveQI int64
	ARP    int64
	QFI    uint8
}

// PDUSessionInformation checks the QoS the network gave a PDU session, as the
// gNB reported it back to the scenario.
func PDUSessionInformation(got gnb.PDUSessionResult, expected *ExpectedPDUSessionInformation) error {
	if got.FiveQI != expected.FiveQI {
		return fmt.Errorf("unexpected NGAP 5QI: got %d, expected %d", got.FiveQI, expected.FiveQI)
	}

	if got.ARP != expected.ARP {
		return fmt.Errorf("unexpected NGAP ARP Priority Level: got %d, expected %d", got.ARP, expected.ARP)
	}

	if got.QFI != expected.QFI {
		return fmt.Errorf("unexpected NGAP QoS Flow Identifier: got %d, expected %d", got.QFI, expected.QFI)
	}

	return nil
}
