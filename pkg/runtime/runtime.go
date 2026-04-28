package runtime

import (
	"archive/tar"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas"
	"github.com/ellanetworks/core/internal/amf/nas/gmm"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/ngap/service"
	amfsctp "github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/api"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/jobs"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/ellanetworks/core/internal/sessions"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/supportbundle"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/internal/upf"
	"github.com/ellanetworks/core/internal/upf/bpfdump"
	"github.com/ellanetworks/core/version"
	nasLogger "github.com/free5gc/nas/logger"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

type RuntimeConfig struct {
	ConfigPath          string
	RegisterExtraRoutes func(mux *http.ServeMux)
	EmbedFS             fs.FS
}

func Start(ctx context.Context, rc RuntimeConfig) error {
	cfg, err := config.Validate(rc.ConfigPath)
	if err != nil {
		return fmt.Errorf("couldn't validate config: %w", err)
	}

	if err := logger.ConfigureLogging(
		cfg.Logging.SystemLogging.Level,
		cfg.Logging.SystemLogging.Output,
		cfg.Logging.SystemLogging.Path,
		cfg.Logging.AuditLogging.Output,
		cfg.Logging.AuditLogging.Path,
	); err != nil {
		return fmt.Errorf("couldn't configure logging: %w", err)
	}

	ver := version.GetVersion()

	logger.EllaLog.Info("Starting Ella Core",
		zap.String("version", ver.Version),
		zap.String("revision", ver.Revision),
	)

	var tp *trace.TracerProvider

	if cfg.Telemetry.Enabled {
		tp, err = tracing.InitTracer(ctx, tracing.TelemetryConfig{
			OTLPEndpoint:    cfg.Telemetry.OTLPEndpoint,
			ServiceName:     "ella-core",
			ServiceVersion:  ver.Version,
			ServiceRevision: ver.Revision,
		})
		if err != nil {
			return fmt.Errorf("couldn't initialize tracer: %w", err)
		}
	}

	ellaraft.RegisterMetrics()

	apiScheme := "http"
	if cfg.Interfaces.API.TLS.Cert != "" && cfg.Interfaces.API.TLS.Key != "" {
		apiScheme = "https"
	}

	apiAddress := fmt.Sprintf("%s://%s:%d", apiScheme, cfg.Interfaces.API.Address, cfg.Interfaces.API.Port)

	raftCfg := ellaraft.ClusterConfig{
		Enabled:           cfg.Cluster.Enabled,
		NodeID:            cfg.Cluster.NodeID,
		BindAddress:       cfg.Cluster.BindAddress,
		AdvertiseAddress:  cfg.Cluster.AdvertiseAddress,
		APIAddress:        apiAddress,
		Peers:             cfg.Cluster.Peers,
		HasJoinToken:      cfg.Cluster.JoinToken != "",
		JoinTimeout:       cfg.Cluster.JoinTimeout,
		ProposeTimeout:    cfg.Cluster.ProposeTimeout,
		SnapshotInterval:  cfg.Cluster.SnapshotInterval,
		SnapshotThreshold: cfg.Cluster.SnapshotThreshold,
		SchemaVersion:     db.SchemaVersion(),
		InitialSuffrage:   cfg.Cluster.InitialSuffrage,
	}

	var clusterLn *listener.Listener

	var raftOpts []ellaraft.ManagerOption

	var pki *pkiState

	var restoredFromBundle bool

	if cfg.Cluster.Enabled {
		dataDir := filepath.Dir(cfg.DB.Path)

		restored, err := maybeRestoreFromBundle(dataDir)
		if err != nil {
			return fmt.Errorf("restore bundle: %w", err)
		}

		restoredFromBundle = restored

		// cluster-id is unknown at construction; the agent re-reads it
		// before signing CSRs.
		pki = newPKIState(cfg.Cluster.NodeID, "", dataDir)

		// Join-token path runs before the listener comes up so raft
		// can mTLS-handshake as soon as it forms.
		if !pki.agent.HaveLeafOnDisk() {
			if err := runJoinFlow(ctx, pki.agent, cfg.Cluster.Peers, cfg.Cluster.JoinToken); err != nil {
				return fmt.Errorf("join flow: %w", err)
			}
		}

		if pki.agent.HaveLeafOnDisk() {
			if err := pki.agent.Load(); err != nil {
				return fmt.Errorf("load leaf: %w", err)
			}

			// Seed the listener's trust bundle from bundle.crt.
			// Without this, incoming mTLS would be rejected until
			// RefreshBundle fires — which requires raft to replicate,
			// which itself requires working mTLS.
			if pki.agent.ClusterID != "" {
				if err := pki.SeedBundleFromAgentDisk(pki.agent.ClusterID); err != nil {
					return fmt.Errorf("seed trust bundle: %w", err)
				}
			}
		}

		clusterLn = listener.New(listener.Config{
			BindAddress:      cfg.Cluster.BindAddress,
			AdvertiseAddress: cfg.Cluster.AdvertiseAddress,
			NodeID:           cfg.Cluster.NodeID,
			TrustBundle:      pki.BundleFunc(),
			Leaf:             pki.LeafFunc(),
			Revoked:          pki.RevokedFunc(),
		})
		raftOpts = append(raftOpts, ellaraft.WithClusterListener(clusterLn))
	}

	dbInstance, err := db.NewDatabase(ctx, cfg.DB.Path, raftCfg, raftOpts...)
	if err != nil {
		return fmt.Errorf("couldn't initialize database: %w", err)
	}

	bufferedWriter := dbwriter.NewBufferedDBWriter(dbInstance, 1000, logger.NetworkLog)
	logger.SetDb(bufferedWriter)

	if pki != nil {
		// Let the leader-side revocation handler nudge our local cache
		// as soon as revocation rows are written, so new handshakes on
		// this node see them without waiting for the 30 s refresher.
		server.SetRevocationRefresher(func(ctx context.Context) {
			if err := pki.RefreshRevocations(ctx, dbInstance); err != nil {
				logger.EllaLog.Warn("immediate revocation refresh failed", zap.Error(err))
			}
		})
	}

	// Provide the runtime config file contents to the supportbundle generator
	// so support bundles include the exact YAML used at startup. This is safe
	// because main controls what is exposed via the provider; here we simply
	// read the file from the configured path.
	supportbundle.ConfigProvider = func(ctx context.Context) ([]byte, error) {
		return os.ReadFile(rc.ConfigPath)
	}

	server.RegisterMetrics()

	// --- Phase A: start the HTTP server with discovery-only routes so
	// peers can probe /api/v1/status and POST /api/v1/cluster/members
	// during Raft cluster formation. ---
	apiServer, err := api.StartDiscovery(ctx, dbInstance, cfg)
	if err != nil {
		return fmt.Errorf("couldn't start API (discovery): %w", err)
	}

	if clusterLn != nil {
		stopClusterHTTP := server.StartClusterHTTP(dbInstance, clusterLn)
		defer stopClusterHTTP()

		if err := clusterLn.Start(ctx); err != nil {
			return fmt.Errorf("cluster listener: %w", err)
		}
	}

	if err := dbInstance.RunDiscovery(ctx); err != nil {
		return fmt.Errorf("cluster discovery failed: %w", err)
	}

	// Leader-side init runs from runLeaderInit via the OnBecameLeader
	// callback registered below. Followers block here until the
	// leader's Initialize replicates. Standalone runs Initialize from
	// NewDatabase.
	if cfg.Cluster.Enabled {
		if !dbInstance.IsLeader() {
			logger.EllaLog.Info("Waiting for leader initialization to replicate")

			if err := dbInstance.WaitForInitialization(ctx, 30*time.Second); err != nil {
				return fmt.Errorf("follower couldn't sync initial settings: %w", err)
			}

			logger.EllaLog.Info("Leader initialization replicated successfully")

			// Follower: build a local issuer handle (can't issue — we're
			// not the leader), and install it for HTTP handlers. The bundle
			// accessor is refreshed here and again on every leadership
			// transition.
			if pki != nil {
				pki.issuer = pkiissuer.New(dbInstance)

				if err := refreshFollowerBundleWithRetry(ctx, pki, 30*time.Second); err != nil {
					logger.EllaLog.Warn("follower refresh bundle", zap.Error(err))
				}

				if err := pki.RefreshRevocations(ctx, dbInstance); err != nil {
					logger.EllaLog.Warn("follower refresh revocations", zap.Error(err))
				}

				server.SetPKIIssuer(pki.issuer)
			}
		}

		// Every node (leader and follower) refreshes its cluster_members
		// row so the migration gate sees an accurate maxSchemaVersion.
		// Best effort: a transient leader miss just defers the gate until
		// the next leadership change or peer self-announce.
		if err := dbInstance.SelfAnnounce(ctx, ver.Version); err != nil {
			logger.EllaLog.Warn("self-announce to leader failed", zap.Error(err))
		}
	} else {
		if err := dbInstance.DeleteAllDynamicLeases(ctx); err != nil {
			return fmt.Errorf("couldn't release all dynamic leases: %w", err)
		}
	}

	var wg sync.WaitGroup

	sessionsGuard := sessions.NewLeaderGuard()
	jobsGuard := jobs.NewLeaderGuard()

	if observer := dbInstance.LeaderObserver(); observer != nil {
		observer.Register(sessionsGuard)
		observer.Register(jobsGuard)
		observer.Register(server.NewLeadershipAuditCallback(dbInstance.NodeID()))

		if pki != nil {
			observer.Register(newPKILeaderCallback(ctx, pki, dbInstance, clusterLn, cfg.Cluster.NodeID, ver.Version, restoredFromBundle))
		}
	}

	wg.Go(func() {
		jobs.RunDataRetentionWorker(ctx, dbInstance, jobsGuard)
	})

	wg.Go(func() {
		jobs.RunPKITidyWorker(ctx, dbInstance, jobsGuard)
	})

	wg.Go(func() {
		sessions.CleanUp(ctx, dbInstance, sessionsGuard)
	})

	// Revocation cache refresher: every node (leader and follower) polls
	// the replicated cluster_revoked_certs table on a short interval so
	// the in-memory revocation set — consulted on every cluster-listener
	// handshake — never drifts more than revocationRefreshInterval from
	// the committed Raft state. A leader-side immediate refresh still
	// happens inside RemoveClusterMember for the common case; this tick
	// is the backstop for followers and for the brief window between
	// leader action and follower apply.
	if pki != nil {
		wg.Go(func() {
			runRevocationRefresher(ctx, pki, dbInstance)
		})

		if clusterLn != nil {
			wg.Go(func() {
				runLeafRenewer(ctx, pki, clusterLn, dbInstance)
			})
		}
	}

	isNATEnabled, err := dbInstance.IsNATEnabled(ctx)
	if err != nil {
		return fmt.Errorf("couldn't determine if NAT is enabled: %w", err)
	}

	isFlowAccountingEnabled, err := dbInstance.IsFlowAccountingEnabled(ctx)
	if err != nil {
		return fmt.Errorf("couldn't determine if flow accounting is enabled: %w", err)
	}

	// Initialize BGP service
	n6IP, err := config.GetInterfaceIPFunc(cfg.Interfaces.N6.Name, config.IPv4)
	if err != nil {
		return fmt.Errorf("couldn't get N6 interface IP: %w", err)
	}

	n6Addr, err := netip.ParseAddr(n6IP)
	if err != nil {
		return fmt.Errorf("couldn't parse N6 IP %q: %w", n6IP, err)
	}

	realKernel := kernel.NewRealKernel(cfg.Interfaces.N3.Name, cfg.Interfaces.N6.Name)

	uePools := collectUEPools(ctx, dbInstance)
	n3Addr, _ := netip.ParseAddr(cfg.Interfaces.N3.Address)
	routeFilter := bgp.BuildRouteFilter(uePools, n3Addr, cfg.Interfaces.N6.Name)
	importStore := &bgpImportPrefixAdapter{db: dbInstance}

	bgpService := bgp.New(n6Addr, logger.EllaLog,
		bgp.WithKernel(realKernel),
		bgp.WithImportPrefixStore(importStore),
		bgp.WithRouteFilter(routeFilter),
	)

	bgpSettings, err := dbInstance.GetBGPSettings(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get BGP settings: %w", err)
	}

	if bgpSettings.Enabled {
		bgpPeers, err := dbInstance.ListAllBGPPeers(ctx)
		if err != nil {
			return fmt.Errorf("couldn't list BGP peers: %w", err)
		}

		servicePeers := server.DBPeersToBGPPeers(bgpPeers)

		err = bgpService.Start(ctx, server.DBSettingsToBGPSettings(bgpSettings), servicePeers, !isNATEnabled)
		if err != nil {
			listenAddr := bgpSettings.ListenAddress
			if listenAddr == "" {
				listenAddr = ":179"
			}

			logger.EllaLog.Error("BGP failed to start: address may be in use. Stop any external BGP daemon (FRR, BIRD) before enabling integrated BGP.", zap.String("address", listenAddr), zap.Error(err))
		}
	}

	// BGP reconciler: the single driver of BGP advertisements. Watches
	// the replicated ip_leases table and keeps the BGP RIB aligned with
	// the leases owned by this cluster node. Runs even when BGP is
	// disabled — the reconciler's calls are no-ops against a stopped
	// service, and starting here means re-enabling BGP via the API does
	// not need separate reconciler wiring.
	bgpWakeup, stopBgpWakeup := dbInstance.Changefeed().Wakeup(db.TopicIPLeases)
	defer stopBgpWakeup()

	bgpReconciler := bgp.NewReconciler(bgpService, &bgpLeaseStoreAdapter{db: dbInstance}, dbInstance.NodeID(), bgpWakeup)
	bgpReconciler.Start()

	n3Settings, err := dbInstance.GetN3Settings(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get N3 external address: %w", err)
	}

	n3IPv4, n3IPv6 := resolveN3Addresses(cfg.Interfaces.N3)

	advertisedN3IPv4 := n3IPv4
	advertisedN3IPv6 := n3IPv6

	if n3Settings != nil && n3Settings.ExternalAddress != "" {
		externalAddr, err := netip.ParseAddr(n3Settings.ExternalAddress)
		if err == nil {
			if externalAddr.Is4() {
				advertisedN3IPv4 = n3Settings.ExternalAddress
				logger.EllaLog.Debug("Using N3 external IPv4 address from N3 settings", zap.String("n3_external_address", advertisedN3IPv4))
			} else {
				advertisedN3IPv6 = n3Settings.ExternalAddress
				logger.EllaLog.Debug("Using N3 external IPv6 address from N3 settings", zap.String("n3_external_address", advertisedN3IPv6))
			}
		}
	}

	// Create SMF with dependency-injected adapters.
	smfPCF := &pcfDBAdapter{db: dbInstance}
	smfStore := &smfDBAdapter{db: dbInstance, allocator: ipam.NewSequentialAllocator(&leaseStoreAdapter{db: dbInstance})}
	smfAMF := &smfAMFAdapter{}

	smfInstance := smf.New(smfPCF, smfStore, nil, smfAMF)

	upfInstance, err := upf.Start(ctx, smfInstance, cfg.Interfaces.N3, n3IPv4, n3IPv6, advertisedN3IPv4, advertisedN3IPv6, cfg.Interfaces.N6, cfg.XDP.AttachMode, isNATEnabled, isFlowAccountingEnabled)
	if err != nil {
		return fmt.Errorf("couldn't start UPF: %w", err)
	}

	fallbackN3, _ := netip.ParseAddr(n3IPv4)
	upfReconciler := upf.NewSettingsReconciler(upfInstance, dbInstance, dbInstance.Changefeed(), fallbackN3)
	upfReconciler.Start()

	defer upfReconciler.Stop()

	eng := upfInstance.Engine()

	smfUPF := &smfUPFAdapter{engine: eng, upf: upfInstance}
	smfInstance.SetUPF(smfUPF)

	// Initialize SDF filters from database
	if eng != nil && dbInstance != nil {
		eng.SetBPFObjects(ctx, eng.BpfObjects, dbInstance)
	}

	// Wire supportbundle BPF dumper to dump live BPF maps from the UPF process.
	// The closure captures the live BPF objects via the session engine.
	// If the UPF hasn't initialized BPF objects, the dumper is a no-op.
	supportbundle.BpfDumper = func(ctx context.Context, tw *tar.Writer) error {
		if eng == nil || eng.BpfObjects == nil {
			// graceful no-op
			return nil
		}

		opts := bpfdump.DumpOptions{
			Exclude:          []string{"nat_ct", "flow_stats", "nocp_map"},
			MaxEntriesPerMap: 10000,
		}

		_, err := bpfdump.DumpAll(ctx, eng.BpfObjects, opts, tw)
		if err != nil {
			logger.EllaLog.Error("supportbundle: bpf dump failed", zap.Error(err))
			return err
		}

		logger.EllaLog.Info("supportbundle: bpf dump completed")

		return nil
	}

	smf.RegisterMetrics(smfInstance)
	upf.RegisterMetrics()

	ausfStore := &ausfDBAdapter{db: dbInstance}
	keyResolver := func(scheme string, keyID int) (string, error) {
		key, err := dbInstance.GetHomeNetworkKeyBySchemeAndIdentifier(ctx, scheme, keyID)
		if err != nil {
			return "", err
		}

		return key.PrivateKey, nil
	}

	ausfInstance := ausf.New(ausfStore, keyResolver)

	wg.Go(func() {
		ausfInstance.Run(ctx)
	})

	amfInstance := amf.New(dbInstance, ausfInstance, smfInstance)
	amfInstance.NAS = &nasAdapter{amf: amfInstance}
	smfAMF.amf = amfInstance

	amf.RegisterMetrics(amfInstance)
	gmm.RegisterMetrics()
	ngap.RegisterMetrics()

	// --- Phase B: upgrade the API server to serve all routes now that
	// the cluster is formed, settings are seeded, and NFs are running. ---
	if err := apiServer.Upgrade(ctx, api.UpgradeConfig{
		DB:                  dbInstance,
		Sessions:            smfInstance,
		AMF:                 amfInstance,
		BGP:                 bgpService,
		EmbedFS:             rc.EmbedFS,
		RegisterExtraRoutes: rc.RegisterExtraRoutes,
		ClusterListener:     clusterLn,
	}); err != nil {
		return fmt.Errorf("couldn't upgrade API: %w", err)
	}

	nasLogger.SetLogLevel(0) // Suppress free5gc NAS log output

	sctpServer := service.NewServer(service.Callbacks{
		Dispatch: func(ctx context.Context, conn *amfsctp.SCTPConn, msg []byte) {
			ngap.Dispatch(ctx, amfInstance, conn, msg)
		},
		Notify: func(conn *amfsctp.SCTPConn, notification amfsctp.Notification) {
			ngap.HandleSCTPNotification(amfInstance, conn, notification)
		},
		OnDisconnect: func(conn *amfsctp.SCTPConn) {
			if ran, ok := amfInstance.FindRadioByConn(conn); ok {
				amfInstance.RemoveRadio(ran)
				logger.AmfLog.Info("removed radio on connection close", zap.Int("fd", conn.Fd()))
			}
		},
	})

	interfaceName := ""
	if cfg.Interfaces.N2.Name != "" {
		interfaceName = cfg.Interfaces.N2.Name
	}

	err = sctpServer.ListenAndServe(ctx, cfg.Interfaces.N2.Address, cfg.Interfaces.N2.Port, interfaceName)
	if err != nil {
		return fmt.Errorf("couldn't start AMF: %w", err)
	}

	supportbundle.AMFDumper = func(ctx context.Context) (any, error) {
		return amfInstance.ExportUEs(ctx)
	}

	defer func() {
		// Each shutdown step gets its own timeout so that a slow step
		// does not starve subsequent ones.
		stepTimeout := 5 * time.Second

		// 0a. Signal the replicated cluster_members row that this node
		//     is draining out. Must happen before the leadership transfer
		//     so the write lands while we are still leader (if we are);
		//     a follower sends the write to the current leader over the
		//     cluster mTLS port.
		if dbInstance.ClusterEnabled() {
			sdCtx, sdCancel := context.WithTimeout(context.Background(), stepTimeout)

			if err := server.SignalShutdownDrain(sdCtx, dbInstance, clusterLn); err != nil {
				logger.EllaLog.Warn("Shutdown-drain state write failed", zap.Error(err))
			}

			sdCancel()
		}

		// 0b. Transfer leadership (HA only) so the cluster can continue
		//     serving writes while this node tears down.
		if dbInstance.ClusterEnabled() && dbInstance.IsLeader() {
			logger.EllaLog.Info("Transferring Raft leadership before shutdown")

			if err := dbInstance.LeadershipTransfer(); err != nil {
				logger.EllaLog.Warn("Leadership transfer failed", zap.Error(err))
			} else {
				logger.EllaLog.Info("Leadership transferred successfully")
			}
		}

		// 1. Stop accepting new HTTP requests.
		logger.EllaLog.Info("Shutting down API server")

		apiCtx, apiCancel := context.WithTimeout(context.Background(), stepTimeout)
		if err := apiServer.Shutdown(apiCtx); err != nil {
			logger.EllaLog.Warn("API server shutdown error", zap.Error(err))
		}

		apiCancel()

		// 2. Cancel all AMF UE timers immediately so paging and other
		//    retransmissions stop firing during teardown.
		logger.EllaLog.Info("Cancelling AMF timers")
		amfInstance.StopAllTimers()

		// 3. Notify RANs and close SCTP connections.
		logger.EllaLog.Info("Shutting down AMF")

		amfCtx, amfCancel := context.WithTimeout(context.Background(), stepTimeout)
		closeAMF(amfCtx, amfInstance, sctpServer)
		amfCancel()

		// 4. Stop the BGP reconciler, then the BGP speaker. The reconciler
		// stops first so no more Announce/Withdraw calls land on a
		// shutting-down service.
		logger.EllaLog.Info("Shutting down BGP reconciler")
		bgpReconciler.Stop()

		logger.EllaLog.Info("Shutting down BGP")

		if err := bgpService.Stop(); err != nil {
			logger.EllaLog.Error("BGP service shutdown error", zap.Error(err))
		}

		// 5. Stop UPF — this flushes remaining flow reports to SMF.
		logger.EllaLog.Info("Shutting down UPF")

		upfCtx, upfCancel := context.WithTimeout(context.Background(), stepTimeout)
		upfInstance.Close(upfCtx)
		upfCancel()

		// 6. Drain the buffered writer so queued events (including the
		//    flow reports just flushed by the UPF) are persisted to DB.
		logger.EllaLog.Info("Flushing buffered writer")

		bwCtx, bwCancel := context.WithTimeout(context.Background(), stepTimeout)
		bufferedWriter.Stop(bwCtx)
		bwCancel()

		// 7. Wait for background goroutines (data retention, session
		//    cleanup, AUSF) which were already signalled via ctx.Done().
		logger.EllaLog.Info("Waiting for background goroutines")
		wg.Wait()

		// 8. Close the database now that all writers have drained.
		logger.EllaLog.Info("Closing database")

		if err := dbInstance.Close(); err != nil {
			logger.EllaLog.Error("couldn't close database", zap.Error(err))
		}

		// 9. Flush the OpenTelemetry tracer.
		if tp != nil {
			logger.EllaLog.Info("Shutting down tracer")

			tpCtx, tpCancel := context.WithTimeout(context.Background(), stepTimeout)
			if err := tp.Shutdown(tpCtx); err != nil {
				logger.EllaLog.Warn("could not shutdown tracer", zap.Error(err))
			}

			tpCancel()
		}
	}()

	<-ctx.Done()
	logger.EllaLog.Info("Shutdown signal received, exiting.")

	return nil
}

func closeAMF(ctx context.Context, amfInstance *amf.AMF, srv *service.Server) {
	// Use a short dedicated timeout for the DB query so it doesn't
	// consume the caller's full shutdown budget.
	queryCtx, queryCancel := context.WithTimeout(ctx, 2*time.Second)
	operatorInfo, err := amfInstance.GetOperatorInfo(queryCtx)

	queryCancel()

	if err != nil {
		logger.AmfLog.Error("Could not get operator info", zap.Error(err))
	} else {
		unavailableGuamiList := send.BuildUnavailableGUAMIList(operatorInfo.Guami)

		for _, ran := range amfInstance.ListRadios() {
			if err := ran.NGAPSender.SendAMFStatusIndication(ctx, unavailableGuamiList); err != nil {
				logger.AmfLog.Error("failed to send AMF Status Indication to RAN", zap.Error(err))
			}
		}
	}

	srv.Shutdown(ctx)

	logger.AmfLog.Info("AMF terminated")
}

type nasAdapter struct {
	amf *amf.AMF
}

func (n *nasAdapter) HandleNAS(ctx context.Context, ue *amf.RanUe, nasPdu []byte) error {
	return nas.HandleNAS(ctx, n.amf, ue, nasPdu)
}

// ausfDBAdapter adapts *db.Database to the ausf.SubscriberStore interface.
type ausfDBAdapter struct {
	db *db.Database
}

func (a *ausfDBAdapter) GetSubscriber(ctx context.Context, imsi string) (*ausf.Subscriber, error) {
	sub, err := a.db.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, err
	}

	return &ausf.Subscriber{
		PermanentKey:   sub.PermanentKey,
		Opc:            sub.Opc,
		SequenceNumber: sub.SequenceNumber,
	}, nil
}

func (a *ausfDBAdapter) UpdateSequenceNumber(ctx context.Context, imsi string, sqn string) error {
	return a.db.EditSubscriberSequenceNumber(ctx, imsi, sqn)
}

// bgpImportPrefixAdapter adapts *db.Database to the bgp.ImportPrefixStore interface.
type bgpImportPrefixAdapter struct {
	db *db.Database
}

func (a *bgpImportPrefixAdapter) ListImportPrefixes(ctx context.Context, peerID int) ([]bgp.ImportPrefixEntry, error) {
	dbPrefixes, err := a.db.ListImportPrefixesByPeer(ctx, peerID)
	if err != nil {
		return nil, err
	}

	entries := make([]bgp.ImportPrefixEntry, len(dbPrefixes))
	for i, p := range dbPrefixes {
		entries[i] = bgp.ImportPrefixEntry{
			Prefix:    p.Prefix,
			MaxLength: p.MaxLength,
		}
	}

	return entries, nil
}

// bgpLeaseStoreAdapter adapts *db.Database to the bgp.LeaseStore
// interface the reconciler consumes. Keeps the bgp package decoupled
// from the full db surface.
type bgpLeaseStoreAdapter struct {
	db *db.Database
}

func (a *bgpLeaseStoreAdapter) ListActiveLeasesByNode(ctx context.Context, nodeID int) ([]bgp.Lease, error) {
	dbLeases, err := a.db.ListActiveLeasesByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	out := make([]bgp.Lease, 0, len(dbLeases))

	for _, l := range dbLeases {
		addr := l.Address()
		if !addr.IsValid() {
			continue
		}

		out = append(out, bgp.Lease{Address: addr, IMSI: l.IMSI})
	}

	return out, nil
}

// resolveN3Addresses scans the N3 interface and returns the first non-link-local
// IPv4 and IPv6 addresses found. The configured address (cfg.Interfaces.N3.Address)
// is used as the primary address for its family; the interface is then scanned
// for an address of the other family.
func resolveN3Addresses(n3Interface config.N3Interface) (n3IPv4, n3IPv6 string) {
	if n3Interface.Address != "" {
		if addr, err := netip.ParseAddr(n3Interface.Address); err == nil {
			if addr.Is4() {
				n3IPv4 = n3Interface.Address
			} else {
				n3IPv6 = n3Interface.Address
			}

			return n3IPv4, n3IPv6
		}
	}

	ifaceName := n3Interface.Name
	if n3Interface.VlanConfig != nil {
		ifaceName = n3Interface.VlanConfig.MasterInterface
	}

	ips, err := config.GetInterfaceIPs(ifaceName)
	if err != nil {
		return n3IPv4, n3IPv6
	}

	for _, ipStr := range ips {
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			continue
		}

		if addr.Is4() && n3IPv4 == "" {
			n3IPv4 = ipStr
		} else if addr.Is6() && n3IPv6 == "" {
			n3IPv6 = ipStr
		}
	}

	return n3IPv4, n3IPv6
}

// collectUEPools returns the UE IP pool CIDRs from all data networks.
func collectUEPools(ctx context.Context, dbInstance *db.Database) []netip.Prefix {
	dataNetworks, err := dbInstance.ListAllDataNetworks(ctx)
	if err != nil {
		logger.EllaLog.Warn("failed to list data networks for BGP filter", zap.Error(err))

		return nil
	}

	var pools []netip.Prefix

	for _, dn := range dataNetworks {
		prefix, err := netip.ParsePrefix(dn.IPPool)
		if err != nil {
			continue
		}

		pools = append(pools, prefix)
	}

	return pools
}
