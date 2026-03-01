package api

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/ellanetworks/core/fleet"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
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

func GenerateJWTSecret() ([]byte, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return bytes, fmt.Errorf("failed to generate JWT secret: %w", err)
	}

	return bytes, nil
}

func Start(ctx context.Context, dbInstance *db.Database, cfg config.Config, upf server.UPFUpdater, fleetBuffer *fleet.FleetBuffer, embedFS fs.FS, registerExtraRoutes func(mux *http.ServeMux)) error {
	jwtSecret, err := GenerateJWTSecret()
	if err != nil {
		return fmt.Errorf("couldn't generate jwt secret: %v", err)
	}

	kernelInt := kernel.NewRealKernel(cfg.Interfaces.N3.Name, cfg.Interfaces.N6.Name)

	scheme := HTTPS
	if cfg.Interfaces.API.TLS.Cert == "" || cfg.Interfaces.API.TLS.Key == "" {
		scheme = HTTP
	}

	secureCookie := scheme == HTTPS

	router := server.NewHandler(dbInstance, cfg, upf, fleetBuffer, kernelInt, jwtSecret, secureCookie, embedFS, registerExtraRoutes)

	go func() {
		httpAddr := fmt.Sprintf("%s:%d", cfg.Interfaces.API.Address, cfg.Interfaces.API.Port)

		h2Server := &http2.Server{
			IdleTimeout: 1 * time.Millisecond,
		}

		srv := &http.Server{
			Addr:              httpAddr,
			ReadHeaderTimeout: 5 * time.Second,
			Handler:           h2c.NewHandler(router, h2Server),
		}
		if scheme == HTTPS {
			srv.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
			if err := srv.ListenAndServeTLS(cfg.Interfaces.API.TLS.Cert, cfg.Interfaces.API.TLS.Key); err != nil {
				logger.APILog.Fatal("couldn't start API server", zap.Error(err))
			}
		} else {
			if err := srv.ListenAndServe(); err != nil {
				logger.APILog.Fatal("couldn't start API server", zap.Error(err))
			}
		}
	}()

	logger.APILog.Info("API server started", zap.String("scheme", string(scheme)), zap.String("address", fmt.Sprintf("%s://%s:%d", scheme, cfg.Interfaces.API.Address, cfg.Interfaces.API.Port)))

	// Reconcile routes on startup and every 5 minutes.
	go func() {
		for {
			err := routeReconciler(dbInstance, kernelInt)
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

func ReconcileKernelRouting(dbInstance *db.Database, kernelInt kernel.Kernel) error {
	expectedRoutes, _, err := dbInstance.ListRoutesPage(context.Background(), 1, 100)
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
		_, ipNetwork, err := net.ParseCIDR(route.Destination)
		if err != nil {
			return fmt.Errorf("couldn't parse destination: %v", err)
		}

		ipGateway := net.ParseIP(route.Gateway)
		if ipGateway == nil || ipGateway.To4() == nil {
			return fmt.Errorf("invalid gateway: %v", route.Gateway)
		}

		ipGateway = ipGateway.To4()

		kernelNetworkInterface, ok := interfaceDBKernelMap[route.Interface]
		if !ok {
			return fmt.Errorf("invalid interface: %v", route.Interface)
		}

		routeExists, err := kernelInt.RouteExists(ipNetwork, ipGateway, route.Metric, kernelNetworkInterface)
		if err != nil {
			return fmt.Errorf("couldn't check if route exists: %v", err)
		}

		if !routeExists {
			err := kernelInt.CreateRoute(ipNetwork, ipGateway, route.Metric, kernelNetworkInterface)
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
