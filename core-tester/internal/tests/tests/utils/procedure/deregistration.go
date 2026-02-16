package procedure

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/ue"
)

type DeregistrationOpts struct {
	UE          *ue.UE
	AMFUENGAPID int64
	RANUENGAPID int64
}

func Deregistration(opts *DeregistrationOpts) error {
	err := opts.UE.SendDeregistrationRequest(opts.AMFUENGAPID, opts.RANUENGAPID)
	if err != nil {
		return fmt.Errorf("could not build Deregistration Request NAS PDU: %v", err)
	}

	err = opts.UE.WaitForRRCRelease(2 * time.Second)
	if err != nil {
		return fmt.Errorf("did not receive RRC Release for UE %s: %v", opts.UE.UeSecurity.Supi, err)
	}

	return nil
}
