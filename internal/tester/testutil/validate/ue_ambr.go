package validate

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
)

type ExpectedUEAmbr struct {
	UplinkBps   int64
	DownlinkBps int64
}

func UEAmbr(got *gnb.UEAmbrInformation, expected *ExpectedUEAmbr) error {
	if got == nil {
		return fmt.Errorf("UE AMBR not received in InitialContextSetupRequest")
	}

	if got.UplinkBps != expected.UplinkBps {
		return fmt.Errorf("unexpected UE AMBR uplink: got %d bps, expected %d bps", got.UplinkBps, expected.UplinkBps)
	}

	if got.DownlinkBps != expected.DownlinkBps {
		return fmt.Errorf("unexpected UE AMBR downlink: got %d bps, expected %d bps", got.DownlinkBps, expected.DownlinkBps)
	}

	return nil
}
