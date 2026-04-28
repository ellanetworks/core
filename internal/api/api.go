package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// interfaceDBKernelMap maps the interface string to the kernel.NetworkInterface enum.
var interfaceDBKernelMap = map[db.NetworkInterface]kernel.NetworkInterface{
	db.N3: kernel.N3,
	db.N6: kernel.N6,
}

type Scheme string

const (
	HTTP  Scheme = "http"
	HTTPS Scheme = "https"
)

// routeReconciler is used to reconcile routes periodically.
// In tests we can override it to disable actual reconciliation.
var routeReconciler = ReconcileKernelRouting

// routeReconcileBackstop is the periodic invariant-checking sweep
// when no change events have fired. The primary trigger is the
// changefeed; this exists only to recover from missed signals.
const routeReconcileBackstop = 5 * time.Minute

// Server wraps the HTTP server and supports two-phase startup. Phase one
// (StartDiscovery) starts the listener with only the routes needed for
// cluster discovery. Phase two (Upgrade) swaps in the full API handler
// after the cluster has formed and settings have been seeded.
type Server struct {
	httpServer *http.Server
	handler    handlerRef
	cfg        config.Config
	ready      atomic.Bool
}

// handlerRef is a concurrency-safe swappable HTTP handler.
type handlerRef struct {
	mu sync.RWMutex
	h  http.Handler
}

func (hr *handlerRef) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hr.mu.RLock()
	h := hr.h
	hr.mu.RUnlock()

	h.ServeHTTP(w, r)
}

func (hr *handlerRef) set(h http.Handler) {
	hr.mu.Lock()
	hr.h = h
	hr.mu.Unlock()
}

// UpgradeConfig holds the dependencies needed to upgrade the API server
// from discovery-only routes to the full API.
type UpgradeConfig struct {
	DB                  *db.Database
	Sessions            smf.SessionQuerier
	AMF                 *amf.AMF
	BGP                 *bgp.BGPService
	EmbedFS             fs.FS
	RegisterExtraRoutes func(*http.ServeMux)
	ClusterListener     *listener.Listener
}

// StartDiscovery creates and starts the HTTP server with only the routes
// required for cluster discovery (status, cluster membership, metrics,
// OpenAPI spec). Call Upgrade after cluster formation to enable the full API.
func StartDiscovery(ctx context.Context, dbInstance *db.Database, cfg config.Config) (*Server, error) {
	s := &Server{cfg: cfg}

	discoveryHandler := server.NewDiscoveryHandler(server.DiscoveryHandlerConfig{
		DB:     dbInstance,
		Config: cfg,
		Ready:  &s.ready,
	})

	s.handler.set(discoveryHandler)

	scheme := resolveScheme(cfg)

	httpAddr := fmt.Sprintf(":%s", strconv.Itoa(cfg.Interfaces.API.Port))
	if cfg.Interfaces.API.Address != "" {
		httpAddr = net.JoinHostPort(cfg.Interfaces.API.Address, strconv.Itoa(cfg.Interfaces.API.Port))
	}

	h2Server := &http2.Server{
		IdleTimeout: 120 * time.Second,
	}

	srv := &http.Server{
		Addr:              httpAddr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       1 * time.Minute,
		WriteTimeout:      5 * time.Minute,
	}

	s.httpServer = srv

	go func() {
		var serveErr error

		var ln net.Listener

		if cfg.Interfaces.API.Name != "" {
			lc := net.ListenConfig{
				Control: func(network, address string, c syscall.RawConn) error {
					var setSockOptErr error

					if err := c.Control(func(fd uintptr) {
						setSockOptErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, cfg.Interfaces.API.Name)
					}); err != nil {
						return err
					}

					return setSockOptErr
				},
			}

			ln, serveErr = lc.Listen(ctx, "tcp", httpAddr)
			if serveErr != nil {
				logger.APILog.Fatal("couldn't create listener", zap.Error(serveErr))
				return
			}
		}

		logFields := []zap.Field{
			zap.String("scheme", string(scheme)),
			zap.String("address", httpAddr),
		}
		if cfg.Interfaces.API.Name != "" {
			logFields = append(logFields, zap.String("interface", cfg.Interfaces.API.Name))
		}

		logger.APILog.Info("API server started", logFields...)

		if scheme == HTTPS {
			srv.Handler = &s.handler

			srv.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
				},
				CurvePreferences: []tls.CurveID{
					tls.X25519,
					tls.CurveP256,
					tls.CurveP384,
				},
			}

			if ln != nil {
				serveErr = srv.ServeTLS(ln, cfg.Interfaces.API.TLS.Cert, cfg.Interfaces.API.TLS.Key)
			} else {
				serveErr = srv.ListenAndServeTLS(cfg.Interfaces.API.TLS.Cert, cfg.Interfaces.API.TLS.Key)
			}
		} else {
			srv.Handler = h2c.NewHandler(&s.handler, h2Server)

			if ln != nil {
				serveErr = srv.Serve(ln)
			} else {
				serveErr = srv.ListenAndServe()
			}
		}

		if serveErr != nil && serveErr != http.ErrServerClosed {
			logger.APILog.Fatal("couldn't start API server", zap.Error(serveErr))
		}
	}()

	return s, nil
}

// Upgrade swaps the discovery-only handler for the full API handler. It
// must be called after cluster formation and database initialization so
// that the JWT secret and all settings are available.
func (s *Server) Upgrade(ctx context.Context, opts UpgradeConfig) error {
	jwtSecretBytes, err := opts.DB.GetJWTSecret(ctx)
	if err != nil {
		return fmt.Errorf("couldn't load jwt secret from database: %v", err)
	}

	jwtSecret := server.NewJWTSecret(jwtSecretBytes)
	kernelInt := kernel.NewRealKernel(s.cfg.Interfaces.N3.Name, s.cfg.Interfaces.N6.Name)
	secureCookie := resolveScheme(s.cfg) == HTTPS

	fullHandler := server.NewHandler(server.HandlerConfig{
		DB:                  opts.DB,
		Config:              s.cfg,
		JWTSecret:           jwtSecret,
		SecureCookie:        secureCookie,
		FrontendFS:          opts.EmbedFS,
		Sessions:            opts.Sessions,
		AMF:                 opts.AMF,
		BGP:                 opts.BGP,
		BcryptCost:          bcrypt.DefaultCost,
		Ready:               &s.ready,
		RegisterExtraRoutes: opts.RegisterExtraRoutes,
		ClusterListener:     opts.ClusterListener,
	})

	s.handler.set(fullHandler)
	s.ready.Store(true)

	// Install the AMF/BGP references used by the cluster-port drain and
	// resume side-effect endpoints. The cluster HTTP mux starts before
	// these services exist (cluster formation needs the port up early),
	// so the endpoints late-bind dependencies from this atomic pointer.
	server.SetClusterSideEffectDeps(server.ClusterSideEffectDeps{
		AMF: opts.AMF,
		BGP: opts.BGP,
	})

	reconcile := routeReconciler

	go func() {
		sub := opts.DB.Changefeed().Subscribe(db.TopicRoutes)
		defer sub.Close()

		runReconcile := func() {
			if err := reconcile(ctx, opts.DB, kernelInt); err != nil {
				logger.APILog.Error("couldn't reconcile routes", zap.Error(err))
			}
		}

		runReconcile()

		backstop := time.NewTicker(routeReconcileBackstop)
		defer backstop.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-sub.Events:
				runReconcile()
			case <-sub.Dropped:
				runReconcile()
			case <-backstop.C:
				runReconcile()
			}
		}
	}()

	if opts.BGP != nil {
		bgpStore := &bgpSettingsStoreAdapter{db: opts.DB}
		filterBuilder := func(fbCtx context.Context) (*bgp.RouteFilter, error) {
			pools := server.CollectUEPools(fbCtx, opts.DB)
			n3Addr, _ := netip.ParseAddr(s.cfg.Interfaces.N3.Address)

			return bgp.BuildRouteFilter(pools, n3Addr, s.cfg.Interfaces.N6.Name), nil
		}

		bgpSettingsWakeup, stopBGPSettingsWakeup := opts.DB.Changefeed().Wakeup(
			db.TopicBGPSettings,
			db.TopicBGPPeers,
			db.TopicNATSettings,
			db.TopicDataNetworks,
		)

		bgpReconciler := bgp.NewSettingsReconciler(opts.BGP, bgpStore, filterBuilder, bgpSettingsWakeup)
		seedReconcilerFromCurrentState(ctx, bgpReconciler, opts.DB)
		bgpReconciler.Start()

		go func() {
			<-ctx.Done()
			bgpReconciler.Stop()
			stopBGPSettingsWakeup()
		}()
	}

	return nil
}

// bgpSettingsStoreAdapter adapts *db.Database to bgp.SettingsStore,
// converting from the DB row types into the BGP service's own types
// so the bgp package does not depend on db.
type bgpSettingsStoreAdapter struct {
	db *db.Database
}

func (a *bgpSettingsStoreAdapter) GetSettings(ctx context.Context) (bgp.BGPSettings, error) {
	settings, err := a.db.GetBGPSettings(ctx)
	if err != nil {
		return bgp.BGPSettings{}, err
	}

	return server.DBSettingsToBGPSettings(settings), nil
}

func (a *bgpSettingsStoreAdapter) ListPeers(ctx context.Context) ([]bgp.BGPPeer, error) {
	dbPeers, err := a.db.ListAllBGPPeers(ctx)
	if err != nil {
		return nil, err
	}

	return server.DBPeersToBGPPeers(dbPeers), nil
}

func (a *bgpSettingsStoreAdapter) IsNATEnabled(ctx context.Context) (bool, error) {
	return a.db.IsNATEnabled(ctx)
}

func seedReconcilerFromCurrentState(ctx context.Context, r *bgp.SettingsReconciler, dbInstance *db.Database) {
	settings, err := dbInstance.GetBGPSettings(ctx)
	if err != nil {
		return
	}

	dbPeers, err := dbInstance.ListAllBGPPeers(ctx)
	if err != nil {
		return
	}

	natEnabled, err := dbInstance.IsNATEnabled(ctx)
	if err != nil {
		return
	}

	r.MarkApplied(server.DBSettingsToBGPSettings(settings), server.DBPeersToBGPPeers(dbPeers), !natEnabled)
}

// Handler returns the swappable HTTP handler backing the API server.
// The returned handler follows the server through Phase A (discovery)
// and Phase B (full API) automatically, so callers such as the cluster
// proxy mux always see the current handler without re-wiring.
func (s *Server) Handler() http.Handler {
	return &s.handler
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func resolveScheme(cfg config.Config) Scheme {
	if cfg.Interfaces.API.TLS.Cert == "" || cfg.Interfaces.API.TLS.Key == "" {
		return HTTP
	}

	return HTTPS
}

// ReconcileKernelRouting drives this node's kernel routing table from
// the replicated routes table. It both adds DB routes that are missing
// from the kernel and removes Ella-owned kernel routes that no longer
// have a DB counterpart. Operator-installed routes are untouched
// because the deletion pass is scoped to Ella's protocol marker.
//
// BGP-learned routes are managed by internal/bgp/watcher.go; this
// reconciler skips the bgpRouteMetric to avoid stepping on it.
const bgpRouteMetric = 200

func ReconcileKernelRouting(ctx context.Context, dbInstance *db.Database, kernelInt kernel.Kernel) error {
	expectedRoutes, _, err := dbInstance.ListRoutesPage(ctx, 1, 100)
	if err != nil {
		return fmt.Errorf("couldn't list routes: %v", err)
	}

	ipForwardingEnabled, err := kernelInt.IsIPForwardingEnabled()
	if err != nil {
		return fmt.Errorf("couldn't check if IP forwarding is enabled: %v", err)
	}

	if !ipForwardingEnabled {
		err := kernelInt.EnableIPForwarding()
		if err != nil {
			return fmt.Errorf("couldn't enable IP forwarding: %v", err)
		}
	}

	type routeKey struct {
		destination string
		gateway     string
		priority    int
		ifKey       kernel.NetworkInterface
	}

	desired := make(map[routeKey]struct{}, len(expectedRoutes))

	for _, route := range expectedRoutes {
		destPrefix, err := netip.ParsePrefix(route.Destination)
		if err != nil {
			return fmt.Errorf("couldn't parse destination: %v", err)
		}

		gwAddr, err := netip.ParseAddr(route.Gateway)
		if err != nil || !gwAddr.Is4() {
			return fmt.Errorf("invalid gateway: %v", route.Gateway)
		}

		kernelNetworkInterface, ok := interfaceDBKernelMap[route.Interface]
		if !ok {
			return fmt.Errorf("invalid interface: %v", route.Interface)
		}

		desired[routeKey{
			destination: destPrefix.String(),
			gateway:     gwAddr.String(),
			priority:    route.Metric,
			ifKey:       kernelNetworkInterface,
		}] = struct{}{}

		routeExists, err := kernelInt.RouteExists(destPrefix, gwAddr, route.Metric, kernelNetworkInterface)
		if err != nil {
			return fmt.Errorf("couldn't check if route exists: %v", err)
		}

		if !routeExists {
			err := kernelInt.CreateRoute(destPrefix, gwAddr, route.Metric, kernelNetworkInterface)
			if err != nil {
				return fmt.Errorf("couldn't create route: %v", err)
			}
		}
	}

	for _, netIf := range interfaceDBKernelMap {
		managed, err := kernelInt.ListManagedRoutes(netIf)
		if err != nil {
			return fmt.Errorf("couldn't list managed routes on %v: %v", netIf, err)
		}

		for _, r := range managed {
			// BGP watcher owns its own metric; never reclaim those here.
			if r.Priority == bgpRouteMetric {
				continue
			}

			key := routeKey{
				destination: r.Destination.String(),
				gateway:     r.Gateway.String(),
				priority:    r.Priority,
				ifKey:       netIf,
			}

			if _, ok := desired[key]; ok {
				continue
			}

			if err := kernelInt.DeleteRoute(r.Destination, r.Gateway, r.Priority, netIf); err != nil {
				logger.APILog.Warn("couldn't delete stale route",
					zap.String("destination", r.Destination.String()),
					zap.String("gateway", r.Gateway.String()),
					zap.Int("priority", r.Priority),
					zap.Error(err))
			}
		}
	}

	for _, netIf := range interfaceDBKernelMap {
		err := kernelInt.EnsureGatewaysOnInterfaceInNeighTable(netIf)
		if err != nil {
			logger.APILog.Warn("failed to ensure gateways are in neighbour table for interface", zap.Any("interface", netIf), zap.Error(err))
		}
	}

	logger.APILog.Debug("Routes reconciled")

	return nil
}
