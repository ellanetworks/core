package procedure

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/free5gc/ngap/ngapType"
)

type UEContextReleaseOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	GnodeB        *gnb.GnodeB
	UE            *ue.UE
	PDUSessionIDs [16]bool
}

func UEContextRelease(opts *UEContextReleaseOpts) error {
	err := opts.GnodeB.SendUEContextReleaseRequest(&gnb.UEContextReleaseRequestOpts{
		AMFUENGAPID:   opts.AMFUENGAPID,
		RANUENGAPID:   opts.RANUENGAPID,
		PDUSessionIDs: opts.PDUSessionIDs,
		Cause:         ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason,
	})
	if err != nil {
		return fmt.Errorf("could not send UEContextReleaseComplete: %v", err)
	}

	err = opts.UE.WaitForRRCRelease(1 * time.Second)
	if err != nil {
		return fmt.Errorf("did not receive RRC Release for UE %s: %v", opts.UE.UeSecurity.Supi, err)
	}

	return nil
}
