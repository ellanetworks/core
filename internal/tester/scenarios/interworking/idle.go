// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_5gs_to_eps",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runIdle5GSToEPS,
		Fixture:   fixture,
	})
}

func runIdle5GSToEPS(ctx context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	u, err := newInterworkingUE(gNodeB, true)
	if err != nil {
		return err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, u)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: movedPDUSessionID,
		UE:           u,
	}); err != nil {
		return fmt.Errorf("initial registration over NR: %w", err)
	}

	before, err := probeOver5GS(ctx, env, gNodeB, u, ranUENGAPID, "over N3 before the idle move")
	if err != nil {
		return err
	}

	if err := provisionEPSNASAlgorithms(gNodeB, u, ranUENGAPID); err != nil {
		return err
	}

	if err := goIdleOnNR(gNodeB, u); err != nil {
		return err
	}

	security, guti, err := idleMobilityMaterial(u)
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)

	var bearerStatus nas.EPSBearerContextStatus

	bearerStatus.Active[movedEPSBearerIdentity] = true

	res, err := e.TrackingAreaUpdateFrom5GS(epsUE, s1enb.IdleTrackingAreaUpdateOpts{
		GUTI:         guti,
		ActiveFlag:   true,
		BearerStatus: &bearerStatus,
		Security:     security,
	}, attachTimeout)
	if err != nil {
		return fmt.Errorf("tracking area update after the idle move to EPS: %w", err)
	}

	if err := assertAdoptedBearer(res); err != nil {
		return err
	}

	after, err := probeAfterHandover(ctx, env, e, handoverBearer{
		upfAddress: res.UpfAddress,
		ulTEID:     res.ULTEID,
		dlTEID:     res.DLTEID,
		mmeUEID:    res.MMEUES1APID,
		epsUE:      epsUE,
	}, before.addrs)
	if err != nil {
		return err
	}

	return assertContinuity(before, after)
}

func goIdleOnNR(gNodeB *gnb.GnodeB, u *ue.UE) error {
	var sessions [16]bool

	sessions[movedPDUSessionID] = true

	if err := procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(mobilityRANUENGAPID),
		RANUENGAPID:   mobilityRANUENGAPID,
		GnodeB:        gNodeB,
		UE:            u,
		PDUSessionIDs: sessions,
	}); err != nil {
		return fmt.Errorf("release the NR connection before the inter-system change: %w", err)
	}

	return nil
}

func idleMobilityMaterial(u *ue.UE) (s1enb.IdleMobilityFrom5GS, eps.GUTI, error) {
	var none s1enb.IdleMobilityFrom5GS

	if u.UeSecurity.Guti == nil || u.UeSecurity.Guti.GUTI == nil {
		return none, eps.GUTI{}, errors.New("the UE holds no 5G-GUTI to map into a tracking area update")
	}

	if u.UeSecurity.EPSNASAlgorithms == nil {
		return none, eps.GUTI{}, errors.New("the AMF signalled no EPS NAS algorithms, so no mapped context can be derived")
	}

	return s1enb.IdleMobilityFrom5GS{
		KAMF:             u.UeSecurity.Kamf,
		KNASInt:          u.UeSecurity.KnasInt,
		NIA:              u.UeSecurity.IntegrityAlg,
		UplinkNASCount:   u.UeSecurity.ULCount,
		DownlinkNASCount: u.UeSecurity.DLCount,
		EPSCiphering:     uint8(u.UeSecurity.EPSNASAlgorithms.Ciphering),
		EPSIntegrity:     uint8(u.UeSecurity.EPSNASAlgorithms.Integrity),
		EKSI:             uint8(u.UeSecurity.NgKsi.Ksi),
	}, etsi.MapGUTI5GToEPS(*u.UeSecurity.Guti.GUTI), nil
}

func assertAdoptedBearer(res *s1enb.AttachResult) error {
	if res.BearerStatus == nil {
		return errors.New("the tracking area update accept carried no EPS bearer context status, so the UE cannot tell which session survived")
	}

	if !res.BearerStatus.Active[movedEPSBearerIdentity] {
		return fmt.Errorf("EPS bearer context status = %v, want EBI %d active: the PDU session did not become a PDN connection",
			res.BearerStatus.Active, movedEPSBearerIdentity)
	}

	if res.GUTI == nil {
		return errors.New("the tracking area update accept reallocated no GUTI")
	}

	return nil
}
