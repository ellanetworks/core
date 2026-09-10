// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const modificationRANUENGAPID = scenarios.DefaultRANUENGAPID + 1

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_eps_to_5gs_session_modification",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runIdleEPSTo5GSSessionModification,
		Fixture:   fixture,
	})
}

func runIdleEPSTo5GSSessionModification(ctx context.Context, env scenarios.Env, _ any) error {
	e, err := startENBOnSecondaryN3(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)
	epsUE.RequestPDNType(uint8(eps.PDNTypeIPv4v6))
	epsUE.AnnounceN1Mode(movedPDUSessionID)

	res, err := e.Attach(epsUE, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach over E-UTRAN: %w", err)
	}

	before, err := probeOverEPS(ctx, env, e, res, "before the idle move to 5GS")
	if err != nil {
		return err
	}

	if res.GUTI == nil || res.GUTI.GUTI == nil {
		return errors.New("the attach accept assigned no GUTI to map into a registration")
	}

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	u, err := newInterworkingUE(gNodeB, false)
	if err != nil {
		return err
	}

	ranUENGAPID := int64(modificationRANUENGAPID)

	if err := arriveOn5GSFromEPS(gNodeB, epsUE, u, *res.GUTI.GUTI, ranUENGAPID, arriveAndResumeUserPlane); err != nil {
		return err
	}

	if err := assertSessionOn(ctx, env, "5G", before.addrs); err != nil {
		return err
	}

	if err := indicate5GSMParameters(gNodeB, u, ranUENGAPID); err != nil {
		return err
	}

	if err := refuseQoSRequest(gNodeB, u, ranUENGAPID); err != nil {
		return err
	}

	return assertSessionOn(ctx, env, "5G", before.addrs)
}

func indicate5GSMParameters(gNodeB *gnb.GnodeB, u *ue.UE, ranUENGAPID int64) error {
	command, err := gNodeB.ModifyPDUSession(u, ranUENGAPID, &ue.PDUSessionModificationRequestOpts{
		PDUSessionID:      movedPDUSessionID,
		ReflectiveQoS:     true,
		MultiHomedIPv6:    true,
		MaxPacketFilters:  64,
		IntegrityMaxRate:  true,
		AlwaysOnRequested: true,
		RequestDNSServer:  true,
	}, registrationTimeout)
	if err != nil {
		return fmt.Errorf("indicating the 5GSM parameters after the inter-system change: %w", err)
	}

	if command.AlwaysOn == nil {
		return errors.New("the UE requested an always-on PDU session but the modification command carries no " +
			"always-on indication, which TS 24.501 §6.3.2.2 b) 1) requires")
	}

	if *command.AlwaysOn {
		return errors.New("the modification command grants an always-on PDU session, which this core never allows")
	}

	dns, err := commandDNSServer(command)
	if err != nil {
		return err
	}

	logger.Logger.Info("The network answered the UE's 5GSM parameter indication",
		zap.Uint8("PDU Session ID", movedPDUSessionID),
		zap.String("DNS server", dns.String()),
		zap.Bool("always-on granted", *command.AlwaysOn),
	)

	return nil
}

func commandDNSServer(command *fgs.PDUSessionModificationCommand) (net.IP, error) {
	if command.ExtendedPCO == nil {
		return nil, errors.New("the UE asked for a DNS server address but the modification command carries no " +
			"extended protocol configuration options, which TS 24.501 §6.3.2.2 requires")
	}

	for _, c := range command.ExtendedPCO.Containers {
		switch c.ID {
		case naslib.PCOContainerDNSServerIPv4Address, naslib.PCOContainerDNSServerIPv6Address:
			if addr := net.IP(c.Content); addr.To16() != nil {
				return addr, nil
			}
		}
	}

	return nil, fmt.Errorf("the modification command carries no DNS server address: %+v", command.ExtendedPCO.Containers)
}

func refuseQoSRequest(gNodeB *gnb.GnodeB, u *ue.UE, ranUENGAPID int64) error {
	reject, err := gNodeB.RefusePDUSessionModification(u, ranUENGAPID, &ue.PDUSessionModificationRequestOpts{
		PDUSessionID:      movedPDUSessionID,
		RequestedQoSFlows: fgs.QoSFlowDescriptions{fgs.FiveQIQoSFlow(2, 9, fgs.QoSFlowOpCreate)},
	}, registrationTimeout)
	if err != nil {
		return fmt.Errorf("asking the network to set the session QoS: %w", err)
	}

	if reject.Cause != fgs.GSMCauseFiveGSQoSNotAccepted {
		return fmt.Errorf("the QoS request was refused with 5GSM cause %s, want %s",
			reject.Cause, fgs.GSMCauseFiveGSQoSNotAccepted)
	}

	logger.Logger.Info("The network refused the UE's QoS request with the cause that names the reason",
		zap.Stringer("cause", reject.Cause),
	)

	return nil
}
