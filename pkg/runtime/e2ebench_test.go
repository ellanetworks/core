// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/ellanetworks/core/internal/sctplisten"
	"github.com/ellanetworks/core/internal/tester/gnb"
	testerlogger "github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	e2eBenchDB       = flag.String("e2ebench.db", "raft", "database mode for E2E benchmarks: raft, raft-fast or local")
	e2eBenchLogLevel = flag.String("e2ebench.loglevel", "warn", "core log level for E2E benchmarks")
	e2eBenchLogFile  = flag.String("e2ebench.logfile", "", "file the core logs of E2E benchmarks are written to; empty discards them")
)

const (
	e2eBenchTimeout = 5 * time.Second
	benchRANUEID    = int64(1)
)

var benchN3Address = netip.MustParseAddr("127.0.0.2")

type benchCore struct {
	db     *db.Database
	cp     *controlPlane
	n2Addr string
}

func newBenchCore(tb testing.TB) *benchCore {
	tb.Helper()

	requireSCTP(tb)
	configureBenchLogging(tb)

	ctx, cancel := context.WithCancel(context.Background())

	dbInstance := openBenchDB(ctx, tb)

	writer := dbwriter.NewBufferedDBWriter(dbInstance, 1000, logger.NetworkLog)
	logger.SetDb(writer)

	if err := dbInstance.RefreshAMFPointer(ctx); err != nil {
		cancel()
		tb.Fatalf("refresh AMF pointer: %v", err)
	}

	cp := newControlPlane(ctx, dbInstance, cmp.Or(dbInstance.RaftID(), "1"))
	cp.smf.SetUPF(&benchUPF{})

	var wg sync.WaitGroup

	wg.Go(func() {
		cp.ausf.Run(ctx)
	})

	registrationsWakeup, stopRegistrationsWakeup := dbInstance.Changefeed().Wakeup(db.TopicUERegistrations)

	wg.Go(func() {
		defer stopRegistrationsWakeup()

		cp.registrations.Run(ctx, registrationsWakeup)
	})

	ln, err := sctplisten.Listen(ctx, logger.AmfLog, "127.0.0.1", 0, "")
	if err != nil {
		cancel()
		tb.Fatalf("listen NGAP: %v", err)
	}

	ngapServer := newNGAPServer(cp.amf)
	ngapServer.Serve(ctx, ln)

	tb.Cleanup(func() {
		stopCtx, stop := context.WithTimeout(context.Background(), e2eBenchTimeout)
		defer stop()

		ngapServer.Shutdown(stopCtx)
		cancel()
		wg.Wait()
		writer.Stop(stopCtx)
		logger.SetDb(nil)

		if err := dbInstance.Close(); err != nil {
			tb.Errorf("close database: %v", err)
		}
	})

	return &benchCore{db: dbInstance, cp: cp, n2Addr: ln.Addr().String()}
}

func openBenchDB(ctx context.Context, tb testing.TB) *db.Database {
	tb.Helper()

	dir := tb.TempDir()

	if base := os.Getenv("ELLA_BENCH_DIR"); base != "" {
		var err error

		dir, err = os.MkdirTemp(base, "e2ebench-")
		if err != nil {
			tb.Fatalf("create bench dir: %v", err)
		}

		tb.Cleanup(func() { _ = os.RemoveAll(dir) })
	}

	path := filepath.Join(dir, "core.db")

	var (
		dbInstance *db.Database
		err        error
	)

	switch *e2eBenchDB {
	case "raft":
		dbInstance, err = db.NewDatabase(ctx, path, ellaraft.ClusterConfig{SchemaVersion: db.SchemaVersion()})
	case "raft-fast":
		cfg := ellaraft.FastTestConfig()
		cfg.SchemaVersion = db.SchemaVersion()
		dbInstance, err = db.NewDatabase(ctx, path, cfg)
	case "local":
		dbInstance, err = db.NewDatabaseWithoutRaft(ctx, path)
	default:
		tb.Fatalf("unknown -e2ebench.db mode %q", *e2eBenchDB)
	}

	if err != nil {
		tb.Fatalf("open database: %v", err)
	}

	if err := dbInstance.StartDiscovery(ctx); err != nil {
		tb.Fatalf("start discovery: %v", err)
	}

	if err := dbInstance.WaitForFSMCatchUp(ctx); err != nil {
		tb.Fatalf("wait for FSM catch-up: %v", err)
	}

	if err := dbInstance.WaitForInitialization(ctx, e2eBenchTimeout); err != nil {
		tb.Fatalf("wait for initialization: %v", err)
	}

	return dbInstance
}

func configureBenchLogging(tb testing.TB) {
	tb.Helper()

	level, err := zapcore.ParseLevel(*e2eBenchLogLevel)
	if err != nil {
		tb.Fatalf("parse -e2ebench.loglevel: %v", err)
	}

	sink := io.Discard

	if *e2eBenchLogFile != "" {
		f, err := os.OpenFile(*e2eBenchLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			tb.Fatalf("open -e2ebench.logfile: %v", err)
		}

		tb.Cleanup(func() { _ = f.Close() })

		sink = f
	}

	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	restore := logger.SwapSystemCore(zapcore.NewCore(enc, zapcore.AddSync(sink), level))
	tb.Cleanup(restore)

	testerlogger.Logger = zap.NewNop()
	testerlogger.GnbLogger = zap.NewNop()
	testerlogger.UeLogger = zap.NewNop()
}

func requireSCTP(tb testing.TB) {
	tb.Helper()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_SCTP)
	if err != nil {
		tb.Skipf("SCTP unavailable (load the sctp kernel module): %v", err)
	}

	_ = syscall.Close(fd)
}

type benchUPF struct {
	nextTEID atomic.Uint32
}

func (u *benchUPF) EstablishSession(context.Context, *models.EstablishRequest) (*models.EstablishResponse, error) {
	return &models.EstablishResponse{N3TEID: u.nextTEID.Add(1), N3IPv4: benchN3Address}, nil
}

func (u *benchUPF) ModifySession(_ context.Context, req *models.ModifyRequest) (*models.ModifyResponse, error) {
	teids := make(map[uint8]uint32)

	for _, pdr := range req.UpdatePDRs {
		if pdr.PDI.LocalFTEID != nil && pdr.PDI.LocalFTEID.ChooseID != 0 {
			teids[pdr.PDI.LocalFTEID.ChooseID] = u.nextTEID.Add(1)
		}
	}

	return &models.ModifyResponse{ChosenTEIDs: teids}, nil
}

func (u *benchUPF) DeleteSession(context.Context, uint64) error { return nil }

func (u *benchUPF) SuppressDownlinkDataNotification(context.Context, uint64) {}

func (u *benchUPF) ClearDownlinkDataNotification(context.Context, uint64) {}

func (u *benchUPF) UpdateFilters(context.Context, string, models.Direction, []models.FilterRule) error {
	return nil
}

func (u *benchUPF) RegisterIPv6Session(context.Context, *models.IPv6SessionRegistration) error {
	return nil
}

func (u *benchUPF) UnregisterIPv6Session(context.Context, uint32) error { return nil }

type benchWorker struct {
	gnb   *gnb.GnodeB
	setup gnb.NGSetupRequestOpts
	imsi  string
}

func newBenchWorker(tb testing.TB, core *benchCore, index int, withSubscriber bool) *benchWorker {
	tb.Helper()

	setup := gnb.NGSetupRequestOpts{
		GnbID: fmt.Sprintf("%06x", 8+index),
		Mcc:   scenarios.DefaultMCC,
		Mnc:   scenarios.DefaultMNC,
		Sst:   scenarios.DefaultSST,
		Tac:   scenarios.DefaultTAC,
	}
	setup.Name = "bench-" + setup.GnbID

	g, err := gnb.Start(&gnb.StartOpts{
		GnbID:           setup.GnbID,
		MCC:             setup.Mcc,
		MNC:             setup.Mnc,
		SST:             setup.Sst,
		TAC:             setup.Tac,
		Name:            setup.Name,
		CoreN2Addresses: []string{core.n2Addr},
		GnbN2Address:    "127.0.0.1",
	})
	if err != nil {
		tb.Fatalf("start gNB %s: %v", setup.GnbID, err)
	}

	tb.Cleanup(g.Close)

	if _, err := g.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, e2eBenchTimeout); err != nil {
		tb.Fatalf("initial NG Setup for gNB %s: %v", setup.GnbID, err)
	}

	w := &benchWorker{gnb: g, setup: setup}

	if withSubscriber {
		w.imsi = fmt.Sprintf("%s%s%010d", scenarios.DefaultMCC, scenarios.DefaultMNC, index+1)
		core.provisionSubscriber(tb, w.imsi)
	}

	return w
}

func (c *benchCore) provisionSubscriber(tb testing.TB, imsi string) {
	tb.Helper()

	ctx := context.Background()

	profile, err := c.db.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		tb.Fatalf("get default profile: %v", err)
	}

	if err := c.db.CreateSubscriber(ctx, &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: scenarios.DefaultSequenceNumber,
		PermanentKey:   scenarios.DefaultKey,
		Opc:            scenarios.DefaultOPC,
		ProfileID:      profile.ID,
	}); err != nil {
		tb.Fatalf("create subscriber %s: %v", imsi, err)
	}
}

func (w *benchWorker) newUE() (*ue.UE, error) {
	return ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		GnodeB:         w.gnb,
		Msin:           w.imsi[len(scenarios.DefaultMCC)+len(scenarios.DefaultMNC):],
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.DefaultDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		NoAutoPDUSession: true,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
	})
}

type samples map[string][]time.Duration

func (s samples) add(procedure string, d time.Duration) {
	s[procedure] = append(s[procedure], d)
}

func (s samples) merge(other samples) {
	for procedure, ds := range other {
		s[procedure] = append(s[procedure], ds...)
	}
}

func (s samples) report(b *testing.B) {
	for procedure, ds := range s {
		if len(ds) == 0 {
			continue
		}

		slices.Sort(ds)

		for _, q := range []struct {
			name string
			at   float64
		}{{"p50", 0.50}, {"p95", 0.95}, {"p99", 0.99}} {
			idx := min(int(q.at*float64(len(ds))), len(ds)-1)
			b.ReportMetric(float64(ds[idx].Nanoseconds())/1e3, procedure+"-"+q.name+"-µs")
		}
	}
}

type benchCycle func(w *benchWorker, s samples) error

type benchScenario struct {
	withSubscriber bool
	cycle          benchCycle
	verify         func(core *benchCore) error
}

func (c *benchCore) reportRaftApplies(b *testing.B, before uint64) {
	if *e2eBenchDB == "local" || b.N == 0 {
		return
	}

	b.ReportMetric(float64(c.db.RaftAppliedIndex()-before)/float64(b.N), "raft-applies/op")
}

func runE2E(b *testing.B, sc benchScenario) {
	b.Run("seq", func(b *testing.B) {
		core := newBenchCore(b)
		w := newBenchWorker(b, core, 0, sc.withSubscriber)
		s := samples{}
		applied := core.db.RaftAppliedIndex()

		b.ReportAllocs()

		for b.Loop() {
			if err := sc.cycle(w, s); err != nil {
				b.Fatal(err)
			}
		}

		core.reportRaftApplies(b, applied)
		s.report(b)
	})

	for _, p := range []int{1, 8} {
		b.Run(fmt.Sprintf("par/p=%d", p), func(b *testing.B) {
			core := newBenchCore(b)

			workers := make([]*benchWorker, p*goruntime.GOMAXPROCS(0))
			for i := range workers {
				workers[i] = newBenchWorker(b, core, i, sc.withSubscriber)
			}

			var (
				mu   sync.Mutex
				all  = samples{}
				next atomic.Int64
			)

			applied := core.db.RaftAppliedIndex()

			b.ReportAllocs()
			b.SetParallelism(p)
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				w := workers[next.Add(1)-1]
				s := samples{}

				for pb.Next() {
					if err := sc.cycle(w, s); err != nil {
						b.Error(err)
						return
					}
				}

				mu.Lock()
				all.merge(s)
				mu.Unlock()
			})

			b.StopTimer()

			b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "cycles/s")
			core.reportRaftApplies(b, applied)
			all.report(b)
		})
	}
}

var ngSetupScenario = benchScenario{
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
	withSubscriber: true,
	cycle: func(w *benchWorker, s samples) error {
		w.gnb.DiscardReceivedFrames()

		u, err := w.newUE()
		if err != nil {
			return fmt.Errorf("create UE: %w", err)
		}

		w.gnb.AddUE(benchRANUEID, u)

		start := time.Now()

		if err := w.gnb.RegisterWithoutSession(u, benchRANUEID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("register %s: %w", w.imsi, err)
		}

		if err := u.WaitForRRCRelease(e2eBenchTimeout); err != nil {
			return fmt.Errorf("release after registration of %s: %w", w.imsi, err)
		}

		registered := time.Now()

		if err := w.gnb.DeregisterFromIdle(u, benchRANUEID, e2eBenchTimeout); err != nil {
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

func BenchmarkE2E(b *testing.B) {
	b.Run("5g", func(b *testing.B) {
		b.Run("NGSetup", func(b *testing.B) { runE2E(b, ngSetupScenario) })
		b.Run("Registration", func(b *testing.B) { runE2E(b, registrationScenario) })
	})
}

func TestE2EBenchSmoke(t *testing.T) {
	for name, sc := range map[string]benchScenario{
		"5g/NGSetup":      ngSetupScenario,
		"5g/Registration": registrationScenario,
	} {
		t.Run(name, func(t *testing.T) {
			core := newBenchCore(t)
			w := newBenchWorker(t, core, 0, sc.withSubscriber)

			for range 2 {
				if err := sc.cycle(w, samples{}); err != nil {
					t.Fatal(err)
				}

				if sc.verify != nil {
					if err := sc.verify(core); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
