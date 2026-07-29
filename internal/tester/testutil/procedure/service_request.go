// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package procedure

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
)

type ServiceRequestOpts struct {
	PDUSessionStatus [16]bool
	RANUENGAPID      int64
	UE               *ue.UE
}

func ServiceRequest(opts *ServiceRequestOpts) error {
	err := opts.UE.SendServiceRequest(opts.RANUENGAPID, opts.PDUSessionStatus, uint8(fgs.ServiceTypeData))
	if err != nil {
		return fmt.Errorf("could not send Service Request NAS message: %v", err)
	}

	_, err = opts.UE.WaitForNASGMMMessage(uint8(fgs.MsgServiceAccept), 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive Service Accept NAS message: %v", err)
	}

	return nil
}
