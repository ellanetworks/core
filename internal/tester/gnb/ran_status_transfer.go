// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

type UplinkRANStatusTransferOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	Container   []byte
}

func BuildUplinkRANStatusTransfer(opts *UplinkRANStatusTransferOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("UplinkRANStatusTransferOpts is nil")
	}

	msg := &ngap.UplinkRANStatusTransfer{
		AMFUENGAPID: ngap.AMFUENGAPID(opts.AMFUENGAPID),
		RANUENGAPID: ngap.RANUENGAPID(opts.RANUENGAPID),
		Container:   ngap.StatusTransferContainer(opts.Container),
	}

	return msg.Marshal()
}

func (g *GnodeB) SendUplinkRANStatusTransfer(opts *UplinkRANStatusTransferOpts) error {
	pdu, err := BuildUplinkRANStatusTransfer(opts)
	if err != nil {
		return fmt.Errorf("couldn't build UplinkRANStatusTransfer: %w", err)
	}

	if err := g.SendMessage(pdu, NGAPProcedureUplinkRANStatusTransfer); err != nil {
		return fmt.Errorf("couldn't send UplinkRANStatusTransfer: %w", err)
	}

	logger.GnbLogger.Debug("Sent Uplink RAN Status Transfer",
		zap.Int64("AMF UE NGAP ID", opts.AMFUENGAPID),
		zap.Int64("RAN UE NGAP ID", opts.RANUENGAPID),
	)

	return nil
}

func (g *GnodeB) WaitForDownlinkRANStatusTransfer(timeout time.Duration) (*ngap.DownlinkRANStatusTransfer, error) {
	frame, err := g.WaitForMessage(Initiating, ngap.ProcDownlinkRANStatusTransfer, timeout)
	if err != nil {
		return nil, fmt.Errorf("gnb: await Downlink RAN Status Transfer: %w", err)
	}

	transfer, err := ngap.ParseDownlinkRANStatusTransfer(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("gnb: parse Downlink RAN Status Transfer: %w", err)
	}

	return transfer, nil
}
