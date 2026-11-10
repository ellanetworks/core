// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	ranTraceIMSI      = "001017271246614"
	traceStartTimeout = 5 * time.Second
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/ran_trace",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBRANTrace,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(ranTraceIMSI, "")},
			}
		},
	})
}

func runS1ENBRANTrace(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	plmn, err := nas.PLMN{MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC}.Octets()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(ranTraceIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	first, err := expectS1TraceStart(e, res.ENBUES1APID, plmn[:])
	if err != nil {
		return fmt.Errorf("after attach: %w", err)
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI, cannot service-request")
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release to ECM-IDLE: %w", err)
	}

	sr, err := e.ServiceRequest(ue, res.GUTI, releaseTimeout, nil)
	if err != nil {
		return fmt.Errorf("service request: %w", err)
	}

	second, err := expectS1TraceStart(e, sr.ENBUES1APID, plmn[:])
	if err != nil {
		return fmt.Errorf("after service request: %w", err)
	}

	if !bytes.Equal(first[:6], second[:6]) {
		return fmt.Errorf("service request carried Trace Reference %x, attach %x: want one per subscriber", second[:6], first[:6])
	}

	if bytes.Equal(first[6:], second[6:]) {
		return fmt.Errorf("service request reused the Trace Recording Session Reference %x", first[6:])
	}

	return nil
}

func expectS1TraceStart(e *s1enb.ENB, enbUEID int64, plmn []byte) (s1ap.EUTRANTraceID, error) {
	ts, err := e.WaitForTraceStart(enbUEID, traceStartTimeout)
	if err != nil {
		return s1ap.EUTRANTraceID{}, err
	}

	ta := ts.TraceActivation
	if ta == nil {
		return s1ap.EUTRANTraceID{}, fmt.Errorf("trace start without a trace activation")
	}

	if !bytes.Equal(ta.EUTRANTraceID[:3], plmn) {
		return s1ap.EUTRANTraceID{}, fmt.Errorf("trace ID %x does not start with the operator PLMN %x", ta.EUTRANTraceID, plmn)
	}

	wantInterfaces := s1ap.InterfaceToTraceS1MME | s1ap.InterfaceToTraceX2 | s1ap.InterfaceToTraceUu | s1ap.InterfaceToTraceF1C | s1ap.InterfaceToTraceE1
	if ta.InterfacesToTrace != wantInterfaces || ta.TraceDepth != s1ap.TraceDepthMinimum {
		return s1ap.EUTRANTraceID{}, fmt.Errorf("interfaces %08b depth %d, want %08b minimum", ta.InterfacesToTrace, ta.TraceDepth, wantInterfaces)
	}

	if len(ta.TraceCollectionEntityIPAddress) == 0 {
		return s1ap.EUTRANTraceID{}, fmt.Errorf("no Trace Collection Entity IP Address")
	}

	return ta.EUTRANTraceID, nil
}
