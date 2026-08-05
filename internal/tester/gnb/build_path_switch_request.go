// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/ngap"
)

type PathSwitchRequestOpts struct {
	RANUENGAPID       int64
	SourceAMFUENGAPID int64
	PDUSessions       [16]*PDUSessionInformation
	N3GnbIp           netip.Addr
	// UESecurityCapabilities is mandatory in a PATH SWITCH REQUEST
	// (TS 38.413 §9.2.3.4), so it is a value rather than an optional pointer.
	// internal/tester/s1enb passes its S1AP counterpart by value too.
	UESecurityCapabilities ngap.UESecurityCapabilities
	Mcc                    string
	Mnc                    string
	Tac                    string
	GnbID                  string
}

// BuildPathSwitchRequest encodes a PATH SWITCH REQUEST PDU (TS 38.413 §8.4.4),
// which the target NG-RAN node sends after an Xn handover to move the downlink
// tunnels of every session it took over.
func BuildPathSwitchRequest(opts *PathSwitchRequestOpts) ([]byte, error) {
	uli, err := userLocation(opts.Mcc, opts.Mnc, opts.GnbID, opts.Tac)
	if err != nil {
		return nil, err
	}

	addr, err := transportLayerAddress(opts.N3GnbIp)
	if err != nil {
		return nil, err
	}

	var dlList ngap.PDUSessionResourceToBeSwitchedDLList

	for _, pduSession := range opts.PDUSessions {
		if pduSession == nil {
			continue
		}

		transfer, err := buildPathSwitchRequestTransfer(pduSession.DLTeid, addr)
		if err != nil {
			return nil, fmt.Errorf("failed to build PathSwitchRequestTransfer: %w", err)
		}

		dlList = append(dlList, ngap.PDUSessionResourceToBeSwitchedDLItem{
			PDUSessionID: ngap.PDUSessionID(pduSession.PDUSessionID),
			Transfer:     transfer,
		})
	}

	msg := &ngap.PathSwitchRequest{
		RANUENGAPID:                          ngap.RANUENGAPID(opts.RANUENGAPID),
		SourceAMFUENGAPID:                    ngap.AMFUENGAPID(opts.SourceAMFUENGAPID),
		UserLocationInformation:              &uli,
		UESecurityCapabilities:               &opts.UESecurityCapabilities,
		PDUSessionResourceToBeSwitchedDLList: dlList,
	}

	return msg.Marshal()
}

// buildPathSwitchRequestTransfer encodes the per-session transfer naming the
// downlink tunnel the target node has set up (TS 38.413 §9.3.4.9). One accepted
// QoS flow is reported: the list is mandatory and this simulator serves one.
func buildPathSwitchRequestTransfer(teid uint32, addr ngap.TransportLayerAddress) (ngap.TransferContainer, error) {
	transfer := &ngap.PathSwitchRequestTransfer{
		DLNGUUPTNLInformation: ngap.UPTransportLayerInformation{
			GTPTunnel: ngap.GTPTunnel{TransportLayerAddress: addr, GTPTEID: ngap.GTPTEID(teid)},
		},
		QosFlowAccepted: ngap.QosFlowAcceptedList{{QosFlowIdentifier: 1}},
	}

	return transfer.Marshal()
}
