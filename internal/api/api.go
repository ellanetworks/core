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
	UPF                 server.UPFUpdater
	Sessions            smf.SessionQuerier
	AMF                 *amf.AMF
	BGP                 *bgp.BGPService
	EmbedFS             fs.FS
	RegisterExtraRoutes func(*http.ServeMux)
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
		UPF:                 opts.UPF,
		Kernel:              kernelInt,
		JWTSecret:           jwtSecret,
		SecureCookie:        secureCookie,
		FrontendFS:          opts.EmbedFS,
		Sessions:            opts.Sessions,
		AMF:                 opts.AMF,
		BGP:                 opts.BGP,
		BcryptCost:          bcrypt.DefaultCost,
		Ready:               &s.ready,
		RegisterExtraRoutes: opts.RegisterExtraRoutes,
	})

	s.handler.set(fullHandler)
	s.ready.Store(true)

	reconcile := routeReconciler

	go func() {
		for {
			err := reconcile(ctx, opts.DB, kernelInt)
			if err != nil {
				logger.APILog.Error("couldn't reconcile routes", zap.Error(err))
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
				continue
			}
		}
	}()

	return nil
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
		err := kernelInt.EnsureGatewaysOnInterfaceInNeighTable(netIf)
		if err != nil {
			logger.APILog.Warn("failed to ensure gateways are in neighbour table for interface", zap.Any("interface", netIf), zap.Error(err))
		}
	}

	logger.APILog.Debug("Routes reconciled")

	return nil
}
