// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/goleak"
)

var ngSetupScenario = benchScenario{
	rat:            rat5G,
	radioPerWorker: true,
	cycle: func(w *benchWorker, s samples) error {
		start := time.Now()

		if err := w.gnb.SendNGSetupRequest(&w.setup); err != nil {
			return fmt.Errorf("send NG Setup Request: %w", err)
		}

		if _, err := w.gnb.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, e2eBenchTimeout); err != nil {
			return fmt.Errorf("wait for NG Setup Response: %w", err)
		}

		s.add("ngsetup", time.Since(start))

		return nil
	},
}

var registrationScenario = benchScenario{
	rat:            rat5G,
	withSubscriber: true,
	cycle: func(w *benchWorker, s samples) error {
		w.gnb.DiscardReceivedFrames()

		u, err := newBenchUE(w.gnb, w.imsi, false)
		if err != nil {
			return fmt.Errorf("create UE: %w", err)
		}

		w.gnb.AddUE(w.ranUEID, u)

		start := time.Now()

		if err := w.gnb.RegisterWithoutSession(u, w.ranUEID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("register %s: %w", w.imsi, err)
		}

		if err := u.WaitForRRCRelease(e2eBenchTimeout); err != nil {
			return fmt.Errorf("release after registration of %s: %w", w.imsi, err)
		}

		registered := time.Now()

		if err := w.gnb.DeregisterFromIdle(u, w.ranUEID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("deregister %s: %w", w.imsi, err)
		}

		s.add("reg", registered.Sub(start))
		s.add("dereg", time.Since(registered))

		return nil
	},
	verify: func(core *benchCore) error {
		if n := core.cp.amf.CountRegisteredSubscribers(); n != 0 {
			return fmt.Errorf("%d subscribers still registered after the deregistration", n)
		}

		return nil
	},
}

var registrationWithPDUScenario = benchScenario{
	rat:            rat5G,
	withSubscriber: true,
	cycle: func(w *benchWorker, s samples) error {
		w.gnb.DiscardReceivedFrames()

		u, err := newBenchUE(w.gnb, w.imsi, true)
		if err != nil {
			return fmt.Errorf("create UE: %w", err)
		}

		w.gnb.AddUE(w.ranUEID, u)

		start := time.Now()

		if _, err := w.gnb.Register(u, w.ranUEID, scenarios.DefaultPDUSessionID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("register %s with a PDU session: %w", w.imsi, err)
		}

		registered := time.Now()

		if err := w.gnb.Deregister(u, w.ranUEID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("deregister %s: %w", w.imsi, err)
		}

		s.add("reg-pdu", registered.Sub(start))
		s.add("dereg", time.Since(registered))

		return w.awaitSessionReleased(func(supi etsi.SUPI) bool {
			return w.core.cp.smf.HasPDUSession(supi, scenarios.DefaultPDUSessionID)
		})
	},
	verify: func(core *benchCore) error {
		if n := core.cp.amf.CountRegisteredSubscribers(); n != 0 {
			return fmt.Errorf("%d subscribers still registered after the deregistration", n)
		}

		return verifyNoLeases(core)
	},
}

var s1SetupScenario = benchScenario{
	rat:            rat4G,
	radioPerWorker: true,
	cycle: func(w *benchWorker, s samples) error {
		start := time.Now()

		if err := w.enb.SendS1SetupRequest(); err != nil {
			return fmt.Errorf("send S1 Setup Request: %w", err)
		}

		if _, err := w.enb.WaitForMessage(s1enb.NoUEID, s1enb.Successful, s1ap.ProcS1Setup, e2eBenchTimeout); err != nil {
			return fmt.Errorf("wait for S1 Setup Response: %w", err)
		}

		s.add("s1setup", time.Since(start))

		return nil
	},
}

var attachScenario = benchScenario{
	rat:            rat4G,
	withSubscriber: true,
	cycle: func(w *benchWorker, s samples) error {
		u := w.enb.NewUE(w.imsi, benchK, benchOPc)

		start := time.Now()

		res, err := w.enb.Attach(u, e2eBenchTimeout)
		if err != nil {
			return fmt.Errorf("attach %s: %w", w.imsi, err)
		}

		defer w.enb.DiscardUEFrames(res.ENBUES1APID)

		if err := w.enb.WaitForEMMInformation(u, res.ENBUES1APID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("attach %s: %w", w.imsi, err)
		}

		attached := time.Now()

		if err := w.enb.SwitchOff(u, res.MMEUES1APID, res.ENBUES1APID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("detach %s: %w", w.imsi, err)
		}

		s.add("attach", attached.Sub(start))
		s.add("detach", time.Since(attached))

		return w.awaitSessionReleased(func(supi etsi.SUPI) bool {
			return w.core.cp.smf.HasEPSSession(supi, uint8(res.ERABID))
		})
	},
	verify: func(core *benchCore) error {
		if n := core.cp.mme.CountRegisteredSubscribers(); n != 0 {
			return fmt.Errorf("%d subscribers still attached after the detach", n)
		}

		return verifyNoLeases(core)
	},
}

func verifyNoLeases(core *benchCore) error {
	n, err := core.db.CountActiveLeases(context.Background())
	if err != nil {
		return fmt.Errorf("count active leases: %w", err)
	}

	if n != 0 {
		return fmt.Errorf("%d IP leases still active", n)
	}

	return nil
}

var e2eScenarios = []struct {
	name string
	sc   benchScenario
}{
	{"5g/NGSetup", ngSetupScenario},
	{"5g/Registration", registrationScenario},
	{"5g/RegistrationWithPDU", registrationWithPDUScenario},
	{"4g/S1Setup", s1SetupScenario},
	{"4g/Attach", attachScenario},
}

var e2eStorms = []struct {
	name string
	rat  rat
}{
	{"5g/Storm", rat5G},
	{"4g/Storm", rat4G},
}

func BenchmarkE2E(b *testing.B) {
	for _, tc := range e2eScenarios {
		b.Run(tc.name, func(b *testing.B) { runE2E(b, tc.sc) })
	}

	for _, tc := range e2eStorms {
		b.Run(tc.name, func(b *testing.B) { benchmarkStorm(b, tc.rat) })
	}
}

func verifyNoLeakedGoroutines(t *testing.T) {
	t.Helper()

	ignore := goleak.IgnoreCurrent()

	t.Cleanup(func() { goleak.VerifyNone(t, ignore) })
}

func TestE2EBenchSmoke(t *testing.T) {
	for _, tc := range e2eScenarios {
		t.Run(tc.name, func(t *testing.T) {
			verifyNoLeakedGoroutines(t)

			core := newBenchCore(t)
			core.uesPerRadio = 2

			workers := []*benchWorker{core.worker(t, tc.sc, 0), core.worker(t, tc.sc, 1)}

			var wg sync.WaitGroup

			errs := make([]error, len(workers))

			for i, w := range workers {
				wg.Go(func() {
					for range 2 {
						if errs[i] = tc.sc.cycle(w, samples{}); errs[i] != nil {
							return
						}
					}
				})
			}

			wg.Wait()

			for _, err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}

			tc.sc.mustVerify(t, core)
		})
	}

	for _, tc := range e2eStorms {
		t.Run(tc.name, func(t *testing.T) {
			verifyNoLeakedGoroutines(t)

			core := newBenchCore(t)
			storm := newStorm(t, core, tc.rat, 8, 2)

			for range 2 {
				storm.round(t, noTimer{}, samples{})
			}
		})
	}
}
