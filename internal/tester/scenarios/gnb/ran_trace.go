// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
)

const traceStartTimeout = 5 * time.Second

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ran_trace",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runRANTrace,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
			}
		},
	})
}

func runRANTrace(_ context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	plmn, err := nas.PLMN{MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC}.Octets()
	if err != nil {
		return err
	}

	newUE, err := newDefaultUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, newUE)

	if err := runInitialRegistration(&initialRegistrationOpts{
		RANUENGAPID:            ranUENGAPID,
		PDUSessionID:           scenarios.DefaultPDUSessionID,
		ExpectedPDUSessionType: env.PDUSessionType(),
		UE:                     newUE,
		GnodeB:                 gNodeB,
	}); err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	first, err := expectTraceStart(gNodeB, ranUENGAPID, plmn[:])
	if err != nil {
		return fmt.Errorf("after registration: %w", err)
	}

	if err := gNodeB.ReleaseContext(newUE, ranUENGAPID, []uint8{scenarios.DefaultPDUSessionID}, gnb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("UE context release failed: %v", err)
	}

	if _, err := gNodeB.ServiceRequest(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout, nil); err != nil {
		return fmt.Errorf("service request procedure failed: %v", err)
	}

	second, err := expectTraceStart(gNodeB, ranUENGAPID, plmn[:])
	if err != nil {
		return fmt.Errorf("after service request: %w", err)
	}

	if !bytes.Equal(first[:6], second[:6]) {
		return fmt.Errorf("service request carried Trace Reference %x, registration %x: want one per subscriber", second[:6], first[:6])
	}

	if bytes.Equal(first[6:], second[6:]) {
		return fmt.Errorf("service request reused the Trace Recording Session Reference %x", first[6:])
	}

	if err := gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout); err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}

func expectTraceStart(gNodeB *gnb.GnodeB, ranUENGAPID int64, plmn []byte) (ngap.NGRANTraceID, error) {
	ts, err := gNodeB.WaitForTraceStart(traceStartTimeout)
	if err != nil {
		return ngap.NGRANTraceID{}, err
	}

	if int64(ts.RANUENGAPID) != ranUENGAPID {
		return ngap.NGRANTraceID{}, fmt.Errorf("trace start for RAN UE NGAP ID %d, want %d", ts.RANUENGAPID, ranUENGAPID)
	}

	ta := ts.TraceActivation
	if ta == nil {
		return ngap.NGRANTraceID{}, fmt.Errorf("trace start without a trace activation")
	}

	if !bytes.Equal(ta.NGRANTraceID[:3], plmn) {
		return ngap.NGRANTraceID{}, fmt.Errorf("trace ID %x does not start with the operator PLMN %x", ta.NGRANTraceID, plmn)
	}

	wantInterfaces := ngap.InterfaceToTraceNGC | ngap.InterfaceToTraceXnC | ngap.InterfaceToTraceUu | ngap.InterfaceToTraceF1C | ngap.InterfaceToTraceE1
	if ta.InterfacesToTrace != wantInterfaces || ta.TraceDepth != ngap.TraceDepthMinimum {
		return ngap.NGRANTraceID{}, fmt.Errorf("interfaces %08b depth %d, want %08b minimum", ta.InterfacesToTrace, ta.TraceDepth, wantInterfaces)
	}

	if !ta.TraceCollectionEntityIPAddress.Valid() {
		return ngap.NGRANTraceID{}, fmt.Errorf("no Trace Collection Entity IP Address")
	}

	return ta.NGRANTraceID, nil
}
