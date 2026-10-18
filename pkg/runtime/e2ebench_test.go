// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"cmp"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"math"
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

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/ellanetworks/core/internal/sctplisten"
	"github.com/ellanetworks/core/internal/tester/gnb"
	testerlogger "github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	e2eBenchDB          = flag.String("e2ebench.db", "raft", "database mode for E2E benchmarks: raft, raft-fast or local")
	e2eBenchLogLevel    = flag.String("e2ebench.loglevel", "warn", "core log level for E2E benchmarks")
	e2eBenchStormUEs    = flag.Int("e2ebench.storm.ues", 1000, "UEs that register at once in the storm benchmark; each op is one storm, so run it with -benchtime=1x or Nx")
	e2eBenchStormRadios = flag.Int("e2ebench.storm.radios", 10, "gNBs or eNBs the storm benchmark spreads its UEs across")
	e2eBenchLogFile     = flag.String("e2ebench.logfile", "", "file the core logs of E2E benchmarks are written to; empty discards them")
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
	s1Addr string
}

func newBenchCore(tb testing.TB) *benchCore {
	tb.Helper()

	requireSCTP(tb)
	configureBenchLogging(tb)

	ctx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(cancel)

	dbInstance := openBenchDB(ctx, tb)

	writer := dbwriter.NewBufferedDBWriter(dbInstance, 1000, logger.NetworkLog)
	logger.SetDb(writer)

	tb.Cleanup(func() {
		stopCtx, stop := context.WithTimeout(context.Background(), e2eBenchTimeout)
		defer stop()

		writer.Stop(stopCtx)
		logger.SetDb(nil)
	})

	if err := dbInstance.RefreshAMFPointer(ctx); err != nil {
		tb.Fatalf("refresh AMF pointer: %v", err)
	}

	cp := newControlPlane(ctx, dbInstance, cmp.Or(dbInstance.RaftID(), "1"))
	cp.smf.SetUPF(&benchUPF{})

	var wg sync.WaitGroup

	tb.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	wg.Go(func() {
		cp.ausf.Run(ctx)
	})

	registrationsWakeup, stopRegistrationsWakeup := dbInstance.Changefeed().Wakeup(db.TopicUERegistrations)

	wg.Go(func() {
		defer stopRegistrationsWakeup()

		cp.registrations.Run(ctx, registrationsWakeup)
	})

	core := &benchCore{db: dbInstance, cp: cp}
	core.n2Addr = serveBenchSCTP(ctx, tb, logger.AmfLog, newNGAPServer(cp.amf))
	core.s1Addr = serveBenchSCTP(ctx, tb, logger.MmeLog, newS1APServer(cp.mme))

	return core
}

func serveBenchSCTP(ctx context.Context, tb testing.TB, log *zap.Logger, server *sctp.Server) string {
	tb.Helper()

	ln, err := sctplisten.Listen(ctx, log, "127.0.0.1", 0, "")
	if err != nil {
		tb.Fatalf("listen SCTP: %v", err)
	}

	server.Serve(ctx, ln)

	tb.Cleanup(func() {
		stopCtx, stop := context.WithTimeout(context.Background(), e2eBenchTimeout)
		defer stop()

		server.Shutdown(stopCtx)
	})

	return ln.Addr().String()
}

func openBenchDB(ctx context.Context, tb testing.TB) *db.Database {
	tb.Helper()

	base := os.Getenv("ELLA_BENCH_DIR")
	if base == "" {
		base = os.TempDir()
	}

	dir, err := os.MkdirTemp(base, "e2ebench-")
	if err != nil {
		tb.Fatalf("create bench dir: %v", err)
	}

	tb.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "core.db")

	var dbInstance *db.Database

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

	tb.Cleanup(func() {
		if err := dbInstance.Close(); err != nil {
			tb.Errorf("close database: %v", err)
		}
	})

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

var benchLogging struct {
	once sync.Once
	err  error
}

func configureBenchLogging(tb testing.TB) {
	tb.Helper()

	benchLogging.once.Do(func() {
		level, err := zapcore.ParseLevel(*e2eBenchLogLevel)
		if err != nil {
			benchLogging.err = fmt.Errorf("parse -e2ebench.loglevel: %w", err)
			return
		}

		sink := io.Discard

		if *e2eBenchLogFile != "" {
			f, err := os.OpenFile(*e2eBenchLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				benchLogging.err = fmt.Errorf("open -e2ebench.logfile: %w", err)
				return
			}

			sink = f
		}

		enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
		core := zapcore.NewCore(enc, zapcore.AddSync(sink), level)
		logger.SwapSystemCore(core)

		testerlogger.Logger = zap.New(core).Named("Tester")
		testerlogger.GnbLogger = zap.New(core).Named("GNB")
		testerlogger.UeLogger = zap.New(core).Named("UE")
	})

	if benchLogging.err != nil {
		tb.Fatal(benchLogging.err)
	}
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
	core  *benchCore
	gnb   *gnb.GnodeB
	enb   *s1enb.ENB
	setup gnb.NGSetupRequestOpts
	imsi  string
}

type newWorkerFunc func(tb testing.TB, core *benchCore, index int) *benchWorker

func gnbWorker(withSubscriber bool) newWorkerFunc {
	return func(tb testing.TB, core *benchCore, index int) *benchWorker {
		return newGNBWorker(tb, core, index, withSubscriber)
	}
}

func enbWorker(withSubscriber bool) newWorkerFunc {
	return func(tb testing.TB, core *benchCore, index int) *benchWorker {
		return newENBWorker(tb, core, index, withSubscriber)
	}
}

func newENBWorker(tb testing.TB, core *benchCore, index int, withSubscriber bool) *benchWorker {
	tb.Helper()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(8 + index), // #nosec G115 -- index is a small worker position
		MCC:              scenarios.DefaultMCC,
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             fmt.Sprintf("bench-enb-%d", index),
		CoreS1MMEAddress: core.s1Addr,
		ENBAddress:       "127.0.0.1",
	})
	if err != nil {
		tb.Fatalf("start eNB %d: %v", index, err)
	}

	tb.Cleanup(func() { _ = e.Close() })

	w := &benchWorker{core: core, enb: e}

	if withSubscriber {
		w.imsi = benchIMSI(index)
		core.provisionSubscriber(tb, w.imsi)
	}

	return w
}

func newGNBWorker(tb testing.TB, core *benchCore, index int, withSubscriber bool) *benchWorker {
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
		GnbN3Address:    "127.0.0.1",
		SignallingOnly:  true,
	})
	if err != nil {
		tb.Fatalf("start gNB %s: %v", setup.GnbID, err)
	}

	tb.Cleanup(g.Close)

	if _, err := g.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, e2eBenchTimeout); err != nil {
		tb.Fatalf("initial NG Setup for gNB %s: %v", setup.GnbID, err)
	}

	w := &benchWorker{core: core, gnb: g, setup: setup}

	if withSubscriber {
		w.imsi = benchIMSI(index)
		core.provisionSubscriber(tb, w.imsi)
	}

	return w
}

func benchIMSI(index int) string {
	return fmt.Sprintf("%s%s%010d", scenarios.DefaultMCC, scenarios.DefaultMNC, index+1)
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

func (w *benchWorker) newUE(withSession bool) (*ue.UE, error) {
	return newBenchUE(w.gnb, w.imsi, withSession)
}

func newBenchUE(g *gnb.GnodeB, imsi string, withSession bool) (*ue.UE, error) {
	return ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		GnodeB:         g,
		Msin:           imsi[len(scenarios.DefaultMCC)+len(scenarios.DefaultMNC):],
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
		IMEISV:           scenarios.DefaultIMEISV,
		NoAutoPDUSession: !withSession,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
	})
}

var benchK, benchOPc = mustKey16(scenarios.DefaultKey), mustKey16(scenarios.DefaultOPC)

func mustKey16(h string) [16]byte {
	var k [16]byte

	if n, err := hex.Decode(k[:], []byte(h)); err != nil || n != len(k) {
		panic(fmt.Sprintf("invalid 128-bit key %q", h))
	}

	return k
}

func (w *benchWorker) awaitSessionReleased(held func(etsi.SUPI) bool) error {
	supi, err := etsi.NewSUPIFromIMSI(w.imsi)
	if err != nil {
		return fmt.Errorf("SUPI of %s: %w", w.imsi, err)
	}

	deadline := time.Now().Add(e2eBenchTimeout)

	for held(supi) {
		if time.Now().After(deadline) {
			return fmt.Errorf("the session of %s was not released", w.imsi)
		}

		time.Sleep(100 * time.Microsecond)
	}

	return nil
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
			if float64(len(ds))*(1-q.at) < 1 {
				continue
			}

			rank := int(math.Ceil(q.at * float64(len(ds))))
			b.ReportMetric(float64(ds[rank-1].Nanoseconds()), procedure+"-"+q.name+"-ns")
		}
	}
}

type benchCycle func(w *benchWorker, s samples) error

type benchScenario struct {
	newWorker newWorkerFunc
	cycle     benchCycle
	verify    func(core *benchCore) error
}

func (sc benchScenario) mustVerify(tb testing.TB, core *benchCore) {
	tb.Helper()

	if sc.verify == nil {
		return
	}

	if err := sc.verify(core); err != nil {
		tb.Fatal(err)
	}
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
		w := sc.newWorker(b, core, 0)
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
		sc.mustVerify(b, core)
	})

	for _, p := range []int{1, 8} {
		b.Run(fmt.Sprintf("par/p=%d", p), func(b *testing.B) {
			core := newBenchCore(b)

			workers := make([]*benchWorker, p*goruntime.GOMAXPROCS(0))
			for i := range workers {
				workers[i] = sc.newWorker(b, core, i)
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

			core.reportRaftApplies(b, applied)
			all.report(b)
			sc.mustVerify(b, core)
		})
	}
}

var ngSetupScenario = benchScenario{
	newWorker: gnbWorker(false),
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
	newWorker: gnbWorker(true),
	cycle: func(w *benchWorker, s samples) error {
		w.gnb.DiscardReceivedFrames()

		u, err := w.newUE(false)
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

var registrationWithPDUScenario = benchScenario{
	newWorker: gnbWorker(true),
	cycle: func(w *benchWorker, s samples) error {
		w.gnb.DiscardReceivedFrames()

		u, err := w.newUE(true)
		if err != nil {
			return fmt.Errorf("create UE: %w", err)
		}

		w.gnb.AddUE(benchRANUEID, u)

		start := time.Now()

		if _, err := w.gnb.Register(u, benchRANUEID, scenarios.DefaultPDUSessionID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("register %s with a PDU session: %w", w.imsi, err)
		}

		registered := time.Now()

		if err := w.gnb.Deregister(u, benchRANUEID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("deregister %s: %w", w.imsi, err)
		}

		s.add("reg-pdu", registered.Sub(start))
		s.add("dereg", time.Since(registered))

		if err := w.awaitSessionReleased(func(supi etsi.SUPI) bool {
			return w.core.cp.smf.HasPDUSession(supi, scenarios.DefaultPDUSessionID)
		}); err != nil {
			return err
		}

		return nil
	},
	verify: func(core *benchCore) error {
		if n := core.cp.amf.CountRegisteredSubscribers(); n != 0 {
			return fmt.Errorf("%d subscribers still registered after the deregistration", n)
		}

		n, err := core.db.CountActiveLeases(context.Background())
		if err != nil {
			return fmt.Errorf("count active leases: %w", err)
		}

		if n != 0 {
			return fmt.Errorf("%d IP leases still active after the deregistration", n)
		}

		return nil
	},
}

const (
	stormTimeout = 60 * time.Second
	t3510        = 15 * time.Second
)

type stormRAN interface {
	prepare(i int) error
	register(i int) error
	deregister(i int) error
}

type gnbStorm struct {
	gnbs []*gnb.GnodeB
	ues  []*ue.UE
}

func (st *gnbStorm) ranUEID(i int) int64 {
	return int64(i/len(st.gnbs)) + 1
}

func (st *gnbStorm) prepare(i int) error {
	g := st.gnbs[i%len(st.gnbs)]

	u, err := newBenchUE(g, benchIMSI(i), true)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	g.DiscardReceivedFrames()
	g.AddUE(st.ranUEID(i), u)
	st.ues[i] = u

	return nil
}

func (st *gnbStorm) register(i int) error {
	_, err := st.gnbs[i%len(st.gnbs)].Register(st.ues[i], st.ranUEID(i), scenarios.DefaultPDUSessionID, stormTimeout)

	return err
}

func (st *gnbStorm) deregister(i int) error {
	return st.gnbs[i%len(st.gnbs)].Deregister(st.ues[i], st.ranUEID(i), stormTimeout)
}

type enbStorm struct {
	enbs    []*s1enb.ENB
	ues     []*s1enb.UE
	results []*s1enb.AttachResult
}

func (st *enbStorm) prepare(i int) error {
	e := st.enbs[i%len(st.enbs)]

	e.DiscardReceivedFrames()
	st.ues[i] = e.NewUE(benchIMSI(i), benchK, benchOPc)

	return nil
}

func (st *enbStorm) register(i int) error {
	res, err := st.enbs[i%len(st.enbs)].Attach(st.ues[i], stormTimeout)
	st.results[i] = res

	return err
}

func (st *enbStorm) deregister(i int) error {
	res := st.results[i]

	return st.enbs[i%len(st.enbs)].Detach(st.ues[i], res.MMEUES1APID, res.ENBUES1APID, stormTimeout)
}

func benchmarkStorm(b *testing.B, newRAN func(b *testing.B, core *benchCore, nUEs, nRadios int) stormRAN) {
	nUEs, nRadios := *e2eBenchStormUEs, *e2eBenchStormRadios

	core := newBenchCore(b)
	ran := newRAN(b, core, nUEs, nRadios)

	for i := range nUEs {
		core.provisionSubscriber(b, benchIMSI(i))
	}

	all := samples{}
	overT3510 := 0

	b.ReportAllocs()

	for b.Loop() {
		b.StopTimer()

		for i := range nUEs {
			if err := ran.prepare(i); err != nil {
				b.Fatal(err)
			}
		}

		latencies := make([]time.Duration, nUEs)
		errs := make([]error, nUEs)

		var wg sync.WaitGroup

		b.StartTimer()

		for i := range nUEs {
			wg.Go(func() {
				t0 := time.Now()
				errs[i] = ran.register(i)
				latencies[i] = time.Since(t0)
			})
		}

		wg.Wait()

		b.StopTimer()

		for i, err := range errs {
			if err != nil {
				b.Fatalf("register %s: %v", benchIMSI(i), err)
			}

			all.add("reg", latencies[i])

			if latencies[i] > t3510 {
				overT3510++
			}
		}

		for i := range nUEs {
			wg.Go(func() {
				errs[i] = ran.deregister(i)
			})
		}

		wg.Wait()

		for i, err := range errs {
			if err != nil {
				b.Fatalf("deregister %s: %v", benchIMSI(i), err)
			}
		}

		deadline := time.Now().Add(stormTimeout)

		for core.cp.smf.SessionCount() != 0 || core.cp.amf.CountRegisteredSubscribers() != 0 || core.cp.mme.CountRegisteredSubscribers() != 0 {
			if time.Now().After(deadline) {
				b.Fatalf("%d sessions, %d 5G and %d 4G registrations left after the deregistration storm",
					core.cp.smf.SessionCount(), core.cp.amf.CountRegisteredSubscribers(), core.cp.mme.CountRegisteredSubscribers())
			}

			time.Sleep(time.Millisecond)
		}

		b.StartTimer()
	}

	b.ReportMetric(float64(overT3510)/float64(b.N), "over-T3510/op")
	all.report(b)
}

func newGNBStorm(b *testing.B, core *benchCore, nUEs, nRadios int) stormRAN {
	st := &gnbStorm{gnbs: make([]*gnb.GnodeB, nRadios), ues: make([]*ue.UE, nUEs)}

	for i := range st.gnbs {
		st.gnbs[i] = newGNBWorker(b, core, i, false).gnb
	}

	return st
}

func newENBStorm(b *testing.B, core *benchCore, nUEs, nRadios int) stormRAN {
	st := &enbStorm{enbs: make([]*s1enb.ENB, nRadios), ues: make([]*s1enb.UE, nUEs), results: make([]*s1enb.AttachResult, nUEs)}

	for i := range st.enbs {
		st.enbs[i] = newENBWorker(b, core, i, false).enb
	}

	return st
}

var s1SetupScenario = benchScenario{
	newWorker: enbWorker(false),
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
	newWorker: enbWorker(true),
	cycle: func(w *benchWorker, s samples) error {
		w.enb.DiscardReceivedFrames()

		u := w.enb.NewUE(w.imsi, benchK, benchOPc)

		start := time.Now()

		res, err := w.enb.Attach(u, e2eBenchTimeout)
		if err != nil {
			return fmt.Errorf("attach %s: %w", w.imsi, err)
		}

		attached := time.Now()

		if err := w.enb.Detach(u, res.MMEUES1APID, res.ENBUES1APID, e2eBenchTimeout); err != nil {
			return fmt.Errorf("detach %s: %w", w.imsi, err)
		}

		s.add("attach", attached.Sub(start))
		s.add("detach", time.Since(attached))

		if err := w.awaitSessionReleased(func(supi etsi.SUPI) bool {
			return w.core.cp.smf.HasEPSSession(supi, uint8(res.ERABID))
		}); err != nil {
			return err
		}

		return nil
	},
	verify: func(core *benchCore) error {
		if n := core.cp.mme.CountRegisteredSubscribers(); n != 0 {
			return fmt.Errorf("%d subscribers still attached after the detach", n)
		}

		n, err := core.db.CountActiveLeases(context.Background())
		if err != nil {
			return fmt.Errorf("count active leases: %w", err)
		}

		if n != 0 {
			return fmt.Errorf("%d IP leases still active after the detach", n)
		}

		return nil
	},
}

func BenchmarkE2E(b *testing.B) {
	b.Run("5g", func(b *testing.B) {
		b.Run("NGSetup", func(b *testing.B) { runE2E(b, ngSetupScenario) })
		b.Run("Registration", func(b *testing.B) { runE2E(b, registrationScenario) })
		b.Run("RegistrationWithPDU", func(b *testing.B) { runE2E(b, registrationWithPDUScenario) })
		b.Run("Storm", func(b *testing.B) { benchmarkStorm(b, newGNBStorm) })
	})
	b.Run("4g", func(b *testing.B) {
		b.Run("S1Setup", func(b *testing.B) { runE2E(b, s1SetupScenario) })
		b.Run("Attach", func(b *testing.B) { runE2E(b, attachScenario) })
		b.Run("Storm", func(b *testing.B) { benchmarkStorm(b, newENBStorm) })
	})
}

func TestE2EBenchSmoke(t *testing.T) {
	for name, sc := range map[string]benchScenario{
		"5g/NGSetup":             ngSetupScenario,
		"5g/Registration":        registrationScenario,
		"5g/RegistrationWithPDU": registrationWithPDUScenario,
		"4g/S1Setup":             s1SetupScenario,
		"4g/Attach":              attachScenario,
	} {
		t.Run(name, func(t *testing.T) {
			ignore := goleak.IgnoreCurrent()

			t.Cleanup(func() { goleak.VerifyNone(t, ignore) })

			core := newBenchCore(t)
			w := sc.newWorker(t, core, 0)

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
