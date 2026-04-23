package ue

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/service_request/data",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runServiceRequestData(ctx, env, params)
		},
	})
}

func runServiceRequestData(_ context.Context, env scenarios.Env, _ any) error {
	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         scenarios.DefaultGNBID,
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SST:           scenarios.DefaultSST,
		SD:            scenarios.DefaultSD,
		DNN:           scenarios.DefaultDNN,
		TAC:           scenarios.DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.FirstCore(),
		GnbN2Address:  g.N2Address,
		GnbN3Address:  g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	newUE, err := newDefaultUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber)
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  int64(scenarios.DefaultRANUENGAPID),
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	pduSessionStatus := [16]bool{}
	pduSessionStatus[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID)),
		RANUENGAPID:   int64(scenarios.DefaultRANUENGAPID),
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionStatus,
	})
	if err != nil {
		return fmt.Errorf("UEContextReleaseProcedure failed: %v", err)
	}

	err = runServiceRequestOnUE(int64(scenarios.DefaultRANUENGAPID), pduSessionStatus, newUE)
	if err != nil {
		return fmt.Errorf("service request procedure failed: %v", err)
	}

	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID)),
		RANUENGAPID: int64(scenarios.DefaultRANUENGAPID),
	})
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}

func runServiceRequestOnUE(ranUENGAPID int64, pduSessionStatus [16]bool, u *ue.UE) error {
	err := u.SendServiceRequest(ranUENGAPID, pduSessionStatus, nasMessage.ServiceTypeData)
	if err != nil {
		return fmt.Errorf("could not send Service Request NAS message: %v", err)
	}

	_, err = u.WaitForNASGMMMessage(nas.MsgTypeServiceAccept, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive Service Accept NAS message: %v", err)
	}

	return nil
}
