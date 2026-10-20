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
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sys/unix"
)

var (
	e2eBenchDB          = flag.String("e2ebench.db", "raft", "database mode for E2E benchmarks: raft, raft-fast or local")
	e2eBenchLogLevel    = flag.String("e2ebench.loglevel", "warn", "core log level for E2E benchmarks")
	e2eBenchLogFile     = flag.String("e2ebench.logfile", "", "file the core and simulator logs of E2E benchmarks are written to; empty discards them")
	e2eBenchUEsPerRadio = flag.Int("e2ebench.ues-per-radio", 1, "UEs that share one gNB or eNB in the par variants of UE benchmarks")
	e2eBenchStormUEs    = flag.Int("e2ebench.storm.ues", 1000, "UEs that register at once in the storm benchmark; each op is one storm, so run it with -benchtime=1x or Nx")
	e2eBenchStormRadios = flag.Int("e2ebench.storm.radios", 10, "gNBs or eNBs the storm benchmark spreads its UEs across")
)

const (
	e2eBenchTimeout = 5 * time.Second
	envBenchDir     = "ELLA_BENCH_DIR"
	envRequireSCTP  = "ELLA_REQUIRE_SCTP"
)

var benchN3Address = netip.MustParseAddr("127.0.0.2")

func TestMain(m *testing.M) {
	flag.Parse()

	if f := flag.Lookup("test.bench"); f != nil && f.Value.String() != "" {
		fmt.Printf("e2e-db-mode: %s\n", *e2eBenchDB)
		fmt.Printf("e2e-db-fs: %s\n", fsType(benchBaseDir()))
		fmt.Printf("e2e-log-level: %s\n", *e2eBenchLogLevel)
		fmt.Printf("e2e-ues-per-radio: %d\n", *e2eBenchUEsPerRadio)
	}

	os.Exit(m.Run())
}

func benchBaseDir() string {
	return cmp.Or(os.Getenv(envBenchDir), os.TempDir())
}

func fsType(dir string) string {
	var st unix.Statfs_t

	if err := unix.Statfs(dir, &st); err != nil {
		return "unknown"
	}

	switch st.Type {
	case unix.TMPFS_MAGIC:
		return "tmpfs"
	case unix.EXT4_SUPER_MAGIC:
		return "ext4"
	case unix.XFS_SUPER_MAGIC:
		return "xfs"
	case unix.BTRFS_SUPER_MAGIC:
		return "btrfs"
	case unix.OVERLAYFS_SUPER_MAGIC:
		return "overlayfs"
	default:
		return fmt.Sprintf("0x%x", st.Type)
	}
}

type rat int

const (
	rat5G rat = iota
	rat4G
)

type benchCore struct {
	owner       testing.TB
	db          *db.Database
	cp          *controlPlane
	writer      *dbwriter.BufferedDBWriter
	n2Addr      string
	s1Addr      string
	uesPerRadio int

	mu        sync.Mutex
	gnbs      []*gnb.GnodeB
	gnbSetups []gnb.NGSetupRequestOpts
	enbs      []*s1enb.ENB
	workers   []*benchWorker
}

func newBenchCore(tb testing.TB) *benchCore {
	tb.Helper()

	requireSCTP(tb)
	configureBenchLogging(tb)

	ctx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(cancel)

	dbInstance := openBenchDB(ctx, tb)

	writer := startRadioEventWriter(dbInstance)

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

	cp.start(ctx, &wg)

	core := &benchCore{owner: tb, db: dbInstance, cp: cp, writer: writer, uesPerRadio: max(*e2eBenchUEsPerRadio, 1)}
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

	dir, err := os.MkdirTemp(benchBaseDir(), "e2ebench-")
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
		if os.Getenv(envRequireSCTP) != "" {
			tb.Fatalf("SCTP unavailable and %s is set: %v", envRequireSCTP, err)
		}

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

func (u *benchUPF) QueryUsage(_ context.Context, seid uint64) (models.UsageReport, error) {
	return models.UsageReport{SEID: seid}, nil
}

func (u *benchUPF) DeleteSession(_ context.Context, seid uint64) (models.UsageReport, error) {
	return models.UsageReport{SEID: seid}, nil
}

func (u *benchUPF) SuppressDownlinkDataNotification(context.Context, uint64) {}

func (u *benchUPF) ClearDownlinkDataNotification(context.Context, uint64) {}

func (u *benchUPF) UpdateFilters(context.Context, string, models.Direction, []models.FilterRule) error {
	return nil
}

func (u *benchUPF) RegisterIPv6Session(context.Context, *models.IPv6SessionRegistration) error {
	return nil
}

func (u *benchUPF) UnregisterIPv6Session(context.Context, uint32) error { return nil }

func (c *benchCore) gnbRadio(tb testing.TB, index int) (*gnb.GnodeB, gnb.NGSetupRequestOpts) {
	tb.Helper()

	for len(c.gnbs) <= index {
		setup := gnb.NGSetupRequestOpts{
			GnbID: fmt.Sprintf("%06x", 8+len(c.gnbs)),
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
			CoreN2Addresses: []string{c.n2Addr},
			GnbN2Address:    "127.0.0.1",
			GnbN3Address:    "127.0.0.1",
			SignallingOnly:  true,
		})
		if err != nil {
			tb.Fatalf("start gNB %s: %v", setup.GnbID, err)
		}

		c.owner.Cleanup(g.Close)

		if _, err := g.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, e2eBenchTimeout); err != nil {
			tb.Fatalf("initial NG Setup for gNB %s: %v", setup.GnbID, err)
		}

		c.gnbs = append(c.gnbs, g)
		c.gnbSetups = append(c.gnbSetups, setup)
	}

	return c.gnbs[index], c.gnbSetups[index]
}

func (c *benchCore) enbRadio(tb testing.TB, index int) *s1enb.ENB {
	tb.Helper()

	for len(c.enbs) <= index {
		n := len(c.enbs)

		e, err := s1enb.Start(&s1enb.StartOpts{
			ENBID:            uint32(8 + n), // #nosec G115 -- n is a small radio position
			MCC:              scenarios.DefaultMCC,
			MNC:              scenarios.DefaultMNC,
			TAC:              scenarios.DefaultTAC,
			Name:             fmt.Sprintf("bench-enb-%d", n),
			CoreS1MMEAddress: c.s1Addr,
			ENBAddress:       "127.0.0.1",
		})
		if err != nil {
			tb.Fatalf("start eNB %d: %v", n, err)
		}

		c.owner.Cleanup(func() { _ = e.Close() })

		c.enbs = append(c.enbs, e)
	}

	return c.enbs[index]
}

type benchWorker struct {
	core    *benchCore
	gnb     *gnb.GnodeB
	enb     *s1enb.ENB
	setup   gnb.NGSetupRequestOpts
	ranUEID int64
	imsi    string
}

func (c *benchCore) worker(tb testing.TB, sc benchScenario, index int) *benchWorker {
	tb.Helper()

	c.mu.Lock()
	defer c.mu.Unlock()

	for len(c.workers) <= index {
		c.workers = append(c.workers, c.newWorker(tb, sc, len(c.workers)))
	}

	return c.workers[index]
}

func (c *benchCore) newWorker(tb testing.TB, sc benchScenario, index int) *benchWorker {
	tb.Helper()

	perRadio := c.uesPerRadio
	if sc.radioPerWorker {
		perRadio = 1
	}

	w := &benchWorker{core: c, ranUEID: int64(index%perRadio) + 1}

	switch sc.rat {
	case rat5G:
		w.gnb, w.setup = c.gnbRadio(tb, index/perRadio)
	case rat4G:
		w.enb = c.enbRadio(tb, index/perRadio)
	}

	if sc.withSubscriber {
		w.imsi = benchIMSI(index)
		c.provisionSubscriber(tb, w.imsi)
	}

	if err := sc.cycle(w, samples{}); err != nil {
		tb.Fatalf("warm-up cycle of worker %d: %v", index, err)
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
	rat            rat
	withSubscriber bool
	radioPerWorker bool
	cycle          benchCycle
	verify         func(core *benchCore) error
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

type dbCounters struct {
	applied uint64
	dropped uint64
}

func (c *benchCore) counters() dbCounters {
	return dbCounters{applied: c.db.RaftAppliedIndex(), dropped: c.writer.Dropped()}
}

func (c *benchCore) reportDBWork(b *testing.B, before dbCounters) {
	b.Helper()

	deadline := time.Now().Add(e2eBenchTimeout)

	for c.writer.Pending() != 0 {
		if time.Now().After(deadline) {
			b.Fatalf("%d radio events still queued", c.writer.Pending())
		}

		time.Sleep(time.Millisecond)
	}

	after := c.counters()

	if *e2eBenchDB != "local" {
		b.ReportMetric(float64(after.applied-before.applied)/float64(b.N), "raft-applies/op")
	}

	b.ReportMetric(float64(after.dropped-before.dropped)/float64(b.N), "radio-events-dropped/op")
}

func runE2E(b *testing.B, sc benchScenario) {
	core := newBenchCore(b)

	b.Run("seq", func(b *testing.B) {
		w := core.worker(b, sc, 0)
		s := samples{}
		before := core.counters()

		b.ReportAllocs()

		for b.Loop() {
			if err := sc.cycle(w, s); err != nil {
				b.Fatal(err)
			}
		}

		core.reportDBWork(b, before)
		s.report(b)
		sc.mustVerify(b, core)
	})

	for _, p := range []int{1, 8} {
		b.Run(fmt.Sprintf("par/p=%d", p), func(b *testing.B) {
			workers := make([]*benchWorker, p*goruntime.GOMAXPROCS(0))
			for i := range workers {
				workers[i] = core.worker(b, sc, i)
			}

			var (
				mu   sync.Mutex
				all  = samples{}
				next atomic.Int64
			)

			before := core.counters()

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

			core.reportDBWork(b, before)
			all.report(b)
			sc.mustVerify(b, core)
		})
	}
}
