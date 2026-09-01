// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const sessionCountPoll = 200 * time.Millisecond

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/pdu_session_release",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runPDUSessionRelease(ctx, env, params)
		},
		Fixture: fixturePDUSessionRelease,
	})
}

func fixturePDUSessionRelease(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

// runPDUSessionRelease exercises the UE-requested PDU session release
// (TS 23.502 §4.3.4.2), the procedure a handset runs whenever it drops one of
// its PDU sessions without leaving the network, and the re-establishment that
// follows it on the same PDU session ID. The UE stays registered throughout.
func runPDUSessionRelease(ctx context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	newUE, err := newDefaultUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := gNodeB.Register(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	if err := awaitSessionCount(ctx, env, scenarios.DefaultIMSI, 1); err != nil {
		return fmt.Errorf("after the initial registration: %w", err)
	}

	if err := gNodeB.ReleasePDUSession(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, releaseTimeout); err != nil {
		return fmt.Errorf("UE-requested PDU session release failed: %v", err)
	}

	logger.Logger.Info("UE-requested PDU session release completed",
		zap.Uint8("PDU Session ID", scenarios.DefaultPDUSessionID),
	)

	if err := awaitSessionCount(ctx, env, scenarios.DefaultIMSI, 0); err != nil {
		return fmt.Errorf("after the UE-requested release: %w", err)
	}

	if ids := newUE.ActivePDUSessionIDs(); len(ids) != 0 {
		return fmt.Errorf("the UE still holds PDU sessions %v after the release", ids)
	}

	// A handset that dropped a PDU session asks for it back on the same PDU
	// session ID, so the release must have freed the identity as well as the
	// session (TS 24.501 §6.4.1.2).
	if _, err := gNodeB.EstablishPDUSession(newUE, ranUENGAPID, scenarios.DefaultPDUSessionID, scenarios.DefaultDNN,
		models.Snssai{Sst: scenarios.DefaultSST, Sd: scenarios.DefaultSD}, registrationTimeout); err != nil {
		return fmt.Errorf("could not re-establish the released PDU session: %v", err)
	}

	if err := awaitSessionCount(ctx, env, scenarios.DefaultIMSI, 1); err != nil {
		return fmt.Errorf("after the re-establishment: %w", err)
	}

	logger.Logger.Info("Released PDU session re-established",
		zap.Uint8("PDU Session ID", scenarios.DefaultPDUSessionID),
	)

	if err := gNodeB.Deregister(newUE, ranUENGAPID, releaseTimeout); err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	return nil
}

// awaitSessionCount polls the core until the subscriber reports want sessions,
// so the assertion covers what the SMF did with its context rather than only
// what the UE saw over NAS.
func awaitSessionCount(ctx context.Context, env scenarios.Env, imsi string, want int) error {
	cl, err := client.New(&client.Config{BaseURL: env.APIAddress})
	if err != nil {
		return fmt.Errorf("create core client: %w", err)
	}

	cl.SetToken(env.APIToken)

	deadline := time.Now().Add(10 * time.Second)

	got := -1

	for {
		sub, err := cl.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
		if err == nil {
			got = len(sub.Sessions)
			if got == want {
				if !sub.Status.Registered {
					return fmt.Errorf("the subscriber reports %d sessions but is no longer registered", got)
				}

				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("the subscriber reports %d sessions, want %d", got, want)
		}

		time.Sleep(sessionCountPoll)
	}
}
