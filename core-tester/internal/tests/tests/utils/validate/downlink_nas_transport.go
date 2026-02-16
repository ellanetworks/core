package validate

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

type DownlinkNASTransportOpts struct {
	Frame gnb.SCTPFrame
}

func DownlinkNASTransport(opts *DownlinkNASTransportOpts) (*ngapType.DownlinkNASTransport, error) {
	err := utils.ValidateSCTP(opts.Frame.Info, 60, 1)
	if err != nil {
		return nil, fmt.Errorf("SCTP validation failed: %v", err)
	}

	pdu, err := ngap.Decoder(opts.Frame.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode NGAP: %v", err)
	}

	if pdu.InitiatingMessage == nil {
		return nil, fmt.Errorf("NGAP PDU is not a InitiatingMessage")
	}

	if pdu.InitiatingMessage.ProcedureCode.Value != ngapType.ProcedureCodeDownlinkNASTransport {
		return nil, fmt.Errorf("NGAP ProcedureCode is not DownlinkNASTransport (%d), received %d", ngapType.ProcedureCodeDownlinkNASTransport, pdu.InitiatingMessage.ProcedureCode.Value)
	}

	downlinkNASTransport := pdu.InitiatingMessage.Value.DownlinkNASTransport
	if downlinkNASTransport == nil {
		return nil, fmt.Errorf("DownlinkNASTransport is nil")
	}

	return downlinkNASTransport, nil
}
