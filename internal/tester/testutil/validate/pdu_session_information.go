package validate

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
)

type ExpectedPDUSessionInformation struct {
	FiveQi int64
	PriArp int64
	QFI    int64
}

func PDUSessionInformation(got *gnb.PDUSessionInformation, expected *ExpectedPDUSessionInformation) error {
	if got.FiveQi != expected.FiveQi {
		return fmt.Errorf("unexpected NGAP 5QI: got %d, expected %d", got.FiveQi, expected.FiveQi)
	}

	if got.PriArp != expected.PriArp {
		return fmt.Errorf("unexpected NGAP ARP Priority Level: got %d, expected %d", got.PriArp, expected.PriArp)
	}

	if got.QFI != expected.QFI {
		return fmt.Errorf("unexpected NGAP QoS Flow Identifier: got %d, expected %d", got.QFI, expected.QFI)
	}

	return nil
}
