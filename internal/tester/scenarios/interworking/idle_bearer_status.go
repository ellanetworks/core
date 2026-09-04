// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	droppedPDUSessionID        = uint8(2)
	droppedEPSBearerIdentity   = s1ap.ERABID(6)
	bearerStatusEnterpriseDNN  = "enterprise"
	bearerStatusEnterprisePool = "10.46.0.0/16"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_eps_to_5gs_bearer_status",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runIdleEPSTo5GSBearerStatus,
		Fixture:   bearerStatusFixture,
	})
}

func bearerStatusFixture(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		DataNetworks: []scenarios.DataNetworkSpec{
			{Name: bearerStatusEnterpriseDNN, IPv4Pool: bearerStatusEnterprisePool, DNS: "8.8.4.4", MTU: scenarios.DefaultMTU},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name:                "iwk-bearer-status-enterprise",
				ProfileName:         scenarios.DefaultProfileName,
				SliceName:           scenarios.DefaultSliceName,
				DataNetworkName:     bearerStatusEnterpriseDNN,
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              7,
				Arp:                 15,
			},
		},
		Subscribers:         []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(interworkingIMSI, "")},
		AssertUsageForIMSIs: []string{interworkingIMSI},
	}
}

func runIdleEPSTo5GSBearerStatus(ctx context.Context, env scenarios.Env, _ any) error {
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

	before, err := probeOverEPS(ctx, env, e, res, "before the move to 5GS")
	if err != nil {
		return err
	}

	pdn, err := e.OpenPDNAnnouncingN1Mode(epsUE, res.MMEUES1APID, res.ENBUES1APID,
		bearerStatusEnterpriseDNN, uint8(eps.PDNTypeIPv4), droppedPDUSessionID, attachTimeout)
	if err != nil {
		return fmt.Errorf("open the second PDN connection: %w", err)
	}

	if pdn.ERABID != droppedEPSBearerIdentity {
		return fmt.Errorf("the second PDN connection took E-RAB %d, want %d", pdn.ERABID, droppedEPSBearerIdentity)
	}

	if err := assertSessionCount(ctx, env, 2, "before the move to 5GS"); err != nil {
		return err
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release the connection before the move to 5GS: %w", err)
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

	if res.GUTI == nil || res.GUTI.GUTI == nil {
		return errors.New("the attach accept assigned no GUTI to map into a registration")
	}

	accept, err := arriveOn5GSReportingBearerStatus(gNodeB, epsUE, u, *res.GUTI.GUTI)
	if err != nil {
		return err
	}

	if err := assertOnlyTheHeldBearerSurvived(accept); err != nil {
		return err
	}

	if err := assertSessionCount(ctx, env, 1, "after the move to 5GS"); err != nil {
		return err
	}

	return assertSessionOn(ctx, env, "5G", before.addrs)
}

func arriveOn5GSReportingBearerStatus(gNodeB *gnb.GnodeB, epsUE *s1enb.UE, u *ue.UE, epsGUTI eps.GUTI) ([]byte, error) {
	var carried nas.EPSBearerContextStatus

	carried.Active[movedEPSBearerIdentity] = true

	container, err := epsUE.BuildTrackingAreaUpdateForContainer(epsGUTI, &carried)
	if err != nil {
		return nil, err
	}

	mapped := epsUE.MappedContextForIdleMobility()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	gNodeB.AddUE(ranUENGAPID, u)

	var sessions [16]bool

	sessions[movedPDUSessionID] = true

	if err := u.SendIdleMobilityRegistration(ue.IdleRegistrationOpts{
		RANUENGAPID:            ranUENGAPID,
		MappedGUTI:             fgs.GUTIIdentity(etsi.MapGUTIEPSTo5G(epsGUTI)),
		EPSNASMessageContainer: container,
		UplinkDataStatus:       &sessions,
		EPSBearerContextStatus: &carried,
		Mapped: ue.MappedFromEPSIdle{
			KASME:          mapped.KASME,
			UplinkNASCount: mapped.UplinkNASCount,
			EKSI:           mapped.EKSI,
			Ciphering:      idleCiphering,
			Integrity:      idleIntegrity,
		},
	}); err != nil {
		return nil, fmt.Errorf("mobility registration update over NR: %w", err)
	}

	accept, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout)
	if err != nil {
		return nil, fmt.Errorf("registration accept for the inter-system change: %w", err)
	}

	time.Sleep(datapathSettle)

	return accept, nil
}

func assertOnlyTheHeldBearerSurvived(plain []byte) error {
	accept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		return fmt.Errorf("parse the registration accept: %w", err)
	}

	if accept.EPSBearerContextStatus == nil {
		return errors.New("the registration accept carried no EPS bearer context status, so the UE cannot tell which bearers the network kept")
	}

	if !accept.EPSBearerContextStatus.Active[movedEPSBearerIdentity] {
		return fmt.Errorf("EPS bearer context status = %+v, want EBI %d active: the UE reported it active",
			accept.EPSBearerContextStatus.Active, movedEPSBearerIdentity)
	}

	if accept.EPSBearerContextStatus.Active[droppedEPSBearerIdentity] {
		return fmt.Errorf("EPS bearer context status reports EBI %d active: the UE reported it inactive and the AMF must release the PDU session it mapped to (TS 23.502 clause 4.11.1.3.3)",
			droppedEPSBearerIdentity)
	}

	if accept.PDUSessionStatus != nil && accept.PDUSessionStatus.PSI[droppedPDUSessionID] {
		return fmt.Errorf("PDU session status reports session %d active, want it released with its EPS bearer", droppedPDUSessionID)
	}

	return nil
}

func assertSessionCount(ctx context.Context, env scenarios.Env, want int, stage string) error {
	cl, err := coreClient(env)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(sessionSettle)

	var got int

	for {
		sub, err := cl.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: interworkingIMSI})
		if err == nil {
			got = len(sub.Sessions)
			if got == want {
				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%s: the subscriber holds %d sessions, want %d", stage, got, want)
		}

		time.Sleep(statusPoll)
	}
}
